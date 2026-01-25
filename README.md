# dbtunnel

AWS SSM経由で踏み台サーバーにポートフォワーディングを確立し、Sequel Aceを起動してRDSに接続するCLIツール。

## 特徴

- 1コマンドでSSMポートフォワーディング + Sequel Ace起動
- 複数環境（production, staging等）に対応
- パスワードはAWS Secrets Managerから自動取得（平文保存なし）
- インタラクティブなTUIで環境・データベースを選択

## 前提条件

以下がインストールされている必要があります：

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
- [Sequel Ace](https://sequel-ace.com/)

## インストール

### ビルド済みバイナリを使用

```bash
# リポジトリをクローン
git clone https://github.com/ishinoryuuichi/dbtunnel.git
cd dbtunnel

# ビルド（Docker必須）
make build

# パスの通った場所にコピー
cp dbtunnel /usr/local/bin/
```

### Intel Mac の場合

```bash
make build-intel
```

## 設定

設定ファイルを `~/.dbtunnel/config.yaml` に作成します。
configコマンドで設定ファイルをデフォルトのエディタで起動します。

```bash
dbtunnel config
```

### 設定ファイルの例

```yaml
environments:
  production:
    aws_profile: prod
    aws_region: ap-northeast-1
    ssm_target: i-0123456789abcdef0  # 踏み台サーバー
    databases:
      application_A:
        rds_host: prod-app-a.xxx.ap-northeast-1.rds.amazonaws.com
        port: 3306
        local_port: 13306
        database: app_a_production
        user: admin
        secret_name: prod/app-a/db-password
        # secret_key: password  # JSON形式の場合のキー名（省略時は "password"）
      application_B:
        ssm_target: i-abcdef1234567890  # DB固有の踏み台を使用する場合
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
```

### 設定項目

| 項目 | 説明 |
|------|------|
| `aws_profile` | 使用するAWSプロファイル名 |
| `aws_region` | AWSリージョン |
| `ssm_target` | 踏み台サーバーのEC2インスタンスID |
| `rds_host` | RDSエンドポイント |
| `port` | RDSポート（通常3306） |
| `local_port` | ローカルでリッスンするポート |
| `database` | データベース名 |
| `user` | DBユーザー名 |
| `secret_name` | Secrets Managerのシークレット名 |
| `secret_key` | JSON形式シークレットのキー名（省略時: `password`） |

## 使い方

```bash
# 環境を指定して接続（DB選択TUIが表示される）
dbtunnel staging
dbtunnel production

# 環境未指定で起動（環境選択TUIが表示される）
dbtunnel

# 環境一覧を表示
dbtunnel list

# 設定ファイルの場所を指定
dbtunnel staging --config /path/to/config.yaml
```

## 仕組み

```
┌─────────────────────────────────────────────────────────┐
│  dbtunnel staging                                       │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  1. 設定ファイルから環境情報を取得                        │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  2. AWS Secrets Managerからパスワードを取得              │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  3. SSMポートフォワーディング開始（バックグラウンド）       │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│  4. Sequel Ace起動                                      │
└─────────────────────────────────────────────────────────┘
```

## 開発

開発はDocker経由で行います。

```bash
# 開発サーバー（ホットリロード）
make dev

# コンテナ内シェル
make shell

# ビルド
make build

# Lint実行
make lint

# コードフォーマット
make fmt
```
