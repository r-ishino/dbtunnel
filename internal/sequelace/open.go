package sequelace

import (
	"fmt"
	"net/url"
	"os/exec"
)

// Process はSequel Aceプロセスを表す
type Process struct {
	cmd *exec.Cmd
}

// ConnectionInfo は接続情報を表す
type ConnectionInfo struct {
	User     string
	Password string
	Host     string
	Port     int
	Database string
	Label    string // 接続名（タブに表示される）
}

// BuildMySQLURL はmysql:// URL schemeを構築する
func BuildMySQLURL(user, password, host string, port int, database string) string {
	encodedPassword := url.QueryEscape(password)
	return fmt.Sprintf("mysql://%s:%s@%s:%d/%s",
		user,
		encodedPassword,
		host,
		port,
		database,
	)
}

// Open はSequel Aceを起動してDBに接続する
// -W オプションにより、Sequel Ace終了まで待機可能なProcessを返す
func Open(user, password, host string, port int, database string) (*Process, error) {
	mysqlURL := BuildMySQLURL(user, password, host, port, database)

	// open -W -a "Sequel Ace" で起動（-W: アプリ終了まで待機）
	cmd := exec.Command("open", "-W", mysqlURL, "-a", "Sequel Ace")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sequel Aceの起動に失敗: %w", err)
	}

	return &Process{cmd: cmd}, nil
}

// OpenMultipleInTabs は複数の接続をSequel Aceのタブで開く
func OpenMultipleInTabs(connections []ConnectionInfo) (*Process, error) {
	if len(connections) == 0 {
		return nil, fmt.Errorf("接続情報がありません")
	}

	// 全ての接続を開く（別ウィンドウで開く）
	for i, conn := range connections {
		mysqlURL := BuildMySQLURL(conn.User, conn.Password, conn.Host, conn.Port, conn.Database)
		cmd := exec.Command("open", "-a", "Sequel Ace", mysqlURL)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("sequel Aceの起動に失敗 (%s): %w", conn.Label, err)
		}
		// 最初以外は少し待機
		if i < len(connections)-1 {
			// 次のウィンドウを開く前に少し待つ
			waitCmd := exec.Command("sleep", "0.5")
			_ = waitCmd.Run()
		}
	}

	// 全ウィンドウをタブに統合
	if len(connections) > 1 {
		script := `
tell application "Sequel Ace"
	activate
end tell


tell application "System Events"
	tell process "Sequel Ace"
		tell menu bar 1
			tell menu bar item "Window"
				tell menu 1
					-- メニュー項目18番目が「すべてのウィンドウを結合」
					click menu item 18
				end tell
			end tell
		end tell
	end tell
end tell
`
		cmd := exec.Command("osascript", "-e", script)
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("タブ統合の警告: %v (%s)\n", err, string(output))
		}
	}

	// Sequel Aceの終了を待機するプロセスを作成
	waitCmd := exec.Command("osascript", "-e", waitScript())
	if err := waitCmd.Start(); err != nil {
		return nil, fmt.Errorf("待機プロセスの開始に失敗: %w", err)
	}

	return &Process{cmd: waitCmd}, nil
}

// waitScript はSequel Aceが終了するまで待機するAppleScriptを生成する
func waitScript() string {
	return `repeat while application "Sequel Ace" is running
	delay 1
end repeat`
}

// Wait はSequel Aceの終了を待機する
func (p *Process) Wait() error {
	if p.cmd != nil {
		return p.cmd.Wait()
	}
	return nil
}
