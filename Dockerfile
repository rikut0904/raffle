# Goのイメージを連れてくる
FROM golang:1.26

# 作業するフォルダを決める
WORKDIR /app

# ファイルを全部コピーする
COPY . .

# 設定ファイル（.env）をコピーする（初心者向けに分かりやすく）
# 本番では docker run --env-file .env を使うのがかっこいいです！
COPY .env .env

# ライブラリを入れる
RUN go mod download

# アプリをビルドする
RUN go build -o main .

# 8080番ポートを使う
EXPOSE 8080

# アプリを動かす
CMD ["./main"]
