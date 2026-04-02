# maidirector - Development Note

メイド（maid）+ ディレクター（director）の合成語。
並行する施策開発のコンテキストスイッチ問題を解決するローカル完結の開発AIアシスタント。

## Status

- **現在のフェーズ**: Phase 2c 実装完了 → Phase 2c'（スヌーズ機能）着手待ち
- **直近の完了**: Phase 2c（組織内PRスキャン → レビュータスク自動生成）
- **次にやること**: Phase 2c'（スヌーズ機能）→ Phase 2d（チャットからのタスク操作）

## Roadmap

### Phase 1: MVP ✅
- Go backend + React dashboard + Ollama + SQLite + Qdrant
- タスク CRUD +「次に何すべき？」提案 + チャット + セマンティック検索

### Phase 2: 外部連携 + 構造的依存認識
- **完了条件**: PR URL を貼れば依存構造を認識し、「次にどのPRを見ればいいか」を自動提案できる

#### Phase 2a: ストーリー/レイヤーのデータモデル + CRUD ✅
- `stories` / `story_layers` テーブル + CRUD API + パイプラインビュー + Suggest/Chat プロンプト統合

#### Phase 2b: GitHub 連携（gh コマンド） ✅
- `internal/github` パッケージ（`gh pr view` ラッパー）
- PR URL → 情報自動取得・表示、Suggest/Chat 前に一括リフレッシュ、自動ブロッキング判定

#### Phase 2b': GitHub 連携強化 ✅
- 一括リロードボタン（`POST /api/stories/refresh-prs`）
- `isDraft`, `statusCheckRollup` 取得 → CI ステータス表示（✓/✗/⏳）
- PR ラベルを GitHub 準拠（マージ済み:紫 / オープン:緑 / ドラフト:灰 / 未着手:灰）
- ブロック表現はラベルに影響せず視覚のみ（opacity + dashed border）
- マージ済みレイヤーのリフレッシュスキップ最適化

#### Phase 2.5: Task/Story データモデル統合 ✅
- Task と Story を統合し、単一のデータモデル（Story）で単タスクも複数レイヤー施策も表現する
- UI のタスク/ストーリータブを廃止し、統合ビューに一本化

#### Phase 2c: 組織内 PR スキャン → レビュータスク自動生成 ✅
- `gh search prs --owner <組織名> --state open --sort created --order asc` で組織内のオープン PR を取得（古い順）
- 初回は直近1週間分を取得、以降は前回実行時刻からの差分のみ（`--updated >=YYYY-MM-DD` で絞り込み、当日分も含む）
- bot PR の扱いは `bot_policy` で設定: `exclude`（デフォルト: スキップ）または priority 数値（1=最高 〜 5=最低）で取り込み
- 取得した PR ごとに「レビュー対応」Story（レイヤー0個 = 単タスク）を自動作成
  - 既に同じ PR URL が紐づいた Story（links JSON）または StoryLayer（pr_url）が存在する場合はスキップ（重複防止）
  - タイトル: `[Review] リポジトリ名 #123: PRタイトル`、tags: `["review"]`、links: `[PR URL]`
- 前回実行時刻は SQLite `settings` テーブルに保存
- UI: 手動実行ボタン（「🔍 組織PRスキャン」）+ 結果バナー通知
- ストーリーのソート順を `priority ASC, created_at ASC` に変更（同一priority内で古いものが先 = FIFO）
- 設定: `maidirector.yml` の `github.org`, `github.exclude_authors`, `github.bot_policy`

#### Phase 2c': スヌーズ機能
- ストーリーカードを一定時間「スヌーズ」し、priority に関係なくリストの一番下に表示する
- `stories` テーブルに `snoozed_until DATETIME` カラム追加
- ソート順: `CASE WHEN snoozed_until > CURRENT_TIMESTAMP THEN 1 ELSE 0 END ASC, priority ASC, created_at ASC`
- スヌーズ期間の選択肢:
  - 30分 / 1時間 / 3時間 / 明日まで（翌日 09:00）/ 来週まで（次の月曜 09:00）
- API:
  - `POST /api/stories/{id}/snooze` — body: `{ "duration": "30m" | "1h" | "3h" | "tomorrow" | "next_week" }`
  - `POST /api/stories/{id}/unsnooze` — スヌーズ解除
- UI:
  - カードに ⏰ ボタン → クリックで期間選択ドロップダウン
  - スヌーズ中のカードは視覚的に区別（opacity 低下 + 残り時間バッジ表示）
  - スヌーズ解除ボタン
