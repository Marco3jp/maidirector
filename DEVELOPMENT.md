# maidirector - Development Note

メイド（maid）+ ディレクター（director）の合成語。
並行する施策開発のコンテキストスイッチ問題を解決するローカル完結の開発 AI アシスタント。

## Status

- **現在のフェーズ**: Phase 3a 完了 + UI 改善継続中
- **次にやること**: Phase 2c'''（Suggest/Chat のレビュー待ち PR を除外する判定）→ Phase 2d（チャットからのタスク操作）

## Roadmap

### Phase 1: MVP ✅
Go backend + React dashboard + Ollama + SQLite + Qdrant。ストーリー CRUD・提案・チャット・セマンティック検索。

### Phase 2: 外部連携 + 構造的依存認識

- **Phase 2a** ✅ stories/story_layers CRUD + パイプラインビュー + Suggest/Chat 統合
- **Phase 2b** ✅ GitHub 連携（`gh pr view` ラッパー）。PR 情報自動取得・表示、一括リロード
- **Phase 2.5** ✅ Task/Story 統合（単タスク = レイヤー 0 個の Story）
- **Phase 2c** ✅ 組織内 PR スキャン → レビュータスク自動生成（`bot_policy` 設定あり）
- **Phase 2c'** ✅ スヌーズ機能（30m/1h/3h/明日/来週、ソート最下部、残り時間バッジ）
- **Phase 2c''** ✅ リポジトリ非表示フィルタ（localStorage 永続化）
- **Phase 2c'''** 部分対応: `formatStories` で単タスクのリンク・依存情報を LLM に渡すよう修正、プロンプトの上流/下流判断基準を明確化。レビュー待ち PR の完全な除外判定は未実装
- **Phase 2d** 未実装: チャットからのタスク操作（Ollama tool use または `/` コマンド）

### Phase 3: データモデル進化

- **Phase 3a** ✅ Task Dependency DAG（`story_layers` → `story_tasks` + `task_dependencies`）
- **Phase 3b** 予定: 音声入力/出力、LLM アップグレード（OpenAI/Anthropic 切替）

### Phase 4: フル統合（将来）
Slack / Miro / スプレッドシート連携、複数サービス横断タスク推定。

## Architecture

- **Backend**: Go (cmd/ + internal/ パターン、単一バイナリ)
- **Frontend**: React 19 + Vite SPA（Go embed.FS で同梱）
- **Chat LLM**: Ollama / qwen2.5:14b（Provider インターフェース抽象化で差替可能）
- **Embedding**: Ollama / nomic-embed-text（768次元）
- **DB**: SQLite（modernc.org/sqlite、CGO 不要）
- **Vector**: Qdrant（localhost:6333、REST API）コレクション名 `maidirector`
- **GitHub 連携**: gh CLI（OAuth 不要、プライベートリポジトリ対応）

## Data Model

```
stories
  id, title, description, layer_chain, status (active/completed/archived)
  priority (0=なし, 1=P1最高 〜 5=P5最低)  ※手動追加デフォルト P3、PRスキャンデフォルト P4
  project, links (JSON), tags (JSON)
  snoozed_until DATETIME (UTC テキスト形式 "YYYY-MM-DD HH:MM:SS" または T+Z)
  last_active_at, created_at, updated_at

  ソート:
    CASE WHEN datetime(snoozed_until) > datetime('now') THEN 1 ELSE 0 END ASC
    CASE WHEN priority = 0 THEN 999 ELSE priority END ASC
    created_at ASC

story_tasks  （Phase 3a で story_layers からリネーム移行）
  id, story_id, name, status (pending/in_progress/review/merged)
  pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status
  sort_order（表示順のみ。ブロッキング判定には使用しない）
  created_at, updated_at
  FOREIGN KEY story_id → stories ON DELETE CASCADE

task_dependencies  （Phase 3a 追加）
  task_id, depends_on_id  PRIMARY KEY (task_id, depends_on_id)  ON DELETE CASCADE
  ブロッキング判定: depends_on_id のステータスが merged でなければブロック中

story_layers  （互換性維持のため残存。story_tasks への移行確認後に DROP TABLE 予定）

settings
  key (PK), value, updated_at
  利用: last_org_pr_scan_time（差分スキャン用）

context_entries
  id, story_id, content, entry_type (note/progress/blocker), created_at

conversations / summaries  （LLM チャット履歴・サマリー）
```

