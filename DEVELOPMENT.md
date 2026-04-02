# maidirector - Development Note

メイド（maid）+ ディレクター（director）の合成語。
並行する施策開発のコンテキストスイッチ問題を解決するローカル完結の開発AIアシスタント。

## Status

- **現在のフェーズ**: Phase 2.5 実装完了 → Phase 2c（組織内PRスキャン）着手待ち
- **直近の完了**: Phase 2.5（Task/Story データモデル統合 - Task を Story に統合、UI 一本化、プロンプト統合）
- **次にやること**: Phase 2c（組織内PRスキャン）→ Phase 2d（チャットからのタスク操作）

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

#### Phase 2c: 組織内 PR スキャン → レビュータスク自動生成
- `gh search prs --org <組織名> --state open --sort updated` で組織内のオープン PR を取得
- 初回は直近1週間分を取得、以降は前回実行時刻からの差分のみ（`--updated >YYYY-MM-DD` で絞り込み）
- bot ユーザーの PR はスキップ（author が `[bot]` suffix または設定ファイルで除外リスト定義）
- 取得した PR ごとに「レビュー対応」Story（レイヤー0個 = 単タスク）を自動作成
  - 既に同じ PR URL が紐づいた Story が存在する場合はスキップ（重複防止）
  - タイトル例: `[Review] リポジトリ名 #123: PRタイトル`
- 前回実行時刻は SQLite に保存（`key_value` テーブルまたは `settings` テーブル）
- UI: 手動実行ボタン（「組織PRスキャン」）+ 結果通知
- Phase 2.5（Task/Story 統合）完了後に着手が前提

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
| GitHub 連携 | gh CLI (os/exec経由) | PR状態取得、イベント駆動 (Phase 2) |
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
  - created_at, updated_at

story_layers テーブル
  - id, story_id, layer_name, status (pending/in_progress/review/merged)
  - pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft (INTEGER), pr_check_status
  - sort_order (レイヤーチェーン内の順序、ブロック判定に使用)
  - created_at, updated_at
  - FOREIGN KEY (story_id) → stories(id) ON DELETE CASCADE
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

### Phase 2c で対応予定
- 組織内 PR スキャン → レビュータスク自動生成（Phase 2.5 完了後に着手）

### Phase 2d で対応予定
- チャットからのタスク操作（Ollama tool use）

### 将来の検討事項
- Chrome SpeechRecognition API のブラウザ制限 (Phase 3)
- 全レイヤー完了時にストーリーを自動完了扱い

## Session Log

### 2026-04-02: Phase 1 MVP 実装
- 要件定義・技術選定・アーキテクチャ設計・ロードマップ策定
- Phase 1 MVP フルスタック実装完了:
  - Go backend: config, store(SQLite), vectorstore(Qdrant), llm(Ollama), task, context, api, ws
  - React frontend: TaskList, TaskForm, SuggestPanel, ChatPanel, WebSocket hook
  - embed.FS で単一バイナリ化 (16MB)、Makefile 統合
- Phase 1 残作業の実装:
  - タスク作成/更新時のベクトル化（Qdrant upsert、SourceID でべき等更新）
  - context_entry のベクトル化、UI 楽観的更新、favicon、タスク削除時の Qdrant クリーンアップ
- 動作確認完了（Ollama qwen2.5:7b でタスクCRUD・提案・チャット・ベクトル化・削除すべて疎通）

### 2026-04-02: Phase 2a - ストーリー/レイヤー + パイプラインビュー
- `stories` / `story_layers` テーブル、Story/StoryLayer CRUD、`StoryService` を実装
- API: `GET/POST /api/stories`, `GET/PUT/DELETE /api/stories/{id}`, Layer CRUD, `GET /api/layer-chains`
- フロントエンド: `StoryForm`, `StoryPipeline`（パイプラインビュー）, タスク/ストーリー切替タブ
- Suggest/Chat プロンプトにストーリー情報を統合（`formatStories` でブロック判定含む）
- 動作確認完了（API + ブラウザ UI: パイプライン表示、ブロック判定、WebSocket リアルタイム更新）
- バグ修正: `LayerChainConfig` の JSON タグ欠如

### 2026-04-02: Phase 2b - GitHub 連携（gh コマンド）
- `internal/github` パッケージ: `GetPRInfo`, `IsAvailable`, `ParsePRURL`
- データモデル拡張: `PRTitle`, `PRReviewDecision` を StoryLayer に追加
- API: `applyPRInfo`（PR URL 設定時に自動取得）, `refreshPRStates`（Suggest/Chat 前に一括更新）, `POST .../refresh-pr`（個別リフレッシュ）
- フロントエンド: `PipelineStage` サブコンポーネント分離、PR URL 入力フォーム、PR 情報表示（番号リンク・タイトル・state バッジ・reviewDecision）、↻ ボタン
- 動作確認完了（gh 認証確認、PR 情報取得・保存、ブラウザ UI 表示）