- スヌーズ期限切れ時はソート位置が自動的に元に戻る（クエリ時に判定するだけで cron 不要）

#### Phase 2d: チャットからのタスク操作
- Ollama の tool use（function calling）サポート状況を検証
- サポートが安定していれば: LLM tool use で Story CRUD を会話から実行（統合後は操作対象が1系統）
- フォールバック案: スラッシュコマンド方式（`/create タスク名`）をフロントエンド側でパース

#### Phase 2 後回し（必要に応じて）
- macOS デスクトップ通知 (osascript)
- ローリングサマリーの自動化

### Phase 3: 音声 + 高度な機能
- 音声入力 (Chrome SpeechRecognition API)
- 音声出力 (Web Speech Synthesis API)
- LLM アップグレード (OpenAI/Anthropic API 切替オプション)
- サブエージェントによるタスク状況の自動調査

### Phase 4: フル統合
- Slack / Miro / スプレッドシート連携
- 複数サービスを横断した自動タスク推定

## Architecture Decisions

- 2026-04-02: バックエンドに Go を採用。cmd/ + internal/ パターン、単一バイナリ配布の利点。
- 2026-04-02: フロントエンドに React + Vite SPA を採用。将来の音声統合に有利、Go embed.FS でバイナリに同梱。
- 2026-04-02: Chat LLM に Ollama (qwen2.5:14b) を採用。ローカル無料、日本語に強い14Bモデル。Provider インターフェース抽象化で後から差替可能。動作確認は qwen2.5:7b で実施済み。
- 2026-04-02: Embedding に Ollama (nomic-embed-text, 768次元) を採用。Ollama の /api/embed エンドポイントで統一。
- 2026-04-02: タスク管理の正規ストアに SQLite (modernc.org/sqlite, CGO不要) を採用。
- 2026-04-02: セマンティック検索用の記憶層に既存 Qdrant Docker インスタンス (localhost:6333) を流用。コレクション名は `maidirector`。Qdrant Go client (gRPC依存) ではなく REST API を直接叩く実装を採用。
- 2026-04-02: プロジェクト名を maidirector に決定。メイドラゴンリスペクト。
- 2026-04-02: GitHub 連携に gh CLI コマンドを採用。OAuth トークン管理不要、プライベートリポジトリにアクセス可能。PR 状態取得はポーリングではなくイベント駆動（Suggest 押下時・PR URL 貼付時・チャット質問時に都度取得）。

## Tech Stack

| 要素 | 選定 | 役割 |
|------|------|------|
| Backend | Go + Cobra CLI + net/http | API サーバー、単一バイナリ (go 1.26) |
| Frontend | React + Vite + TypeScript | ダッシュボード SPA |
| Chat LLM | Ollama / qwen2.5:14b (7b で検証済み) | 提案・チャットの推論 |
| Embedding | Ollama / nomic-embed-text | テキスト → 768次元ベクトル変換 |
| DB | SQLite (modernc.org/sqlite) | タスク・ストーリー・会話・サマリーの CRUD (正規ストア) |
| セマンティック検索 | Qdrant v1.17.0 (既存Docker) | 過去の文脈をベクトル検索で引き出す記憶層 |
| GitHub 連携 | gh CLI (os/exec経由) | PR状態取得・組織PRスキャン |
| Config | YAML (gopkg.in/yaml.v3) | 設定ファイル |

## Feature Design: レイヤー依存グラフ

### 課題

施策（ストーリー）の開発は proto → microservice/gateway → backend → frontend のようにレイヤーごとに構造的なブロッキング関係がある。これが複数のストーリーで並行して進行するため、「今どのストーリーのどのレイヤーまで進んでいるか」「次にどのPRを見ればいいか」の把握が困難になる。

### 設計の方向性

```
ストーリーA ─┬─ proto (PR #101)          ← マージ済み
             ├─ microservice (PR #102)   ← レビュー中 ← 次にやるべき
             ├─ gateway (PR #103)        ← ブロック中（microservice待ち）
             └─ frontend (PR #104)       ← ブロック中（gateway待ち）

ストーリーB ─┬─ proto (PR #201)          ← レビュー中 ← 次にやるべき
             ├─ microservice (PR #202)   ← 未着手
             └─ frontend (PR #203)       ← 未着手
```

レイヤーの依存順序は設定ファイルで定義:
```yaml
layer_chains:
  - name: default
    layers: [proto, microservice, gateway, frontend]
  - name: simple
    layers: [backend, frontend]
```

### データモデル（実装済み）

