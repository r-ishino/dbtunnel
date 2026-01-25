package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/viper"
)

// Database はアプリケーションごとのDB接続情報
type Database struct {
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
	AWSProfile string              `mapstructure:"aws_profile"`
	AWSRegion  string              `mapstructure:"aws_region"`
	SSMTarget  string              `mapstructure:"ssm_target"`
	Databases  map[string]Database `mapstructure:"databases"`
}

// Config はアプリケーション全体の設定
type Config struct {
	Environments map[string]Environment `mapstructure:"environments"`
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

	return &cfg, nil
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

// ListEnvironments は環境名の一覧を返す
func (c *Config) ListEnvironments() []string {
	names := make([]string, 0, len(c.Environments))
	for name := range c.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListDatabases は指定された環境のデータベース名一覧を返す
func (c *Config) ListDatabases(envName string) ([]string, error) {
	env, err := c.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(env.Databases))
	for name := range env.Databases {
		names = append(names, name)
	}
	sort.Strings(names)
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
