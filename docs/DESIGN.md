# Design

## Title
README: fieldctlのマイグレーションとapi-server起動手順を追記

## Summary

# README: fieldctlのマイグレーションとapi-server起動手順を追記

## 背景
gcfm の README.md において、以下の運用上重要な手順が不足しており、新規参加者やCIジョブ作成時にハマりやすい状態です。
- fieldctl による DB マイグレーション手順
- api-server による API サーバー起動手順

提示コマンドをベースに、前提条件や検証方法を含む「コピペで再現可能」な説明へ拡充します。

## 目的
README を見れば、ローカル環境で以下が最短で再現できる状態にする。
- MySQL に対するスキーママイグレーション（必要に応じて seed投入）
- API サーバーの起動と疎通確認

## 要件
- README.md に以下の新規/改訂セクションを追加
  - 前提条件
    - MySQL 8.x がローカルで起動済み（例: localhost:3306）
    - データベース `gcfm` が存在する（なければ作成手順を記載）
    - 認証情報例: `root:rootpw`（安全な環境のみで使用。実運用では環境変数などを推奨）
    - 実行バイナリが存在: `bin/fieldctl` と `bin/api-server`（ビルド/取得方法を簡潔に記載）
    - 対応OS: macOS/Linux（Windows の場合のクォート等は備考/注意に記載）
  - データベースのマイグレーション
    - 提示コマンドをREADMEに掲載し、各フラグの意味を簡潔に補足
      - `--table-prefix="gcfm_"`: 作成/管理するテーブルのプレフィックス
      - `--db "root:rootpw@tcp(localhost:3306)/gcfm"`: MySQL の DSN
      - `--driver=mysql`: 使用するDBドライバ
      - `--seed`: 初期データ投入（必要な場合のみ）
      - `--schema public`: TODO: MySQL で必須か要確認（PostgreSQL向けの可能性）。不要ならREADMEから除外。
    - マイグレーションは冪等である旨の記載（同コマンドの再実行で破壊的変更がない想定。異なる場合は注意書き）
  - API サーバーの起動
    - 提示コマンドをREADMEに掲載し、各フラグの意味を簡潔に補足
      - `-addr=:18081`: リッスンアドレス
      - `--driver=mysql`, `--dsn "root:rootpw@tcp(localhost:3306)/gcfm"`
    - 起動ログの例（1〜2行）と終了方法（Ctrl+C）
    - 資格情報の直書きは開発環境のみで使用し、本番は環境変数/Secret を推奨（環境変数例は TODO: 正式名が不明のため後追い）
  - 動作確認
    - DB: `SHOW TABLES LIKE 'gcfm_%';` でテーブル作成確認
    - HTTP: ヘルスチェック/疎通用エンドポイントに対する curl 例を記載（例: `/healthz`）。TODO: 正式エンドポイントを確認して更新。
  - トラブルシューティング
    - Unknown database → DB 作成手順へ誘導
    - 認証失敗 → ユーザー/パス/権限の確認
    - ポート競合 → `-addr` の変更方法
- 記法/スタイル
  - コマンドは Markdown のコードブロックで記載し、macOS/Linux でコピペ実行可能な形にする
  - 危険操作（初期化/破壊的変更）がある場合は明記（現状なし想定。異なる場合は注意書き）

## 非機能(性能/アクセシビリティ/可観測性など必要なもののみ)
- 再現性: 新規メンバーが10分以内にローカルでマイグレーションとサーバー起動ができる内容
- セキュリティ（ドキュメント観点）: 実運用での資格情報直書き回避の指針を明記
- 可読性: セクション構成と箇条書きで迷わない記述

## 影響範囲
- ドキュメント（README.md）のみ。アプリケーションコード・スキーマ・CI定義には影響なし。

## テスト観点
- README を初見の開発者が手順通り実行して、以下を確認できる
  - MySQL に `gcfm_` プレフィックスのテーブルが作成される
  - API サーバーが `:18081` で起動し、疎通が取れる
  - `--seed` 指定時にサンプルデータ/初期データが投入される（何が入るかをREADMEに明記）
  - macOS（zsh）/Linux（bash）でコマンドがそのまま動作する
- コマンドと出力例（要点のみ）がREADMEに記載されている
- TODO 項目（ヘルスエンドポイント/--schema要否/推奨環境変数名）が解消または明示されている

## 想定コマンド
```bash
# 前提: DBがなければ作成
mysql -u root -prootpw -h 127.0.0.1 -e "CREATE DATABASE IF NOT EXISTS gcfm CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"

# マイグレーション（必要なら --schema は削除/修正）
bin/fieldctl db migrate \
  --table-prefix="gcfm_" \
  --db "root:rootpw@tcp(localhost:3306)/gcfm" \
  --schema public \
  --seed \
  --driver=mysql

# APIサーバー起動
bin/api-server -addr=:18081 --driver=mysql --dsn "root:rootpw@tcp(localhost:3306)/gcfm"

# 疎通確認（エンドポイントは要確認）
curl -sSf http://localhost:18081/healthz || echo "TODO: 正式なヘルスチェックエンドポイントをREADMEで案内してください"

# DBの確認例
echo "SHOW TABLES LIKE 'gcfm_%';" | mysql -u root -prootpw -h 127.0.0.1 gcfm
```

## 受け入れ条件
- READMEに「データベースのマイグレーション」セクションが追加され、提示コマンドがコピペで実行できる
- READMEに「APIサーバーの起動」セクションが追加され、提示コマンドがコピペで実行できる
- 前提条件（MySQL、DB作成、バイナリ配置/ビルド）が明記されている
- 動作確認手順（テーブル作成の確認とHTTP疎通確認）が記載されている
- --schemaフラグの要否に関する注記（または修正/TODO）が明記されている
- seedデータの有無と目的が説明されている
- macOS/Linuxでの動作が確認できるコマンド例になっている
- ドキュメント変更のみ（コード変更なし）で完了する

## 備考
リスク/補足:
- --schema フラグは MySQL では通常不要です（PostgreSQL向けの概念）。使用している fieldctl 実装が MySQL でも受け付ける/無視するか要確認。不要ならREADMEから除外してください。
- API のヘルスチェック/疎通エンドポイント名が不明のため README では TODO として明示しています。実装に合わせて確定してください（例: /healthz, /readyz, /version など）。
- バイナリ取得/ビルド方法（例: make build など）の標準手順があれば README へ追記してください。
- 認証情報はサンプルです。実運用では環境変数やSecret管理を推奨（変数名はプロジェクト標準に合わせて追記: 例 GCFM_DSN など。現時点では不明のため TODO）。
- Windows 環境ではクォートやパスの違いに注意が必要です（必要なら別途例をREADMEに追加）。

**Labels:** feature, ci:ship

## API / DB Changes
- TODO

## Test Cases
- TODO
