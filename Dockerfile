FROM golang:1.20
#FROM golang:1.20-alpine

# コンテナログイン時のディレクトリ指定
WORKDIR /app

# ホストのファイルをコンテナの作業ディレクトリにコピー
COPY . .

# 起動
CMD ["go", "run", "main.go"]
