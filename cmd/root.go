package cmd

import (
	"fmt"
	"os"

	"github.com/ishinoryuuichi/dbtunnel/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "dbtunnel",
	Short: "DB接続を簡単にするCLIツール",
	Long: `dbtunnel は AWS SSM 経由で踏み台サーバーにポートフォワーディングを確立し、
Sequel Ace を起動して RDS に接続する CLI ツールです。

使用例:
  dbtunnel staging          # staging環境に接続（DB選択あり）
  dbtunnel production       # production環境に接続（DB選択あり）
  dbtunnel list             # 環境一覧を表示`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// list コマンド以外のサブコマンドでも設定ファイルを読み込む
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗しました: %w", err)
		}
		return nil
	},
}

// Execute はルートコマンドを実行する
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "設定ファイルのパス (デフォルト: ~/.dbtunnel/config.yaml)")
}

// GetConfig は読み込んだ設定を返す
func GetConfig() *config.Config {
	return cfg
}
