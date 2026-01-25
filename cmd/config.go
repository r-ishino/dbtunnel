package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "設定ファイルを開く",
	Long:  `設定ファイル（~/.dbtunnel/config.yaml）をデフォルトのエディタで開きます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := cfgFile
		if configPath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("ホームディレクトリの取得に失敗しました: %w", err)
			}
			configPath = filepath.Join(home, ".dbtunnel", "config.yaml")
		}

		// ファイルの存在確認
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("設定ファイルが存在しません: %s", configPath)
		}

		// macOSのopenコマンドで設定ファイルを開く
		openCmd := exec.Command("open", configPath)
		if err := openCmd.Run(); err != nil {
			return fmt.Errorf("設定ファイルを開けませんでした: %w", err)
		}

		fmt.Printf("設定ファイルを開きました: %s\n", configPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
