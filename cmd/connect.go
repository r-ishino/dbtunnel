package cmd

import (
	"fmt"

	"github.com/ishinoryuuichi/dbtunnel/internal/config"
	"github.com/ishinoryuuichi/dbtunnel/internal/tui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect [environment]",
	Short: "指定した環境のDBに接続",
	Long: `指定した環境のDBに接続します。
環境を指定しない場合は、環境選択プロンプトが表示されます。
環境を指定すると、その環境内のデータベースを選択するプロンプトが表示されます。`,
	Args: cobra.MaximumNArgs(1),
	RunE: RunConnect,
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

// RunConnect は環境に接続するコマンドの実行関数
func RunConnect(_ *cobra.Command, args []string) error {
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("設定が読み込まれていません")
	}

	// 環境名を取得（引数がなければ選択）
	envName, err := selectEnvironment(cfg, args)
	if err != nil {
		return err
	}

	// 環境の設定を取得
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// データベース一覧を取得
	dbNames, err := cfg.ListDatabases(envName)
	if err != nil {
		return err
	}

	if len(dbNames) == 0 {
		return fmt.Errorf("環境 %s にデータベースが設定されていません", envName)
	}

	// データベースを選択
	selectedDB, err := selectDatabase(dbNames)
	if err != nil {
		return err
	}

	// 環境とデータベースの設定を取得
	_, db, err := cfg.GetDatabase(envName, selectedDB)
	if err != nil {
		return err
	}

	// 接続情報を表示
	fmt.Println()
	fmt.Printf("環境: %s\n", envName)
	fmt.Printf("データベース: %s\n", selectedDB)
	fmt.Printf("RDSホスト: %s\n", db.RDSHost)
	fmt.Printf("ローカルポート: %d\n", db.LocalPort)
	fmt.Printf("AWSプロファイル: %s\n", env.AWSProfile)
	fmt.Printf("踏み台サーバー: %s\n", env.SSMTarget)
	fmt.Println()

	// TODO: AWS Secrets Managerからパスワードを取得
	// TODO: SSMポートフォワーディングを開始
	// TODO: Sequel Aceを起動

	fmt.Println("[未実装] AWS連携・Sequel Ace起動は後続タスクで実装します")

	return nil
}

// selectEnvironment は環境を選択する
// 引数があれば解決、なければTUIで選択
func selectEnvironment(cfg *config.Config, args []string) (string, error) {
	if len(args) > 0 {
		return cfg.ResolveEnvironment(args[0])
	}

	envNames := cfg.ListEnvironments()
	if len(envNames) == 0 {
		return "", fmt.Errorf("環境が設定されていません")
	}
	if len(envNames) == 1 {
		fmt.Printf("環境: %s\n", envNames[0])
		return envNames[0], nil
	}

	return tui.Select("接続する環境を選択してください", envNames)
}

// selectDatabase はデータベースを選択する
// 選択肢が1つの場合は自動選択、複数の場合はTUIで選択
func selectDatabase(dbNames []string) (string, error) {
	if len(dbNames) == 1 {
		fmt.Printf("データベース: %s\n", dbNames[0])
		return dbNames[0], nil
	}

	return tui.Select("接続するデータベースを選択してください", dbNames)
}
