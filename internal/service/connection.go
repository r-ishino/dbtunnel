// Package service は接続管理のビジネスロジックを提供する
package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/r-ishino/dbtunnel/internal/aws"
	"github.com/r-ishino/dbtunnel/internal/config"
	"github.com/r-ishino/dbtunnel/internal/sequelace"
	"golang.org/x/sync/errgroup"
)

// Connection は単一のDB接続情報を保持する
type Connection struct {
	EnvName   string
	DBName    string
	Env       *config.Environment
	DB        *config.Database
	SSMTarget string
	password  string
	session   *aws.PortForwardingSession
}

// ConnectionManager は複数のDB接続を管理する
type ConnectionManager struct {
	connections []*Connection
}

// NewConnectionManager は新しいConnectionManagerを作成する
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make([]*Connection, 0),
	}
}

// BuildConnections は選択されたDBの接続情報を構築する
func (m *ConnectionManager) BuildConnections(cfg *config.Config, envName string, dbNames []string) error {
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	for _, dbName := range dbNames {
		_, db, err := cfg.GetDatabase(envName, dbName)
		if err != nil {
			return err
		}

		ssmTarget := env.SSMTarget
		if db.SSMTarget != "" {
			ssmTarget = db.SSMTarget
		}

		m.connections = append(m.connections, &Connection{
			EnvName:   envName,
			DBName:    dbName,
			Env:       env,
			DB:        db,
			SSMTarget: ssmTarget,
		})
	}

	return nil
}

// EstablishConnections はすべての接続を並行で確立する（パスワード取得、SSMポートフォワーディング）
func (m *ConnectionManager) EstablishConnections(ctx context.Context) error {
	total := len(m.connections)
	fmt.Printf("接続を確立中... (%d件を並行処理)\n", total)

	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex // 標準出力の排他制御用

	for _, conn := range m.connections {
		g.Go(func() error {
			// パスワードを取得
			password, err := aws.GetPassword(ctx, conn.Env.AWSProfile, conn.Env.AWSRegion, conn.DB.SecretName, conn.DB.SecretKey)
			if err != nil {
				mu.Lock()
				fmt.Printf("  %s: パスワード取得失敗\n", conn.DBName)
				mu.Unlock()
				return fmt.Errorf("%s のパスワード取得に失敗: %w", conn.DBName, err)
			}
			conn.password = password

			mu.Lock()
			fmt.Printf("  %s: パスワード取得完了\n", conn.DBName)
			mu.Unlock()

			// SSMポートフォワーディングを開始
			session, err := aws.StartPortForwarding(
				ctx,
				conn.Env.AWSProfile,
				conn.Env.AWSRegion,
				conn.SSMTarget,
				conn.DB.RDSHost,
				conn.DB.Port,
				conn.DB.LocalPort,
			)
			if err != nil {
				mu.Lock()
				fmt.Printf("  %s: SSMポートフォワーディング失敗\n", conn.DBName)
				mu.Unlock()
				return fmt.Errorf("%s のSSMポートフォワーディング開始に失敗: %w", conn.DBName, err)
			}
			conn.session = session

			mu.Lock()
			fmt.Printf("  %s: SSMポートフォワーディング開始完了\n", conn.DBName)
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// エラー発生時は確立済みのセッションをクリーンアップ
		m.cleanupEstablishedSessions()
		return err
	}

	fmt.Println("すべての接続が確立されました")
	return nil
}

// cleanupEstablishedSessions は確立済みのSSMセッションのみを終了する
func (m *ConnectionManager) cleanupEstablishedSessions() {
	for _, conn := range m.connections {
		if conn.session != nil {
			_ = conn.session.Stop()
			conn.session = nil
		}
	}
}

// OpenSequelAce はSequel Aceを起動する
func (m *ConnectionManager) OpenSequelAce() (*sequelace.Process, error) {
	fmt.Print("Sequel Aceを起動中...")

	var proc *sequelace.Process
	var err error

	if len(m.connections) == 1 {
		conn := m.connections[0]
		proc, err = sequelace.Open(conn.DB.User, conn.password, "127.0.0.1", conn.DB.LocalPort, conn.DB.Database)
	} else {
		var connInfos []sequelace.ConnectionInfo
		for _, conn := range m.connections {
			connInfos = append(connInfos, sequelace.ConnectionInfo{
				User:     conn.DB.User,
				Password: conn.password,
				Host:     "127.0.0.1",
				Port:     conn.DB.LocalPort,
				Database: conn.DB.Database,
				Label:    conn.DBName,
			})
		}
		proc, err = sequelace.OpenMultipleInTabs(connInfos)
	}

	if err != nil {
		fmt.Println(" 失敗")
		return nil, fmt.Errorf("sequel Aceの起動に失敗: %w", err)
	}
	fmt.Println(" 完了")

	return proc, nil
}

// Cleanup はすべてのSSMセッションを終了する
func (m *ConnectionManager) Cleanup() {
	fmt.Print("\nSSMセッションを終了中...")
	for _, conn := range m.connections {
		if conn.session != nil {
			_ = conn.session.Stop()
		}
	}
	fmt.Println(" 完了")
}

// WaitForTermination はSequel Ace終了・シグナル・SSMセッションの切断のいずれかを待機する
func (m *ConnectionManager) WaitForTermination(proc *sequelace.Process) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sequelAceDone := make(chan struct{})
	go func() {
		_ = proc.Wait()
		close(sequelAceDone)
	}()

	disconnected := make(chan *Connection, len(m.connections))
	for _, conn := range m.connections {
		go func() {
			<-conn.session.Done()
			disconnected <- conn
		}()
	}

	select {
	case <-sequelAceDone:
		fmt.Println("\nSequel Aceが終了しました。")
	case <-sigChan:
		fmt.Println("\n終了シグナルを受信しました。")
	case conn := <-disconnected:
		fmt.Printf("\n%s のSSMセッションが切断されました: %v\n", conn.DBName, conn.session.Err())
	}
}

// Connections は接続情報のスライスを返す
func (m *ConnectionManager) Connections() []*Connection {
	return m.connections
}
