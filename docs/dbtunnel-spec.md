# dbtunnel 設計ドキュメント

AWS SSM経由で踏み台サーバーにポートフォワーディングを確立し、Sequel Aceを起動してRDSに接続するCLIツール。

## 背景・課題

本番DBに接続するために、以下の手順を毎回実行する必要がある：

1. `aws ssm start-session --target ***` で踏み台サーバーに接続
2. Sequel Aceアプリケーションを起動
3. 接続情報を入力してDB接続

これを1コマンドで実行できるようにしたい。

## 要件

- 複数環境（production, staging等）に対応
- パスワードは平文で保存しない（AWS Secrets Managerから取得）
- まずは個人利用、将来的にチーム共有の可能性あり

## 使用イメージ

```bash
# ステージング環境に接続
dbtunnel staging
 - application_A # コマンドライン上で選択
 - application_B

# 本番環境に接続
dbtunnel production
 - application_A # コマンドライン上で選択
 - application_B

# 環境一覧を表示
dbtunnel list
production:
  - application_A
  - application_B
staging:
  - application_A
  - application_B

# 設定ファイルの場所を指定
dbtunnel staging --config ~/.dbtunnel/config.yaml
```

## アーキテクチャ

```
┌─────────────────────────────────────────────────────────┐
│  dbtunnel staging                                       │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  1. 設定ファイルから環境情報を取得                        │
│     (config.yaml: インスタンスID, RDSホスト, DB名など)    │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  2. AWS Secrets Managerからパスワードを取得              │
│     aws secretsmanager get-secret-value ...             │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  3. SSMポートフォワーディング開始（バックグラウンド）       │
│     aws ssm start-session --target ...                  │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  4. Sequel Ace起動                                      │
│     open "mysql://user:${password}@..." -a "Sequel Ace" │
└─────────────────────────────────────────────────────────┘
```

## 設定ファイル

`~/.dbtunnel/config.yaml`

```yaml
environments:
  production:
    aws_profile: prod
    aws_region: ap-northeast-1
    ssm_target: i-0123456789abcdef0  # 踏み台サーバー（環境で共通）
    databases:
      application_A:
        rds_host: prod-app-a.xxx.ap-northeast-1.rds.amazonaws.com
        port: 3306
        local_port: 13306
        database: app_a_production
        user: admin
        secret_name: prod/app-a/db-password
      application_B:
        rds_host: prod-app-b.xxx.ap-northeast-1.rds.amazonaws.com
        port: 3306
        local_port: 13307
        database: app_b_production
        user: admin
        secret_name: prod/app-b/db-password

  staging:
    aws_profile: staging
    aws_region: ap-northeast-1
    ssm_target: i-abcdef0123456789
    databases:
      application_A:
        rds_host: staging-app-a.xxx.ap-northeast-1.rds.amazonaws.com
        port: 3306
        local_port: 13316
        database: app_a_staging
        user: admin
        secret_name: staging/app-a/db-password
      application_B:
        rds_host: staging-app-b.xxx.ap-northeast-1.rds.amazonaws.com
        port: 3306
        local_port: 13317
        database: app_b_staging
        user: admin
        secret_name: staging/app-b/db-password
```

## 技術スタック

- **言語**: Go
- **CLIフレームワーク**: Cobra
- **設定管理**: Viper
- **インタラクティブTUI**: Bubbletea + Bubbles + Lipgloss (github.com/charmbracelet)
- **AWS SDK**: aws-sdk-go-v2

## 前提条件

ユーザー環境に以下がインストールされている必要があります：

- AWS CLI
- Session Manager Plugin
- Sequel Ace

## ディレクトリ構成

```
dbtunnel/
├── cmd/
│   ├── root.go           # ルートコマンド、グローバルフラグ
│   ├── connect.go        # メインのconnectコマンド（デフォルト）
│   └── list.go           # 環境一覧表示
├── internal/
│   ├── config/
│   │   └── config.go     # 設定ファイル読み込み
│   ├── aws/
│   │   ├── ssm.go        # SSMセッション管理
│   │   └── secrets.go    # Secrets Manager連携
│   └── sequelace/
│       └── open.go       # Sequel Ace起動
├── config.example.yaml   # 設定ファイルのサンプル
├── go.mod
├── go.sum
├── main.go
└── README.md
```

## 実装の詳細

### 1. 設定ファイル読み込み (internal/config/config.go)

```go
type Database struct {
    RDSHost    string `mapstructure:"rds_host"`
    Port       int    `mapstructure:"port"`
    LocalPort  int    `mapstructure:"local_port"`
    Database   string `mapstructure:"database"`
    User       string `mapstructure:"user"`
    SecretName string `mapstructure:"secret_name"`
}

type Environment struct {
    AWSProfile string              `mapstructure:"aws_profile"`
    AWSRegion  string              `mapstructure:"aws_region"`
    SSMTarget  string              `mapstructure:"ssm_target"`
    Databases  map[string]Database `mapstructure:"databases"`
}

type Config struct {
    Environments map[string]Environment `mapstructure:"environments"`
}
```

### 2. Secrets Manager連携 (internal/aws/secrets.go)

```go
func GetSecret(ctx context.Context, profile, region, secretName string) (string, error) {
    // AWS SDK v2を使用
    // プロファイルとリージョンを指定してクライアント作成
    // GetSecretValueでパスワードを取得
}
```

### 3. SSMポートフォワーディング (internal/aws/ssm.go)

```go
func StartPortForwarding(ctx context.Context, env Environment, db Database) (*exec.Cmd, error) {
    // aws ssm start-session コマンドをバックグラウンドで実行
    // AWS-StartPortForwardingSessionToRemoteHost ドキュメントを使用
    // env: AWSプロファイル、リージョン、踏み台サーバー情報
    // db: RDSホスト、ポート、ローカルポート情報
}
```

### 4. Sequel Ace起動 (internal/sequelace/open.go)

```go
func Open(user, password, host string, port int, database string) error {
    // mysql:// URL schemeを構築
    // open -a "Sequel Ace" で起動
}
```

## 注意事項

### Sequel AceのURL scheme

- `mysql://user:password@host:port/database` 形式をサポート
- **パスワードに特殊文字があると正しく動作しない既知の問題あり**
  - 参考: https://github.com/sequel-ace/sequel-ace/issues/1843
  - 回避策: パスワードをURLエンコードしても完全には解決しない場合がある

### セキュリティ考慮

- パスワードはメモリ上にのみ保持し、ファイルには書き込まない
- プロセス引数にパスワードが見える可能性があるため、将来的には改善を検討
  - 代替案: Sequel Aceの接続ファイル（.plist）を一時生成して削除

## 将来の拡張案

- [ ] TablePlus等、他のDBクライアント対応
- [ ] PostgreSQL対応
- [ ] 接続履歴の保存
- [ ] セッション終了時のクリーンアップ（Ctrl+Cでポートフォワーディング終了）
- [ ] Homebrew tap でのインストール対応

## 参考リンク

- [Cobra](https://github.com/spf13/cobra) - Go CLI framework
- [Viper](https://github.com/spf13/viper) - Go configuration library
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for TUIs
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
- [Sequel Ace](https://sequel-ace.com/)
- [AWS SSM Port Forwarding](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-sessions-start.html)