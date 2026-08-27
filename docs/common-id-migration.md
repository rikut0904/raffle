# Firebase users から Common ID への移行

`cmd/migrate-users` は Firebase Admin SDK で既存ユーザーを一覧取得し、Common ID の migration API に最大 1,000 件ずつ送信します。パスワード、Firebase ID token、サービスアカウントの内容は Common ID へ送信しません。

まず dry-run を実行します。

```bash
go run ./cmd/migrate-users \
	-endpoint "$COMMON_ID_API_ORIGIN" \
	-api-key "$COMMON_ID_API_KEY" \
	-client-id "$COMMON_ID_CLIENT_ID" \
  -firebase-project-id "$FIREBASE_PROJECT_ID" \
	-firebase-credentials "$FIREBASE_SERVICE_ACCOUNT_JSON" \
  -dry-run=true
```

出力を確認後、Common ID 側でパスワード再設定メールの送信設定を確認し、次を実行します。本移行では `DATABASE_URL` のDBをトランザクションで直接更新します。実行前に必ずバックアップと件数確認を行ってください。

```bash
go run ./cmd/migrate-users \
	-endpoint "$COMMON_ID_API_ORIGIN" \
	-api-key "$COMMON_ID_API_KEY" \
	-client-id "$COMMON_ID_CLIENT_ID" \
  -firebase-project-id "$FIREBASE_PROJECT_ID" \
	-firebase-credentials "$FIREBASE_SERVICE_ACCOUNT_JSON" \
	-dry-run=false \
	-update-db=true
```

同じ `application_id` と Firebase UID の組み合わせは Common ID 側の移行台帳で冪等に処理されます。DB更新も、既に新UIDへ移行済みの行はスキップします。

`cmd/migrate-users` は実行時にプロジェクト直下の `.env` を自動で読み込みます。`FIREBASE_SERVICE_ACCOUNT_JSON` にはファイルパスではなくサービスアカウントJSONそのものを設定します。`.env` は必ずGit管理対象外にし、本番ではsecret storeから環境変数として注入してください。

`COMMON_ID_ORIGIN` はブラウザ用ポータルURL（localでは `http://localhost:13000`）、`COMMON_ID_API_ORIGIN` は移行APIを提供するBackend URL（localでは `http://localhost:18080`）です。移行スクリプトは `COMMON_ID_API_ORIGIN` を使用します。

`COMMON_ID_API_KEY` はアプリ登録時に発行されたアプリAPIキーです。Common ID側でmigration endpointのアプリキー検証を有効にした後、OAuth token交換とユーザー移行の両方にこのキーを使用します。

## GitHub Actionsから実行する

`.github/workflows/migrate-users.yml` は `workflow_dispatch` から手動実行できます。次のRepository Secretsを登録してください。

- `COMMON_ID_API_ORIGIN`
- `COMMON_ID_API_KEY`
- `COMMON_ID_CLIENT_ID`
- `FIREBASE_PROJECT_ID`
- `FIREBASE_SERVICE_ACCOUNT_JSON`（JSON本文そのもの）
- `DATABASE_URL`（DB更新時のみ必須）

GitHubのActions画面で `dry_run=true`、`update_db=false` を選ぶと確認用のdry-runになります。本移行は出力内容を確認したうえで、`dry_run=false`、`update_db=true` を選択して実行してください。

workflowには `common-id-migration` Environmentを設定しています。本移行を承認制にする場合は、Repository settingsのEnvironmentsで同名Environmentを作成し、Required reviewersを設定してください。サービスアカウントJSON、APIキー、DB接続情報はログへ出力せず、GitHub Secretsにのみ保存してください。
