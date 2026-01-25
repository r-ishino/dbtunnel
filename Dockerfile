FROM golang:1.25-alpine

WORKDIR /app

# 必要なツールをインストール
RUN apk add --no-cache git curl

# ホットリロード用ツール（開発時に便利）
RUN go install github.com/air-verse/air@latest

# golangci-lint（lintツール）
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b /usr/local/bin v2.8.0

# ソースコードはdocker-compose.ymlでマウントされる
CMD ["air", "-c", ".air.toml"]
