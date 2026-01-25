package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Database はアプリケーションごとのDB接続情報
type Database struct {
	SSMTarget  string `mapstructure:"ssm_target"`  // データベース固有の踏み台サーバー（省略時は環境のSSMTargetを使用）
	RDSHost    string `mapstructure:"rds_host"`
	Port       int    `mapstructure:"port"`
	LocalPort  int    `mapstructure:"local_port"`
	Database   string `mapstructure:"database"`
	User       string `mapstructure:"user"`
	SecretName string `mapstructure:"secret_name"`
	SecretKey  string `mapstructure:"secret_key"` // JSON形式シークレットのキー名（省略時は "password"）
}

// Environment は環境ごとの設定（AWS情報 + 複数のDB）
type Environment struct {
	AWSProfile    string              `mapstructure:"aws_profile"`
	AWSRegion     string              `mapstructure:"aws_region"`
	SSMTarget     string              `mapstructure:"ssm_target"`
	Databases     map[string]Database `mapstructure:"databases"`
	databaseOrder []string            // YAMLファイルでの定義順序
}

// Config はアプリケーション全体の設定
type Config struct {
	Environments     map[string]Environment `mapstructure:"environments"`
	environmentOrder []string               // YAMLファイルでの定義順序
}

// Load は設定ファイルを読み込む
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, ".dbtunnel", "config.yaml")
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// YAMLファイルから順序情報を取得
	if err := cfg.loadOrder(configPath); err != nil {
		return nil, fmt.Errorf("failed to load order from config file: %w", err)
	}

	return &cfg, nil
}

// loadOrder はYAMLファイルから環境・データベースの定義順序を取得する
func (c *Config) loadOrder(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	// ルートはDocumentNode、その中にMappingNodeがある
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}

	rootMap := root.Content[0]
	if rootMap.Kind != yaml.MappingNode {
		return nil
	}

	// environments を探す
	for i := 0; i < len(rootMap.Content); i += 2 {
		if rootMap.Content[i].Value == "environments" {
			envsNode := rootMap.Content[i+1]
			if envsNode.Kind != yaml.MappingNode {
				continue
			}

			// 環境の順序を取得
			for j := 0; j < len(envsNode.Content); j += 2 {
				envName := envsNode.Content[j].Value
				c.environmentOrder = append(c.environmentOrder, envName)

				// 各環境内のdatabasesの順序を取得
				envNode := envsNode.Content[j+1]
				if envNode.Kind != yaml.MappingNode {
					continue
				}

				dbOrder := extractDatabaseOrder(envNode)
				if env, ok := c.Environments[envName]; ok {
					env.databaseOrder = dbOrder
					c.Environments[envName] = env
				}
			}
			break
		}
	}

	return nil
}

// extractDatabaseOrder は環境ノードからdatabasesの順序を取得する
func extractDatabaseOrder(envNode *yaml.Node) []string {
	var order []string
	for i := 0; i < len(envNode.Content); i += 2 {
		if envNode.Content[i].Value == "databases" {
			dbsNode := envNode.Content[i+1]
			if dbsNode.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j < len(dbsNode.Content); j += 2 {
				order = append(order, dbsNode.Content[j].Value)
			}
			break
		}
	}
	return order
}

// GetEnvironment は指定された環境の設定を取得する
func (c *Config) GetEnvironment(name string) (*Environment, error) {
	env, ok := c.Environments[name]
	if !ok {
		return nil, fmt.Errorf("environment not found: %s", name)
	}
	return &env, nil
}

// GetDatabase は指定された環境・データベースの設定を取得する
func (c *Config) GetDatabase(envName, dbName string) (*Environment, *Database, error) {
	env, err := c.GetEnvironment(envName)
	if err != nil {
		return nil, nil, err
	}

	db, ok := env.Databases[dbName]
	if !ok {
		return nil, nil, fmt.Errorf("database not found: %s in environment %s", dbName, envName)
	}

	return env, &db, nil
}

// ListEnvironments は環境名の一覧を返す（YAMLファイルの定義順）
func (c *Config) ListEnvironments() []string {
	// 順序情報がある場合はそれを使用
	if len(c.environmentOrder) > 0 {
		return c.environmentOrder
	}

	// フォールバック: mapから取得（順序不定）
	names := make([]string, 0, len(c.Environments))
	for name := range c.Environments {
		names = append(names, name)
	}
	return names
}

// ListDatabases は指定された環境のデータベース名一覧を返す（YAMLファイルの定義順）
func (c *Config) ListDatabases(envName string) ([]string, error) {
	env, err := c.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	// 順序情報がある場合はそれを使用
	if len(env.databaseOrder) > 0 {
		return env.databaseOrder, nil
	}

	// フォールバック: mapから取得（順序不定）
	names := make([]string, 0, len(env.Databases))
	for name := range env.Databases {
		names = append(names, name)
	}
	return names, nil
}

// 環境名のエイリアス定義
var environmentAliases = map[string][]string{
	"production":  {"prd", "prod", "p"},
	"staging":     {"stg", "stag", "s"},
	"development": {"dev", "d"},
}

// ResolveEnvironment は入力された文字列から環境名を解決する
// 優先順位: 完全一致 → エイリアス → プレフィックスマッチング
func (c *Config) ResolveEnvironment(input string) (string, error) {
	// 完全一致をチェック
	if _, ok := c.Environments[input]; ok {
		return input, nil
	}

	// エイリアスマッチング
	for envName, aliases := range environmentAliases {
		if _, exists := c.Environments[envName]; !exists {
			continue
		}
		for _, alias := range aliases {
			if input == alias {
				return envName, nil
			}
		}
	}

	// プレフィックスマッチング
	var matches []string
	for name := range c.Environments {
		if len(input) <= len(name) && name[:len(input)] == input {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		envList := c.ListEnvironments()
		return "", fmt.Errorf("環境が見つかりません: %s (利用可能: %v)", input, envList)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("'%s' に複数の環境がマッチします: %v", input, matches)
	}
}
