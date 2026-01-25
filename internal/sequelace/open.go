package sequelace

import (
	"fmt"
	"net/url"
	"os/exec"
)

// Open はSequel Aceを起動してDBに接続する
func Open(user, password, host string, port int, database string) error {
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

	// open -a "Sequel Ace" で起動
	cmd := exec.Command("open", mysqlURL, "-a", "Sequel Ace")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sequel Aceの起動に失敗: %w", err)
	}

	return nil
}
