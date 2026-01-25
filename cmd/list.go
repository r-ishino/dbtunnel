package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "環境とデータベースの一覧を表示",
	Long:  `設定ファイルに定義されている環境とデータベースの一覧を表示します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("設定が読み込まれていません")
		}

		envNames := cfg.ListEnvironments()
		if len(envNames) == 0 {
			fmt.Println("環境が設定されていません。")
			return nil
		}

		for _, envName := range envNames {
			fmt.Printf("%s:\n", envName)

			dbNames, err := cfg.ListDatabases(envName)
			if err != nil {
				return err
			}

			for _, dbName := range dbNames {
				fmt.Printf("  - %s\n", dbName)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
