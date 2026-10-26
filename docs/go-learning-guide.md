# Go コードリーディングガイド（Rubyエンジニア向け）

## 学習進捗

- [x] 1. `main.go` - エントリーポイント
- [x] 2. `cmd/root.go` - CLI基盤、変数宣言、Cobra
- [ ] 3. `internal/config/config.go` - struct, メソッド, エラー処理
- [ ] 4. `internal/aws/secrets.go` - 外部API呼び出しパターン
- [ ] 5. `cmd/connect.go` - 全体のフロー制御
- [ ] 6. `internal/tui/selector.go` - interface, TEAパターン
- [ ] 7. `internal/service/connection.go` - goroutine, channel

---

## プロジェクト構造

```
dbtunnel/
├── main.go                    # エントリーポイント（6行）
├── go.mod                     # 依存管理（Gemfile相当、自動生成）
├── cmd/                       # CLIコマンド定義
│   ├── root.go               # ルートコマンド、フラグ定義
│   ├── connect.go            # 接続コマンド
│   ├── list.go               # 一覧コマンド
│   └── config.go             # 設定コマンド
└── internal/                  # 外部から import 不可
    ├── config/               # 設定ファイル管理
    ├── aws/                  # AWS連携（Secrets Manager, SSM）
    ├── tui/                  # ターミナルUI（Bubbletea）
    ├── sequelace/            # Sequel Ace起動
    └── service/              # ビジネスロジック
```

---

## Ruby vs Go 基本概念対比表

### パッケージ / モジュール

| Go | Ruby |
|----|------|
| `package main` | 実行可能ファイル |
| `package config` | `module Config` |
| `internal/` | 外部から使えない（強制） |

### 可視性

| Go | Ruby |
|----|------|
| `Execute()` 大文字始まり | `public` |
| `execute()` 小文字始まり | `private` |

### 変数宣言

```go
// Go: 型を明示
var name string = "foo"
var count int
name := "foo"  // 短縮形（関数内のみ）
```

```ruby
# Ruby: 型宣言不要
name = "foo"
count = 0
```

### ポインタ

```go
// Go: 値とポインタを区別
cfg := Config{}    // 値（コピーされる）
cfg := &Config{}   // ポインタ（参照）
*cfg               // ポインタの中身にアクセス
&cfg               // アドレスを取得
```

```ruby
# Ruby: 常に参照（ポインタのようなもの）
cfg = Config.new
```

### エラーハンドリング

```go
// Go: 戻り値でエラーを返す
result, err := doSomething()
if err != nil {
    return fmt.Errorf("失敗: %w", err)
}
```

```ruby
# Ruby: 例外を投げる
begin
  result = do_something
rescue => e
  raise "失敗: #{e.message}"
end
```

### 無名関数

```go
// Go
handler := func(x int) int {
    return x * 2
}
```

```ruby
# Ruby
handler = ->(x) { x * 2 }
```

---

## 依存管理（go.mod）

### Ruby との違い

| 操作 | Ruby | Go |
|------|------|-----|
| 初期化 | `bundle init` | `go mod init <module名>` |
| 追加 | Gemfileに手書き → `bundle install` | コードにimport → `go mod tidy` |
| 削除 | Gemfileから消す → `bundle install` | importから消す → `go mod tidy` |

### 重要なポイント

- **go.mod はほぼ自動生成**される
- **コードが先、go.mod は後**（Rubyと逆）
- `go mod tidy` = 「整理整頓」（追加と削除を同時に行う）

---

## 学習済み: main.go

```go
package main                              // 実行可能プログラム

import "github.com/r-ishino/dbtunnel/cmd" // パッケージのimport

func main() {                             // エントリーポイント
    cmd.Execute()                         // public関数呼び出し
}
```

**ポイント:**
- `package main` + `func main()` = 実行可能プログラム
- import パスは常にフルパス
- 大文字始まりの関数は外部から呼べる

---

## 学習済み: cmd/root.go

### 変数宣言
```go
var (
    cfgFile string         // ゼロ値 "" で初期化
    cfg     *config.Config // ゼロ値 nil で初期化
)
```

### Cobra（CLIフレームワーク、Thor相当）
```go
var rootCmd = &cobra.Command{
    Use:   "dbtunnel [environment]",  // コマンド名
    Short: "説明",                     // 短い説明
    RunE:  func(...) error { ... },   // 実行する関数
}
```

### init() 関数
```go
func init() {
    // パッケージロード時に自動実行
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "...")
}
```

### & アドレス演算子
```go
&cfgFile  // cfgFile のアドレス（ポインタ）を取得
          // StringVar が値を書き込めるようにするため
```

### if 内での変数宣言
```go
if err := something(); err != nil {
    // err はこのブロック内でのみ有効
}
```

---

## 次回: internal/config/config.go

学ぶ予定の概念:
- struct の定義とメソッド
- struct タグ（`mapstructure:"..."`）
- レシーバー（`func (c *Config) Method()`）
- map と slice の操作
- 複数戻り値のパターン

---

## コマンドリファレンス

```bash
# 開発環境
make dev      # 開発サーバー（ホットリロード）
make shell    # コンテナ内シェル
make build    # ビルド
make lint     # Lint実行

# 依存管理
go mod tidy   # 依存を整理（追加・削除）
go mod init   # 初期化（最初の1回だけ）
go get <pkg>  # パッケージ追加
```