### 2026-04-02: 開発環境スクリプト + AGENTS.md
- `dev.sh` 追加: 全依存サービス（Qdrant, Ollama, バックエンド, フロントエンド）のべき等一括起動
- `AGENTS.md` 追加: ブラウザ UI テストの判断基準を明文化

### 2026-04-02: Phase 2b' - GitHub 連携強化
- `gh pr view` に `isDraft`, `statusCheckRollup` を追加、`summarizeChecks()` で CI ステータスを集約
- データモデル拡張: `PRIsDraft`, `PRCheckStatus` を StoryLayer に追加、`scanStoryLayer()` ヘルパーで bool ↔ INTEGER 共通化
- API: `POST /api/stories/refresh-prs`（全ストーリー一括更新）、マージ済みレイヤーのリフレッシュスキップ
- フロントエンド: PR ラベル GitHub 準拠化（マージ済み:紫/オープン:緑/ドラフト:灰/未着手:灰）、CI 表示（✓/✗/⏳）、「↻ 全PR情報を更新」ボタン
- LLM プロンプト: draft/ci 情報追加、CI FAILURE/PENDING の判断基準追加
- コンパイル確認: Go build + TypeScript type check エラーなし

### 2026-04-02: Phase 2.5 - Task/Story データモデル統合
- Task と Story を統合。単タスク = レイヤー0個の Story として表現
- バックエンド:
  - `model.go`: Task 型・ステータス定数を削除、Story に priority/project/links/tags/last_active_at を追加、ContextEntry の task_id → story_id 変更
  - `store.go`: stories テーブルにカラム追加マイグレーション、tasks → stories 自動データ移行（起動時）、context_entries.task_id → story_id リネーム、tasks テーブル DROP、Story CRUD を新カラム対応に更新、scanStoryRow ヘルパー追加、CreateStory で links/tags nil → 空配列デフォルト
  - `service.go`: TaskStore/Service を削除、StoryStore に CreateContextEntry/ListContextEntries を追加
  - `server.go`: Task 系 API エンドポイント・ハンドラを全削除、Story 系に context エンドポイント追加（POST/GET /api/stories/{id}/context）、ベクトル化を story ベースに変更（source_type: "story"）、deleteStoryVectors でコンテキストベクトルも削除
  - `vectorstore.go`: PointPayload の TaskID → StoryID 変更
  - `context/manager.go`: ConversationStore から ListTasks 削除、filterActiveTasks/formatTasks/buildRecentActivityQuery を Story ベースに統合、buildSuggestPrompt/buildChatPrompt を Story のみに統一、formatStories に priority/project/tags 情報追加
  - `main.go`: taskService を削除、NewServer シグネチャ更新
  - `dev.sh`: ヘルスチェック URL を `/api/tasks` → `/api/stories` に修正
- フロントエンド:
  - `types.ts`: Task 型を削除、Story に priority/project/links/tags/last_active_at 追加、ContextEntry を story_id に変更
  - `client.ts`: Task 系メソッドを全削除、addContext/listContext を stories/{id}/context に変更
  - `App.tsx`: タスク/ストーリータブ切替を廃止、統合ビューに一本化（active/completed/archived フィルタ）
  - `StoryPipeline.tsx`: 単タスク（レイヤー0個）表示対応、ステータス変更セレクト、コンテキスト入力統合、priority/project/tags 表示
  - `StoryForm.tsx`: priority/project/tags 入力追加、レイヤーチェーン未選択=単タスク扱い
  - `TaskList.tsx`, `TaskForm.tsx`: 削除
  - `App.css`: nav-tabs スタイル削除、story-status-badge/story-footer/story-status-select 追加
- 動作確認:
  - コンパイル確認: Go build + TypeScript type check エラーなし
  - API 確認: Story CRUD（作成・取得・更新・削除）、Context 追加・一覧、ステータスフィルタ → 全 OK。links が null 返却されるバグ発見 → CreateStory で nil → `[]` デフォルト設定して修正
  - ブラウザ UI 確認: 統合ビュー（タブなし）、既存レイヤーストーリー表示（パイプライン・PR情報・CI状態）、単タスク作成・表示、ステータス変更（active→completed、完了セクション折りたたみ移動）、削除 → 全 OK
- AGENTS.md: 作業完了の定義を明文化（検証ステップ → DEVELOPMENT.md 更新の順序を厳格化）
