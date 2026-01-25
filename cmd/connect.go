package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ishinoryuuichi/dbtunnel/internal/aws"
	"github.com/ishinoryuuichi/dbtunnel/internal/config"
	"github.com/ishinoryuuichi/dbtunnel/internal/sequelace"
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
	fmt.Println()

	// コンテキストとシグナルハンドリングの設定
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// シグナルを受け取るチャンネル
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 1. AWS Secrets Managerからパスワードを取得
	fmt.Print("パスワードを取得中...")
	password, err := aws.GetPassword(ctx, env.AWSProfile, env.AWSRegion, db.SecretName, db.SecretKey)
	if err != nil {
		fmt.Println(" 失敗")
		return fmt.Errorf("パスワードの取得に失敗: %w", err)
	}
	fmt.Println(" 完了")

	// 2. SSMポートフォワーディングを開始
	fmt.Print("SSMポートフォワーディングを開始中...")
	// データベース固有のSSMターゲットがあればそちらを優先
	ssmTarget := env.SSMTarget
	if db.SSMTarget != "" {
		ssmTarget = db.SSMTarget
	}
	session, err := aws.StartPortForwarding(
		ctx,
		env.AWSProfile,
		env.AWSRegion,
		ssmTarget,
		db.RDSHost,
		db.Port,
		db.LocalPort,
	)
	if err != nil {
		fmt.Println(" 失敗")
		return fmt.Errorf("SSMポートフォワーディングの開始に失敗: %w", err)
	}
	fmt.Println(" 完了")

	// クリーンアップ用のdefer
	defer func() {
		fmt.Print("\nSSMセッションを終了中...")
		if err := session.Stop(); err != nil {
			fmt.Printf(" エラー: %v\n", err)
		} else {
			fmt.Println(" 完了")
		}
	}()

	// 3. Sequel Aceを起動
	fmt.Print("Sequel Aceを起動中...")
	sequelAce, err := sequelace.Open(db.User, password, "127.0.0.1", db.LocalPort, db.Database)
	if err != nil {
		fmt.Println(" 失敗")
		return fmt.Errorf("sequel Aceの起動に失敗: %w", err)
	}
	fmt.Println(" 完了")

	fmt.Println()
	fmt.Println("接続が確立されました。")
	fmt.Println("Sequel Aceを終了するか、Ctrl+C を押すとセッションが終了します。")

	// Sequel Ace終了を待機するgoroutine
	sequelAceDone := make(chan struct{})
	go func() {
		_ = sequelAce.Wait()
		close(sequelAceDone)
	}()

	// Sequel Ace終了 または Ctrl+C を待機
	select {
	case <-sequelAceDone:
		fmt.Println("\nSequel Aceが終了しました。")
	case <-sigChan:
		fmt.Println("\n終了シグナルを受信しました。")
	}

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
