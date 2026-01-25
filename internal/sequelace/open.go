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

// Open はSequel Aceを起動してDBに接続する
// -W オプションにより、Sequel Ace終了まで待機可能なProcessを返す
func Open(user, password, host string, port int, database string) (*Process, error) {
	// パスワードをURLエンコード
	encodedPassword := url.QueryEscape(password)

	// mysql:// URL schemeを構築
	mysqlURL := fmt.Sprintf("mysql://%s:%s@%s:%d/%s",
		user,
		encodedPassword,
		host,
		port,
		database,
	)

	// open -W -a "Sequel Ace" で起動（-W: アプリ終了まで待機）
	cmd := exec.Command("open", "-W", mysqlURL, "-a", "Sequel Ace")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sequel Aceの起動に失敗: %w", err)
	}

	return &Process{cmd: cmd}, nil
}

// Wait はSequel Aceの終了を待機する
func (p *Process) Wait() error {
	if p.cmd != nil {
		return p.cmd.Wait()
	}
	return nil
}
