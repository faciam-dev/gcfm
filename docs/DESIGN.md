# Design

## Title
READMEにfieldctlのMigration手順とapi-server起動方法を追記する

## Summary

# READMEにfieldctlのMigration手順とapi-server起動方法を追記する

## 背景
gcfm の README.md において、以下の手順説明が不足しており、初学者/新規参画者がセットアップで詰まっています。
- fieldctl を用いた DB マイグレーション（seed 含む）
- api-server による API サーバー起動と動作確認

## 目的
README を修正し、ローカル開発環境で以下がコピー＆ペーストで再現できるようにします。
- MySQL へのスキーマ適用（プレフィックス gcfm_、seed 投入）
- API サーバーの起動とヘルス確認
- 代表的な失敗時の対処

## 要件
- 新規見出しを追加：
  - 「Database migration (fieldctl)」
  - 「API サーバー起動 (api-server)」
- 前提条件を明記：
  - ローカルに MySQL が動作（例: localhost:3306、ユーザー root、パスワード rootpw、データベース gcfm）
  - bin/fieldctl と bin/api-server が実行可能であること（パス/権限）
- マイグレーション手順：
  - DB 作成例（utf8mb4）
  - fieldctl による migrate コマンド例（下記 想定コマンドのとおり）
  - フラグの意味を簡潔に説明：
    - --table-prefix: 生成テーブル接頭辞（例: gcfm_）
    - --db/--dsn: DB 接続文字列（MySQL DSN）
    - --driver: ドライバ指定（mysql）
    - --schema: スキーマ指定（MySQLでの挙動について注記）
    - --seed: 開発用の初期データ投入
  - 実行後の検証方法（テーブル/レコード確認）
- API サーバー起動手順：
  - 起動コマンド例（:18081 でリッスン）
  - ヘルスチェック/簡易動作確認（curl 例）
  - ログ例の説明（起動成功/エラー）
- セキュリティ注意：
  - README の資格情報は開発用途限定、商用/共有環境での流用禁止
- トラブルシューティングを追記：
  - 接続失敗（認証/ホスト/ポート）
  - 権限不足（CREATE/ALTER 権限）
  - ポート競合（:18081 使用中）
  - DSN の書式誤り
- 可能であれば補助セクション：
  - 環境変数での設定例（例: GCFM_DSN, GCFM_ADDR）【TODO: 環境変数キー要確認】
  - Docker/MySQL コンテナでの最小手順【任意/TODO】

## 非機能(性能/アクセシビリティ/可観測性など必要なもののみ)
- 再現性：記載コマンドをそのまま実行して成功率が高い（95%以上）
- 可読性：3分以内に通読・実行可能な分量に整理
- 移植性：macOS/Linux（WSL含む）での手順差異がある場合は注記
- 安全性：開発用資格情報の取り扱いに明確な警告を付記

## 影響範囲
- gcfm/README.md（ドキュメントのみ）
- 参照されるクイックスタート/セットアップガイドがある場合は整合性確認
- CI やテンプレートスクリプトで README の手順を参照している場合はリンク修正の可能性

## テスト観点
- クリーン環境（新規 DB）で手順どおりに実行し成功すること
- マイグレーション後、gcfm_ で始まるテーブルが作成されていること
- --seed 指定時に最低1件のサンプルデータが投入されていること【TODO: 対象テーブル要確認】
- api-server が :18081 で待受し、ヘルスエンドポイントが 200 を返すこと【/health などエンドポイント名要確認】
- 代表的な失敗時に、README の対処で復旧できること（DSN 誤り、権限不足、ポート競合）

## 想定コマンド
```bash
# 1) DB 作成（必要に応じて）
mysql -uroot -prootpw -h 127.0.0.1 -P 3306 \
  -e "CREATE DATABASE IF NOT EXISTS gcfm DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"

# 2) マイグレーションと seed（MySQL）
bin/fieldctl db migrate \
  --table-prefix="gcfm_" \
  --db "root:rootpw@tcp(localhost:3306)/gcfm" \
  --schema public \
  --seed \
  --driver=mysql

# 3) 起動（MySQL DSN を指定）
bin/api-server -addr=:18081 \
  --driver=mysql \
  --dsn "root:rootpw@tcp(localhost:3306)/gcfm"

# 4) 動作確認（ヘルスチェックの例。実際のパスは要確認）
curl -i http://localhost:18081/health || true

# 5) テーブル作成確認（例）
mysql -uroot -prootpw -h 127.0.0.1 -P 3306 gcfm -e "SHOW TABLES LIKE 'gcfm_%';"
```

## 受け入れ条件
- README.mdに「Database migration(fieldctl)」「APIサーバー起動(api-server)」の2節が追加されている
- 記載のコマンドをコピペで実行するとMySQLにgcfm_プレフィックスのテーブルが作成され、seedが投入される
- api-serverを:18081で起動でき、/health(もしくは相当のヘルスチェック)が200を返す検証手順がREADMEに含まれる
- 前提条件(必要なバイナリ/MySQLの用意/DB作成/権限)がREADMEに明記されている
- トラブルシューティング(接続失敗/権限不足/ポート競合/DSN誤り)がREADMEに1つ以上含まれる
- DSN/フラグ(--driver, --dsn, --table-prefix, --schema, --seed)の意味がREADMEで簡潔に説明されている
- ローカル用の資格情報は開発用途限定である旨の注意書きがREADMEに含まれる

## 備考
不確定事項/リスク:
- --schema フラグは MySQL では一般的でないため、意味/必要性を実装側に確認が必要（PostgreSQL 互換の可能性）。
- ヘルスチェックの正確なエンドポイント（/health や /ready など）と応答仕様を確認し記載更新が必要。
- seed の内容（対象テーブル、件数）と期待挙動の確定が必要。
- bin/fieldctl, bin/api-server の配置と実行権限（chmod +x）をREADMEに追記するか要判断。
- DSN のパラメータ（例: parseTime=true, charset=utf8mb4 等）を推奨として記載するかは運用方針に依存。

**Labels:** feature, ci:ship

## API / DB Changes
- TODO

## Test Cases
- TODO
