.PHONY: dev build shell init lint fmt

# 開発サーバー起動（ホットリロード）
dev:
	docker compose up --build

# macOS用バイナリをビルド（クロスコンパイル）
build:
	docker compose run --rm -e GOOS=darwin -e GOARCH=arm64 dev go build -o dbtunnel .

# Intel Mac用バイナリをビルド
build-intel:
	docker compose run --rm -e GOOS=darwin -e GOARCH=amd64 dev go build -o dbtunnel .

# コンテナ内でシェルを起動
shell:
	docker compose run --rm dev sh

# プロジェクト初期化
init:
	docker compose run --rm dev go mod init github.com/ishinoryuuichi/dbtunnel
	docker compose run --rm dev go mod tidy

# Lint実行
lint:
	docker compose run --rm dev golangci-lint run

# コードフォーマット
fmt:
	docker compose run --rm dev golangci-lint fmt
