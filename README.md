## 構築手順

### 1. 設定ファイル（.env）の準備

まず、アプリケーションの設定ファイルを作成する。

```bash
cp .env.example .env
```

`.env` ファイルを開き、以下の項目を環境に合わせて編集してください。
- `DATABASE_URL`: 接続先データベースのURL
- `SESSION_SECRET`: 32文字以上のランダムな文字列（`openssl rand -base64 32` などで生成）
- Firebase関連の各項目

### 2. Dockerでの起動

```bash
# イメージのビルド
docker build -t raffle-app .

# コンテナの起動
docker run -p 8080:8080 raffle-app
```

起動後、ブラウザで `http://localhost:8080` にアクセスする。

---

## Docker利用時の注意点

- `.env` ファイルの内容を書き換えた後は、イメージの再ビルドする必要がある。
- すでにPC上で8080番ポートを使用しているアプリがある場合、起動に失敗しするため、すでに起動しているアプリを閉じるかポート番号を変更する。
