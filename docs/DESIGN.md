# Design

## Title
READMEにfieldctlのMigration手順とapi-server起動手順を具体化する(gcfm)

## Summary

# READMEにfieldctlのMigration手順とapi-server起動手順を具体化する(gcfm)

## 背景
gcfmのREADME.mdにおける「fieldctlによるDBマイグレーション」と「api-serverの起動手順」の説明が不足しており、新規参加者が環境構築時に詰まっています。特に、MySQL向けのDSN書式、テーブルプレフィックス、seed投入の有無、起動後の確認方法が不明瞭です。

## 目的
README.mdを読めば、MySQLを用いた初期セットアップ(マイグレーションとAPIサーバー起動)がコピペで完了し、起動確認まで到達できる状態にする。

## 要件
- README.mdに以下のセクションを追加・更新する
  - 「Database migration (fieldctl)」
    - 前提条件: MySQL 8.xがlocalhost:3306で動作、root:rootpw、DB名gcfm(変更方法を記載)
    - DB作成手順(コマンド例を提示: mysql CLI / Docker)
    - マイグレーション実行コマンド(提示値をそのままコピー可能)
    - 各フラグの説明: --driver、--db(DSN)、--table-prefix、--schema、--seed
    - 成功判定(テーブル作成/seed投入の確認方法)
  - 「API serverの起動」
    - 起動コマンド(提示値をそのままコピー可能)
    - 起動後の確認方法(ヘルスチェック or ポート疎通。現状のエンドポイントはTODOで確認し、決まり次第確定表記)
  - 「設定のカスタマイズ」
    - DSNの一般形と、ユーザー・パスワード・ホスト・DB名の変更例
    - テーブルプレフィックスの変更例と注意点
    - seed有無の切替(本番では--seedを外す旨)
- コマンドはmacOS/Linux向けに検証したものを掲載。Windows PowerShell向けの差分があれば補足(なければ後日追記TODO明記)
- 既存のREADME章立てや目次(存在する場合)に新セクションのリンクを追加

## 非機能(性能/アクセシビリティ/可観測性など必要なもののみ)
- ドキュメントの再現性: コピペで動作することを最優先。前提条件と前後関係(「DB作成→migration→サーバー起動」)を明示
- 読みやすさ: 見出し/手順/コードブロックを整理し、重要パラメータは箇条書きで強調

## 影響範囲
- gcfm/README.md のみ(他コード・設定への変更は不要)。ただし、ビルド済みバイナリ(bin/fieldctl, bin/api-server)の入手/ビルド方法がREADMEに未記載であれば合わせて追記が必要(TODO)

## テスト観点
- 前提: ローカルにMySQL 8.xが動作、もしくはDockerで起動できる
- マイグレーション
  - DB未作成の状態から、DB作成→migration実行が成功する
  - gcfm_プレフィックスのテーブルが作成される
  - --seed指定時に初期データが投入される(行数/代表レコード確認例を提示)
- APIサーバー
  - 指定ポート(:18081)で起動する
  - DB接続エラー時のエラーメッセージ例と対処をREADMEに記載
  - 動作確認(HTTP 200の返却またはヘルスチェックエンドポイント)の手順が成功する(エンドポイント名はTODO確定)
- 代替設定
  - DSN/テーブルプレフィックス変更例を実行して動作する

## 想定コマンド
```bash
# (任意) DockerでMySQLを用意
docker run --name gcfm-mysql -e MYSQL_ROOT_PASSWORD=rootpw -e MYSQL_DATABASE=gcfm -p 3306:3306 -d mysql:8.0

# DB作成(ローカルMySQLの場合)
mysql -uroot -prootpw -e 'CREATE DATABASE IF NOT EXISTS gcfm CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'

# Migration (MySQL)
bin/fieldctl db migrate \
  --table-prefix="gcfm_" \
  --db "root:rootpw@tcp(localhost:3306)/gcfm" \
  --schema public \
  --seed \
  --driver=mysql

# APIサーバー起動
bin/api-server -addr=:18081 --driver=mysql --dsn "root:rootpw@tcp(localhost:3306)/gcfm"

# 動作確認(例: ヘルスチェックのエンドポイントは要確認 TODO)
# curl -f http://localhost:18081/healthz || curl -f http://localhost:18081/
```

## 受け入れ条件
- README.mdに「Database migration(fieldctl)」と「API serverの起動」セクションが追加され、提示コマンドがそのままコピー&ペーストで動く
- DSNの書式、--driver、--table-prefix、--schema、--seed各フラグの意味と変更方法がREADMEに明記されている
- MySQL未準備の読者向けにDB作成手順(ローカル/ Dockerいずれか)がREADMEに明記されている
- 初回migrationでgcfm_プレフィックスのテーブルが作成され、seedが投入されることの確認手順がREADMEに記載されている
- APIサーバー起動後の動作確認手順(ヘルスチェック or ポート確認)がREADMEに記載されている
- macOS/Linuxでの手順動作確認済みの注記が追記されている
- 既存の章立て/目次(あれば)に新セクションへのリンクが追加されている
- CI/ドキュメントリンタ(あれば)でエラー・警告が出ない

## 備考
TODO: 1) APIサーバーのヘルスチェック/疎通確認に使える正式なエンドポイント名(/healthz, /readyz, /ping, /など)を確認してREADMEに確定表記。2) bin/fieldctl, bin/api-server の入手方法(ローカルビルド or 配布バイナリ)がREADME未記載なら追記。3) --schemaフラグの扱い(MySQLでの意味/互換性)を確認し、必要なら注記。4) Windows(PowerShell)でのコマンド差分の有無を確認し、必要に応じて例を追記。

**Labels:** feature, ci:ship

## API / DB Changes
- TODO

## Test Cases
- TODO
