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

// dbConnection は接続に必要な情報をまとめた構造体
type dbConnection struct {
	envName   string
	dbName    string
	env       *config.Environment
	db        *config.Database
	password  string
	session   *aws.PortForwardingSession
	ssmTarget string
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

	// データベースを選択（複数選択可能）
	selectedDBs, err := selectDatabases(dbNames)
	if err != nil {
		return err
	}

	// 接続情報を構築
	var connections []*dbConnection
	for _, dbName := range selectedDBs {
		_, db, err := cfg.GetDatabase(envName, dbName)
		if err != nil {
			return err
		}
		ssmTarget := env.SSMTarget
		if db.SSMTarget != "" {
			ssmTarget = db.SSMTarget
		}
		connections = append(connections, &dbConnection{
			envName:   envName,
			dbName:    dbName,
			env:       env,
			db:        db,
			ssmTarget: ssmTarget,
		})
	}

	// 接続情報を表示
	fmt.Println()
	fmt.Printf("環境: %s\n", envName)
	fmt.Printf("データベース: %v\n", selectedDBs)
	fmt.Println()

	// コンテキストとシグナルハンドリングの設定
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// シグナルを受け取るチャンネル
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// クリーンアップ関数
	cleanup := func() {
		fmt.Print("\nSSMセッションを終了中...")
		for _, conn := range connections {
			if conn.session != nil {
				_ = conn.session.Stop()
			}
		}
		fmt.Println(" 完了")
	}
	defer cleanup()

	// 各DBに対してパスワード取得とSSMポートフォワーディングを開始
	for i, conn := range connections {
		// パスワードを取得
		fmt.Printf("[%d/%d] %s: パスワードを取得中...", i+1, len(connections), conn.dbName)
		password, err := aws.GetPassword(ctx, conn.env.AWSProfile, conn.env.AWSRegion, conn.db.SecretName, conn.db.SecretKey)
		if err != nil {
			fmt.Println(" 失敗")
			return fmt.Errorf("%s のパスワード取得に失敗: %w", conn.dbName, err)
		}
		conn.password = password
		fmt.Println(" 完了")

		// SSMポートフォワーディングを開始
		fmt.Printf("[%d/%d] %s: SSMポートフォワーディングを開始中...", i+1, len(connections), conn.dbName)
		session, err := aws.StartPortForwarding(
			ctx,
			conn.env.AWSProfile,
			conn.env.AWSRegion,
			conn.ssmTarget,
			conn.db.RDSHost,
			conn.db.Port,
			conn.db.LocalPort,
		)
		if err != nil {
			fmt.Println(" 失敗")
			return fmt.Errorf("%s のSSMポートフォワーディング開始に失敗: %w", conn.dbName, err)
		}
		conn.session = session
		fmt.Println(" 完了")
	}

	// Sequel Aceを起動
	fmt.Print("Sequel Aceを起動中...")
	var sequelAce *sequelace.Process

	if len(connections) == 1 {
		// 単一接続の場合は従来通り
		conn := connections[0]
		sequelAce, err = sequelace.Open(conn.db.User, conn.password, "127.0.0.1", conn.db.LocalPort, conn.db.Database)
	} else {
		// 複数接続の場合はタブで開く
		var connInfos []sequelace.ConnectionInfo
		for _, conn := range connections {
			connInfos = append(connInfos, sequelace.ConnectionInfo{
				User:     conn.db.User,
				Password: conn.password,
				Host:     "127.0.0.1",
				Port:     conn.db.LocalPort,
				Database: conn.db.Database,
				Label:    conn.dbName,
			})
		}
		sequelAce, err = sequelace.OpenMultipleInTabs(connInfos)
	}

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

// selectDatabases はデータベースを選択する（複数選択可能）
// 選択肢が1つの場合は自動選択、複数の場合はTUIで複数選択
func selectDatabases(dbNames []string) ([]string, error) {
	if len(dbNames) == 1 {
		fmt.Printf("データベース: %s\n", dbNames[0])
		return dbNames, nil
	}

	return tui.SelectMultiple("接続するデータベースを選択してください（スペースで選択）", dbNames)
}