## Known Issues / TODO

- Phase 2c''': Suggest/Chat でレビュー待ち PR の完全な除外判定が未実装
- Phase 2d: チャットからのタスク操作が未実装
- `story_layers` テーブルが残存中 → `story_tasks` 移行確認後に `DROP TABLE story_layers` を追加すること
- 全タスク完了時にストーリーを自動完了扱いにする機能が未実装

## Session Log

### 2026-04-02: Phase 1〜2.5
Go backend + React frontend フルスタック実装、embed.FS で単一バイナリ化（16MB）。stories/story_layers CRUD、パイプラインビュー、GitHub 連携（gh CLI）、Task/Story 統合。

### 2026-04-02: Phase 2c – 組織内PRスキャン
`SearchOrgPRs`、`FindStoryByPRURL`（重複防止）、`settings` テーブル（差分スキャン）、`bot_policy` 設定。

### 2026-04-03: Phase 2c'〜2c'' – スヌーズ + リポジトリフィルタ
スヌーズ: `stories.snoozed_until` 追加、SnoozeStory/UnsnoozeStory API、⏰ ドロップダウン UI。  
**バグ**: `modernc.org/sqlite` が `time.Time` を `+0900 JST` 形式で保存するため SQLite の `datetime()` が NULL を返す問題 → UTC テキスト形式に変換して保存し解消。  
リポジトリフィルタ: `hiddenRepos: Set<string>` で複数リポジトリを同時非表示、localStorage 永続化（`maidirector:hiddenRepos`）。

### 2026-04-03: Phase 3a – Task Dependency DAG
`story_tasks` テーブル（`story_layers` から `layer_name → name` でリネーム移行）、`task_dependencies` ジャンクションテーブル追加。ブロッキング判定を `DependsOn` ベースに変更。レイヤーチェーンテンプレート選択時にタスク + deps を自動生成。`TaskManager` コンポーネントで deps 追加・解除 UI。

### 2026-04-03: UI 改善各種
優先度変更 UI、ストーリー説明文インライン編集・URL 自動リンク化、一括/個別 collapse、ストーリー追加フォーム改善。ヘッダー削除、PRスキャン/全PR更新ボタンをフィルタバーへ移動（セパレーター区切り）。

### 2026-04-03: Suggest 誤判定修正（Phase 2c''' 部分対応）
単タスクストーリー（`Tasks` が空）の `Links` が LLM プロンプトに含まれていなかった問題と、`DependsOn` が空のタスクに「独立」が明示されていなかった問題を修正。`formatStories` で単タスクのリンクを出力、タスクに `[依存なし・独立タスク]` / `[上流: タスク名(ステータス)]` を明示。システムプロンプトの判断基準 2 に「depends_on で定義された同一ストーリー内の関係のみ」を明記。

### 2026-04-03: PRスキャン由来レビューストーリーの自動完了
`refreshPRStates()` を拡張し、タスクなし・`links[0]` に GitHub PR URL が含まれるストーリー（PRスキャン由来の `[Review]` ストーリー）を対象に PR 状態を確認、MERGED/CLOSED であればストーリーを `completed` に自動更新するロジック（`checkAndCompleteReviewStory`）を追加。`handleScanOrgPRs` の末尾でも `refreshPRStates()` を呼ぶことで、スキャン実行時・Suggest/Chat 時・全PR更新ボタン押下時の 3 タイミングで自動反映される。コンパイル確認済み（`go build ./...` 成功）。
