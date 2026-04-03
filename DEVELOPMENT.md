# maidirector - Development Note

メイド（maid）+ ディレクター（director）の合成語。
並行する施策開発のコンテキストスイッチ問題を解決するローカル完結の開発AIアシスタント。

## Status

- **現在のフェーズ**: Phase 2c'' 完了 → Phase 2c''' 着手待ち
- **直近の完了**: Phase 2c'（スヌーズ）+ Phase 2c''（リポジトリ非表示フィルタ + ツールバー整理）
- **次にやること**: Phase 2c'''（Suggest/Chat のレビュー待ち判定改善）→ Phase 2d（チャットからのタスク操作）

## Roadmap

### Phase 1: MVP ✅
- Go backend + React dashboard + Ollama + SQLite + Qdrant
- ストーリー CRUD +「次に何すべき？」提案 + チャット + セマンティック検索

### Phase 2: 外部連携 + 構造的依存認識
**完了条件**: PR URL を貼れば依存構造を認識し、「次にどのPRを見ればいいか」を自動提案できる

#### Phase 2a: ストーリー/レイヤーのデータモデル + CRUD ✅
- `stories` / `story_layers` テーブル + CRUD API + パイプラインビュー + Suggest/Chat プロンプト統合

#### Phase 2b: GitHub 連携（gh コマンド） ✅
- `internal/github` パッケージ（`gh pr view` ラッパー）
- PR URL → 情報自動取得・表示（isDraft, CI status, reviewDecision）、自動ブロッキング判定
- 一括リロードボタン（`POST /api/stories/refresh-prs`）、マージ済みレイヤーはリフレッシュスキップ

#### Phase 2.5: Task/Story データモデル統合 ✅
- Task と Story を統合。単タスク = レイヤー0個の Story として表現
- Task 系 API・コンポーネント全削除、Story に priority/project/links/tags 追加、UI 統合ビュー一本化

#### Phase 2c: 組織内 PR スキャン → レビュータスク自動生成 ✅
- `gh search prs --owner` で組織内オープン PR を取得（初回: 直近1週間、以降: 差分のみ）
- PR ごとに `[Review] リポジトリ名 #N: タイトル` という単タスク Story を自動作成（重複防止あり）
- `bot_policy`: `exclude`（デフォルト）または priority 数値で bot PR の扱いを制御
- 設定: `maidirector.yml` の `github.org`, `github.exclude_authors`, `github.bot_policy`
- ソート順を `priority ASC, created_at ASC` に変更（FIFO）

#### Phase 2c': スヌーズ機能 ✅
- `stories.snoozed_until DATETIME` カラム追加。スヌーズ中はリスト末尾に表示
- 期間選択: 30分 / 1時間 / 3時間 / 明日まで（翌日 09:00）/ 来週まで（次の月曜 09:00）
- API: `POST /api/stories/{id}/snooze`（body: `{ "duration": "30m"|"1h"|"3h"|"tomorrow"|"next_week" }`）、`POST /api/stories/{id}/unsnooze`
- UI: ⏰ ボタン → ドロップダウン選択、スヌーズ中は opacity 低下 + 残り時間バッジ
- 期限切れ時は cron 不要でクエリ時に自動復帰。クライアント側 setTimeout でスムーズに再取得

#### Phase 2c'': リポジトリ非表示フィルタ + ツールバー整理 ✅
- ヘッダーに「🔍 組織PRスキャン」「↻ 全PR情報を更新」ボタンを並置
- リストのツールバーをリポジトリ非表示チップに刷新
  - 単タスクの `links` から GitHub リポジトリ名を動的抽出してチップ生成（API 不要）
  - クリックで非表示トグル（複数同時可）。非表示チップは 🚫 + 打ち消し線 + 赤みで表示
  - 「すべて表示」ボタンで一括解除。レイヤーチェーン付きストーリーはフィルタ対象外
  - 状態はクライアントローカルステートのみ（`hiddenRepos: Set<string>`、DB 保存なし）

#### Phase 2c''': Suggest/Chat のレビュー待ち判定改善
- 他者レビュー待ちの PR を「次にやるべき」から除外する
- 除外条件: PR が review 状態 かつ 自分がアクション不要（未読コメントなし）
- 例外（提案に含める）: 未読コメントあり、または前日以降放置（再依頼アドバイス）
- 実装方針:
  - `context/manager.go` の `formatStories` / Suggest プロンプトでレビュー待ち状態を分類
  - PR の `reviewDecision` + 最終更新日時で「アクション可能かどうか」を判定

