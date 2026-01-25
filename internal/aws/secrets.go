package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// GetSecret はSecrets Managerからシークレットを取得する
func GetSecret(ctx context.Context, profile, region, secretName string) (string, error) {
	// AWS設定を読み込み
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return "", fmt.Errorf("AWS設定の読み込みに失敗: %w", err)
	}

	// Secrets Managerクライアントを作成
	client := secretsmanager.NewFromConfig(cfg)

	// シークレットを取得
	result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return "", fmt.Errorf("シークレットの取得に失敗 (%s): %w", secretName, err)
	}

	if result.SecretString == nil {
		return "", fmt.Errorf("シークレットが空です: %s", secretName)
	}

	return *result.SecretString, nil
}

// GetPassword はSecrets ManagerからDBパスワードを取得する
// シークレットがJSON形式の場合は指定されたキー（デフォルト: "password"）を探し、
// プレーンテキストの場合はそのまま返す
func GetPassword(ctx context.Context, profile, region, secretName, secretKey string) (string, error) {
	secretValue, err := GetSecret(ctx, profile, region, secretName)
	if err != nil {
		return "", err
	}

	// JSONとしてパースを試みる
	var secretData map[string]string
	if err := json.Unmarshal([]byte(secretValue), &secretData); err == nil {
		// JSON形式の場合、指定されたキーを探す
		// キーが指定されていない場合は "password" を使用
		key := secretKey
		if key == "" {
			key = "password"
		}
		if password, ok := secretData[key]; ok {
			return password, nil
		}
		// キーが見つからなければエラー
		return "", fmt.Errorf("シークレットに '%s' キーが見つかりません: %s", key, secretName)
	}

	// プレーンテキストの場合はそのまま返す
	return secretValue, nil
}