```
stories テーブル
  - id, title, description, layer_chain, status (active/completed/archived)
  - priority (0=なし, 1=P1最高 〜 5=P5最低), project, links (JSON), tags (JSON)
  - last_active_at, created_at, updated_at
  - ソート順: priority ASC, created_at ASC

story_layers テーブル
  - id, story_id, layer_name, status (pending/in_progress/review/merged)
  - pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft (INTEGER), pr_check_status
  - sort_order (レイヤーチェーン内の順序、ブロック判定に使用)
  - created_at, updated_at
  - FOREIGN KEY (story_id) → stories(id) ON DELETE CASCADE

settings テーブル
  - key (PRIMARY KEY), value, updated_at
```

ブロック判定は `sort_order` を使い表示時に計算。直前のレイヤーが merged でなければブロック中と判定する。

### GitHub PR 取得タイミング（イベント駆動）

- 「提案してもらう」ボタン押下時 → Suggest 前に全紐付き PR をリフレッシュ（マージ済みスキップ）
- PR URL をストーリーレイヤーに貼り付けた時 → その場で取得・表示
- チャット質問時 → 回答前にリフレッシュ
- 「↻ 全PR情報を更新」ボタン押下時 → 一括リフレッシュ

```bash
gh pr view <url> --json number,title,state,isDraft,mergeable,reviewDecision,url,statusCheckRollup
```

### Suggest/Chat プロンプト統合（実装済み）

- アクティブなストーリーのパイプライン状況（ステータス + ブロック判定 + PR state/isDraft/reviewDecision/CI status）をプロンプトに注入
- 判断基準: ブロッカー解消 > 上流マージ済み下流未着手 > PR APPROVED未マージ > CHANGES_REQUESTED修正 > CI FAILURE修正 > CI PENDING待ち > 進行中完了 > 高優先度 > 長時間放置

## Known Issues / TODO

### Phase 2d で対応予定
- チャットからのタスク操作（Ollama tool use）

### 将来の検討事項
- Chrome SpeechRecognition API のブラウザ制限 (Phase 3)
- 全レイヤー完了時にストーリーを自動完了扱い

## Session Log

### 2026-04-02: Phase 1 MVP 実装
- 要件定義・技術選定・アーキテクチャ設計・ロードマップ策定
- Go backend + React frontend フルスタック実装、embed.FS で単一バイナリ化 (16MB)
- 動作確認完了（qwen2.5:7b でタスクCRUD・提案・チャット・ベクトル化・削除すべて疎通）

### 2026-04-02: Phase 2a - ストーリー/レイヤー + パイプラインビュー
- stories/story_layers CRUD + パイプラインビュー + Suggest/Chat プロンプト統合
- 動作確認完了（API + ブラウザ UI: パイプライン表示、ブロック判定、WebSocket リアルタイム更新）

### 2026-04-02: Phase 2b/2b' - GitHub 連携
- `internal/github` パッケージ（`gh pr view` ラッパー）、PR URL → 情報自動取得・表示
- isDraft / statusCheckRollup / CI ステータス表示、PR ラベル GitHub 準拠化、一括リフレッシュ
- 動作確認完了（gh 認証、PR 情報取得・保存、ブラウザ UI 表示）

### 2026-04-02: 開発環境スクリプト + AGENTS.md
- `dev.sh`（全依存サービスべき等一括起動）、`AGENTS.md`（動作確認の判断基準を明文化）

### 2026-04-02: Phase 2.5 - Task/Story データモデル統合
- Task と Story を統合。単タスク = レイヤー0個の Story として表現
- Task 系 API・コンポーネント全削除、Story に priority/project/links/tags 追加、UI 統合ビュー一本化
- 動作確認完了（API: Story CRUD + Context + フィルタ、ブラウザ UI: 統合ビュー・単タスク・ステータス変更・削除）
- バグ修正: links が null 返却 → CreateStory で nil → `[]` デフォルト

### 2026-04-02: Phase 2c - 組織内PRスキャン → レビュータスク自動生成
- `SearchOrgPRs`（`gh search prs --owner`）、`FindStoryByPRURL`（重複防止）、`settings` テーブル（差分スキャン）
- `POST /api/stories/scan-org-prs` エンドポイント、UI に「🔍 組織PRスキャン」ボタン
- ソート順を `priority ASC, created_at ASC` に変更（FIFO）
- `bot_policy` 設定: exclude（デフォルト）または priority 数値で bot PR の扱いを切替
- `maidirector.yml` に `github` セクション追加（`org`, `exclude_authors`, `bot_policy`）
- 動作確認完了（54件スキャン → 41件作成・13件bot スキップ、再スキャン時の重複防止・差分スキャン、ブラウザ UI 表示）