#### Phase 2d: チャットからのタスク操作
- Ollama tool use（function calling）サポート状況を検証
- 安定していれば: LLM tool use で Story CRUD を会話から実行
- フォールバック案: スラッシュコマンド方式（`/create タスク名`）をフロントエンド側でパース

#### Phase 2 後回し（必要に応じて）
- macOS デスクトップ通知 (osascript)
- ローリングサマリーの自動化

### Phase 3: 音声 + 高度な機能
- 音声入力 (Chrome SpeechRecognition API)、音声出力 (Web Speech Synthesis API)
- LLM アップグレード (OpenAI/Anthropic API 切替オプション)
- サブエージェントによるタスク状況の自動調査

### Phase 4: フル統合
- Slack / Miro / スプレッドシート連携
- 複数サービスを横断した自動タスク推定

## Architecture Decisions

- バックエンドに Go を採用。cmd/ + internal/ パターン、単一バイナリ配布。
- フロントエンドに React + Vite SPA を採用。Go embed.FS でバイナリに同梱。
- Chat LLM に Ollama (qwen2.5:14b) を採用。ローカル無料、日本語に強い。Provider インターフェース抽象化で差替可能。動作確認は qwen2.5:7b で実施。
- Embedding に Ollama (nomic-embed-text, 768次元) を採用。
- SQLite (modernc.org/sqlite, CGO不要) をタスク管理の正規ストアに採用。
- Qdrant (localhost:6333, REST API 直叩き) をセマンティック検索の記憶層に採用。コレクション名は `maidirector`。
- GitHub 連携に gh CLI を採用。OAuth トークン管理不要、プライベートリポジトリにアクセス可能。PR 状態取得はイベント駆動（Suggest 押下時・PR URL 貼付時・チャット質問時・手動更新時）。

## Tech Stack

| 要素 | 選定 |
|------|------|
| Backend | Go + Cobra CLI + net/http (go 1.26) |
| Frontend | React 19 + Vite + TypeScript |
| Chat LLM | Ollama / qwen2.5:14b (7b で検証済み) |
| Embedding | Ollama / nomic-embed-text (768次元) |
| DB | SQLite (modernc.org/sqlite) |
| Vector | Qdrant v1.17.0 (既存 Docker, REST API) |
| GitHub 連携 | gh CLI (os/exec 経由) |
| Config | YAML (gopkg.in/yaml.v3) |

## Data Model

```
stories テーブル
  - id, title, description, layer_chain, status (active/completed/archived)
  - priority (0=なし, 1=P1最高 〜 5=P5最低), project, links (JSON), tags (JSON)
  - snoozed_until DATETIME (NULL=スヌーズなし)
  - last_active_at, created_at, updated_at
  - ソート: CASE WHEN snoozed_until > CURRENT_TIMESTAMP THEN 1 ELSE 0 END ASC, priority ASC, created_at ASC

story_layers テーブル
  - id, story_id, layer_name, status (pending/in_progress/review/merged)
  - pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft (INTEGER), pr_check_status
  - sort_order (ブロック判定に使用。直前レイヤーが merged でなければブロック中)
  - created_at, updated_at
  - FOREIGN KEY (story_id) → stories(id) ON DELETE CASCADE

settings テーブル
  - key (PRIMARY KEY), value, updated_at
  - 現在の利用: last_org_pr_scan_time（差分スキャン用の前回実行時刻）

context_entries テーブル
  - id, story_id, content, entry_type (note/progress/blocker), created_at

conversations / summaries テーブル (LLM チャット履歴・サマリー)
```

### レイヤー依存グラフ（設計イメージ）

```
ストーリーA ─┬─ proto (PR #101)          ← マージ済み
             ├─ microservice (PR #102)   ← レビュー中 ← 次にやるべき
             ├─ gateway (PR #103)        ← ブロック中（microservice待ち）
             └─ frontend (PR #104)       ← ブロック中（gateway待ち）
```

`layer_chains` は `maidirector.yml` で定義。ブロック判定は `sort_order` を使い表示時に計算。

### GitHub PR 取得

```bash
gh pr view <url> --json number,title,state,isDraft,mergeable,reviewDecision,url,statusCheckRollup
```

イベント駆動: Suggest 押下時 / PR URL 貼付時 / Chat 質問時 / 「↻ 全PR情報を更新」押下時

### Suggest/Chat プロンプト（優先度判断基準）

ブロッカー解消 > 上流マージ済み下流未着手 > PR APPROVED未マージ > CHANGES_REQUESTED修正 >
CI FAILURE修正 > CI PENDING待ち > 進行中完了 > 高優先度 > 長時間放置

## Known Issues / TODO

- Phase 2c''': Suggest/Chat でレビュー待ち PR を「次にやるべき」から除外する判定が未実装
- Phase 2d: チャットからのタスク操作（Ollama tool use）が未実装
- 全レイヤー完了時にストーリーを自動完了扱いにする機能が未実装
- Chrome SpeechRecognition API のブラウザ制限（Phase 3 以降の課題）

## Session Log

### 2026-04-02: Phase 1 MVP
- 要件定義・技術選定・アーキテクチャ設計・ロードマップ策定
- Go backend + React frontend フルスタック実装、embed.FS で単一バイナリ化 (16MB)
- 動作確認: qwen2.5:7b でタスクCRUD・提案・チャット・ベクトル化・削除すべて疎通

### 2026-04-02: Phase 2a – ストーリー/レイヤー + パイプラインビュー
- stories/story_layers CRUD + パイプラインビュー + Suggest/Chat プロンプト統合
- 動作確認: API + ブラウザ UI（パイプライン表示、ブロック判定、WebSocket リアルタイム更新）

### 2026-04-02: Phase 2b/2b' – GitHub 連携
- `internal/github` パッケージ（`gh pr view` ラッパー）、PR URL → 情報自動取得・表示
- isDraft / CI ステータス / PR ラベル GitHub 準拠化、一括リフレッシュ
- 動作確認: gh 認証、PR 情報取得・保存、ブラウザ UI 表示

### 2026-04-02: 開発環境スクリプト + AGENTS.md
- `dev.sh`（全依存サービスべき等一括起動）、`AGENTS.md`（動作確認の判断基準を明文化）

### 2026-04-02: Phase 2.5 – Task/Story データモデル統合
- Task と Story を統合。Task 系 API・コンポーネント全削除、Story に priority/project/links/tags 追加
- 動作確認: Story CRUD + Context + フィルタ、統合 UI での単タスク・ステータス変更・削除
- バグ修正: links が null 返却 → CreateStory で nil → `[]` デフォルト

### 2026-04-02: Phase 2c – 組織内PRスキャン → レビュータスク自動生成
- `SearchOrgPRs`、`FindStoryByPRURL`（重複防止）、`settings` テーブル（差分スキャン用）
- `POST /api/stories/scan-org-prs`、`bot_policy` 設定、`maidirector.yml` に `github` セクション追加
- 動作確認: 54件スキャン → 41件作成・13件 bot スキップ、再スキャン時の重複防止・差分スキャン

### 2026-04-03: Phase 2c' – スヌーズ機能
- `stories.snoozed_until` カラム追加（migration）、ソート変更
- `SnoozeStory` / `UnsnoozeStory` ストアメソッド追加（`internal/store/store.go`）
- `POST /api/stories/{id}/snooze` / `POST /api/stories/{id}/unsnooze` エンドポイント追加
- フロントエンド: ⏰ ボタン + ドロップダウン + 残り時間バッジ + クライアント側 setTimeout
- 動作確認: スヌーズ適用 → リスト最下部移動 + 「29分後に復帰」バッジ表示

### 2026-04-03: Phase 2c'' – リポジトリ非表示フィルタ + ツールバー整理
- ヘッダーに「🔍 組織PRスキャン」「↻ 全PR情報を更新」を並置（`App.tsx` の `header-actions`）
- `StoryPipeline.tsx` のツールバーをリポジトリ非表示チップに変更
  - `hiddenRepos: Set<string>` で複数リポジトリを同時非表示可能
  - 非表示チップ: 🚫 + 打ち消し線 + 赤みのスタイル、「すべて表示」ボタンで一括解除
- 動作確認: 23リポジトリのチップ表示、複数リポジトリの同時非表示・解除
