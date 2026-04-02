# maidirector - Development Note

メイド（maid）+ ディレクター（director）の合成語。
並行する施策開発のコンテキストスイッチ問題を解決するローカル完結の開発AIアシスタント。

## Status

- **現在のフェーズ**: Phase 2b' 実装完了 → Phase 2.5（Task/Story 統合）着手待ち
- **直近の完了**: Phase 2b'（GitHub 連携強化 - 一括リロード + CI ステータス + isDraft + UI ラベル/色体系のGitHub準拠化）
- **次にやること**: Phase 2.5（Task/Story データモデル統合）→ Phase 2c（チャットからのタスク操作）

## Roadmap

### Phase 1: MVP ✅
- Go backend: API server + SQLite + Qdrant + Ollama 連携
- React dashboard: タスク CRUD +「次に何すべき？」ボタン + チャット
- 手動入力のみ、外部連携なし
- 単一バイナリ (embed.FS)
- **完了条件**: タスクを入れたら LLM がセマンティック検索で文脈を理解して次にやることを教えてくれる

### Phase 2: 外部連携 + 構造的依存認識 ← 次
- **完了条件**: PR URL を貼れば依存構造を認識し、「次にどのPRを見ればいいか」を自動提案できる

#### Phase 2a: ストーリー/レイヤーのデータモデル + CRUD ✅
- SQLite に `stories` / `story_layers` テーブル追加（マイグレーション）
- ストーリー CRUD API（`POST/GET/PUT/DELETE /api/stories`）
- ストーリーにレイヤーを手動で追加・管理する API（`POST /api/stories/{id}/layers`）
- レイヤーチェーン定義 API（`GET /api/layer-chains`）
- ダッシュボードにストーリー一覧 + パイプラインビュー（各レイヤーの状態を横並び表示）
- Suggest / Chat プロンプトにストーリー/レイヤー情報を組み込み

#### Phase 2b: GitHub 連携（gh コマンド） ✅
- `internal/github` パッケージ: `os/exec` で `gh pr view` を呼ぶラッパー
- PR URL を story_layer に貼り付け → gh で PR 情報取得・表示
- Suggest/チャット時にストーリーに紐づく PR の最新状態を gh で取得してからプロンプトに注入
- 上流レイヤーの PR マージ状態による自動ブロッキング判定

#### Phase 2b': GitHub 連携強化 ✅
- ストーリー単位の PR ステータス一括リロードボタンを UI に追加（`POST /api/stories/refresh-prs`）
- `gh pr view` の取得項目に `statusCheckRollup`, `isDraft` を追加
- `StoryLayer` モデル / DB に `pr_is_draft`（INTEGER）, `pr_check_status`（TEXT）フィールドを追加
- パイプラインビューに CI ステータス表示（✓/✗/⏳ アイコン + ラベル）
- PR ステータスラベルを GitHub 準拠に変更（マージ済み:紫 / オープン:緑 / オープン（ドラフト）:灰 / 未着手:灰）
- ブロック状態はラベルではなく視覚表現（opacity + dashed border）のみで表現
- Suggest/Chat プロンプトに CI 結果・draft 状態を含める（CI 失敗時に修正を促す判断基準を追加）

#### Phase 2.5: Task/Story データモデル統合
- Task と Story を統合し、単一のデータモデル（Story）で単タスクも複数レイヤー施策も表現する
- UI のタスク/ストーリータブを廃止し、統合ビューに一本化
- 詳細は後述の「Phase 2.5 設計ドキュメント」セクションを参照

#### Phase 2c: チャットからのタスク操作
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
- **完了条件**: 音声でタスク追加・状況確認ができる

### Phase 4: フル統合
- Slack / Miro / スプレッドシート連携
- 複数サービスを横断した自動タスク推定
- **完了条件**: 外部サービスから自動的にやるべきことを推定できる

## Architecture Decisions

- 2026-04-02: バックエンドに Go を採用。cmd/ + internal/ パターン、単一バイナリ配布の利点。
- 2026-04-02: フロントエンドに React + Vite SPA を採用。将来の音声統合に有利、Go embed.FS でバイナリに同梱。
- 2026-04-02: Chat LLM に Ollama (qwen2.5:14b) を採用。ローカル無料、日本語に強い14Bモデル。Provider インターフェース抽象化で後から差替可能。動作確認は qwen2.5:7b で実施済み。
- 2026-04-02: Embedding に Ollama (nomic-embed-text, 768次元) を採用。Ollama の /api/embed エンドポイントで統一。
- 2026-04-02: タスク管理の正規ストアに SQLite (modernc.org/sqlite, CGO不要) を採用。タスクCRUD、会話履歴、サマリーなど構造化データの永続化。
- 2026-04-02: セマンティック検索用の記憶層に既存 Qdrant Docker インスタンス (localhost:6333) を流用。タスク説明文・メモ・会話をベクトル化し「意味的に関連する過去の文脈」を引き出す用途。タスク管理自体は SQLite が担う。コレクション名は `maidirector`。
- 2026-04-02: プロジェクト名を maidirector に決定。メイドラゴンリスペクト。
- 2026-04-02: Qdrant Go client (gRPC依存) ではなく REST API を直接叩く実装を採用。依存の軽さを優先。
- 2026-04-02: GitHub 連携に gh CLI コマンドを採用。ローカルに導入・認証済みのため OAuth トークン管理不要。Go から os/exec で呼ぶだけでプライベートリポジトリにもアクセス可能。
- 2026-04-02: GitHub PR 状態取得はポーリングではなくイベント駆動を採用。Suggest 押下時・PR URL 貼付時・チャット質問時に都度 gh コマンドで取得。バックグラウンドプロセス不要。
- 2026-04-02: ベクトル化の対象をタスク・コンテキスト・会話の3種に拡大。タスク/コンテキストは `api.Server` が `vectorStore` を直接保持して goroutine で非同期 upsert。会話は従来通り `context.Manager` 経由。`SourceID` が設定されたポイントはべき等更新（タスク更新時に同一ポイントIDで上書き）。
- 2026-04-02: タスク削除時のベクトルクリーンアップを Qdrant REST API の `POST /points/delete` で実装。タスク本体は `DeletePoints`（ポイントID指定）、紐づく context_entry は `DeleteByFilter`（`task_id` フィルタ）の2段階で削除。会話ベクトルはタスクに紐づかないため削除対象外。

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

## Feature Design: レイヤー依存グラフ（優先度: 高）

### 課題

施策（ストーリー）の開発は proto → microservice/gateway → backend → frontend のようにレイヤーごとに構造的なブロッキング関係がある。例:

- proto の PR がマージされないと microservice や gateway の PR がマージできない
- microservice/gateway が完了しないと frontend の PR がマージできない

これが複数のストーリーで並行して進行するため、「今どのストーリーのどのレイヤーまで進んでいるか」「次にどのPRを見ればいいか」の把握が困難になる。

現状は手動でタスクの優先順位やブロッキングを入力しているが、この依存関係はプロジェクト構造から自動的に認識させたい。

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

1. **レイヤー定義**: プロジェクトごとにレイヤーの依存順序を設定ファイルで定義
   ```yaml
   layer_chains:
     - name: default
       layers: [proto, microservice, gateway, frontend]
     - name: simple
       layers: [backend, frontend]
   ```

2. **ストーリー単位のグルーピング**: タスク/PRをストーリー（施策）単位でグルーピングし、各レイヤーの進捗を一元管理

3. **GitHub PR との紐付け**:
   - ユーザーが PR の URL を貼る → `gh` コマンドで PR 情報を取得してレイヤーに紐付け
   - ブランチ名やラベルからレイヤーを推定（例: `feature/story-a/proto`）も補助的に使う
   - PR 状態の取得は `gh pr view <url> --json state,mergeable,reviews` で実現

4. **自動ブロッキング判定**: 上流レイヤーの PR がマージ済みかどうかで下流レイヤーのブロック状態を自動判定

5. **ダッシュボード表示**: ストーリーごとのパイプラインビュー（各レイヤーの状態を横並びで表示）

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

ブロック判定は `blocked_by` カラムではなく `sort_order` を使い表示時に計算する方式を採用。直前のレイヤーが merged でなければブロック中と判定する。

### 「次にやること」への統合（Phase 2b' まで実装済み）

Suggest / Chat プロンプトにストーリー/レイヤー情報を注入済み:
- ✅ アクティブなストーリーのパイプライン状況（各レイヤーのステータス + ブロック判定）をプロンプトに含める
- ✅ 判断基準に「上流がマージ済みで下流が未着手の場合、着手を提案」を追加
- ✅ PR の実際の状態を gh コマンドで取得してから判定（Suggest/Chat 実行前に全 PR を一括リフレッシュ、マージ済みレイヤーはスキップ）
- ✅ PR の reviewDecision (APPROVED/CHANGES_REQUESTED) に基づく提案を追加
- ✅ PR の isDraft 状態をプロンプトに含める
- ✅ CI ステータス (SUCCESS/FAILURE/PENDING) に基づく提案を追加（FAILURE → 修正促進、PENDING → 待ち通知）
- 将来: 全レイヤー完了時にストーリーを自動完了扱い

### GitHub 連携方針: gh コマンド + イベント駆動

ポーリングではなく、ユーザーアクション起点で PR 状態を取得する方針。

**なぜ gh コマンドか:**
- ローカルに gh CLI が導入済み、認証もセットアップ済み
- OAuth トークン管理が不要（gh auth の仕組みに乗れる）
- 対象のリポジトリにプライベートなものが多いので gh コマンド経由が自然
- Go から `os/exec` で呼ぶだけなので実装がシンプル

**取得タイミング（イベント駆動）:**
- 「提案してもらう」ボタン押下時 → Suggest 前に全紐付き PR の最新状態を gh で取得
- PR URL をタスクやストーリーに貼り付けた時 → その場で PR 情報を取得・表示
- チャットで「ストーリーAの状況教えて」→ gh で各レイヤー PR の状態を取得してからLLMに渡す

**gh コマンド活用例:**
```bash
# PR の状態取得
gh pr view <url> --json number,title,state,isDraft,mergeable,reviewDecision,url,statusCheckRollup

# 自分に関連する PR 一覧
gh pr list --author @me --json number,title,url,headRefName,state

# 特定リポジトリの PR 検索
gh pr list --repo <owner>/<repo> --search "label:story-a" --json number,title,url,state
```

**段階的な自動化:**
1. まず: URL を手動で貼る → gh で情報取得・表示
2. 次に: ブランチ命名規約から自動でストーリー/レイヤーを推定
3. 最終的に: `gh pr list` でアクティブな PR を定期スキャンし、未登録のストーリーを提案

### 実装フェーズ

- ✅ Phase 2a: ストーリー/レイヤーのデータモデル・CRUD・パイプラインビュー・Suggest 統合を実装済み
- ✅ Phase 2b: gh コマンド連携による PR 状態取得・自動ブロッキング判定を実装済み
- ✅ Phase 2b': 一括リロード・CI ステータス・isDraft・UI ラベル/色体系の GitHub 準拠化を実装済み
- Phase 2.5: Task/Story データモデル統合（次に着手）

## Known Issues / TODO

### Phase 1 残作業 ✅
- ~~タスク作成時のベクトル化~~ → 実装済み（タスク作成/更新時に Qdrant upsert、SourceID でべき等更新）
- ~~context_entry のベクトル化~~ → 実装済み（メモ追加時に Qdrant upsert）
- ~~UI のリスト更新~~ → 楽観的更新に改善（作成レスポンスを即座に反映 + WebSocket重複抑制）
- ~~favicon~~ → maidirector 用 SVG に差し替え

### 軽微な残課題 ✅
- ~~タスク削除時の Qdrant クリーンアップ~~ → 実装済み（`DeletePoints` でタスク本体、`DeleteByFilter` で紐づく context_entry を削除）

### Phase 2a 実装済み ✅
- ストーリー/レイヤーのデータモデル + CRUD + パイプラインビュー + Suggest統合

### Phase 2b 実装済み ✅
- GitHub 連携（gh コマンドによる PR 状態取得・自動ブロッキング判定）

### Phase 2b' 実装済み ✅
- GitHub 連携強化（一括リロード・CI ステータス・isDraft・UI ラベル/色体系の GitHub 準拠化・マージ済みレイヤーのリフレッシュスキップ）

### Phase 2.5 で対応予定
- Task/Story データモデル統合（詳細は「Phase 2.5 設計ドキュメント」セクションを参照）

### Phase 2c で対応予定
- → Roadmap の Phase 2c を参照（Phase 2.5 完了後に着手）

### 将来の検討事項
- Chrome SpeechRecognition API のブラウザ制限 (Phase 3)

## Phase 2.5 設計ドキュメント（引き継ぎ用・実装完了後に削除）

> **注意**: このセクションは Phase 2.5 実装時の引き継ぎ用に詳細を記載している。実装完了後、セッション最後にこのセクション全体を削除すること。

### 背景・動機

現在 Task と Story が別のデータモデル・別の UI タブとして存在している。しかし:

- LLM の Suggest/Chat では既に Task と Story を横断的に扱っている（`buildSuggestPrompt`/`buildChatPrompt` で両方取得してプロンプトに注入）
- ユーザーから見ると「やるべきこと」を2箇所で管理するのは認知負荷が高い
- Phase 2c（チャットからの操作）で操作対象が Task と Story の2系統あると tool use の設計が複雑化する

→ Story にデータモデルを統合し、単タスク = レイヤー1個のストーリーとして扱うことで UI とバックエンドをシンプルにする。

### 方針: Story 側に寄せる

Task を Story の特殊ケース（レイヤー0〜1個のストーリー）として統合する。

**理由**: Story の方が表現力が高い（レイヤー依存・PR連携・CI連携を既に持っている）。Task の各フィールドは Story に自然にマッピングできる。

### 現行 Task フィールドの統合マッピング

```
Task.Title        → Story.Title
Task.Description  → Story.Description
Task.Status       → Story.Status（型の統合が必要、後述）
Task.Priority     → Story に priority フィールドを新設
Task.Project      → Story に project フィールドを新設
Task.Links        → Story に links フィールドを新設（JSON TEXT）
Task.Tags         → Story に tags フィールドを新設（JSON TEXT）
Task.BlockedBy    → 廃止。Story 内のレイヤー依存（sort_order）で表現
Task.LastActiveAt → Story に last_active_at フィールドを新設
ContextEntry      → task_id を story_id に変更
```

### ステータス体系の統合

現在の2つの体系を統合する必要がある:

```
Task:  todo / in_progress / blocked / done
Story: active / completed / archived
Layer: pending / in_progress / review / merged
```

統合案:
- Story レベルのステータスは **active / completed / archived** を維持（これが一覧のフィルタに使われる）
- 旧 Task の `todo` → active な Story（レイヤーが全て pending）
- 旧 Task の `in_progress` → active な Story（レイヤーが in_progress）
- 旧 Task の `blocked` → UI 上のブロック表現で代替（sort_order ベースの自動判定が既にある）
- 旧 Task の `done` → completed な Story（全レイヤー merged、または手動で completed に変更）

### DB マイグレーション方針

**新規テーブルは作らない**。既存の `stories` テーブルにカラムを追加する:

```sql
ALTER TABLE stories ADD COLUMN priority INTEGER DEFAULT 0;
ALTER TABLE stories ADD COLUMN project TEXT DEFAULT '';
ALTER TABLE stories ADD COLUMN links TEXT DEFAULT '[]';
ALTER TABLE stories ADD COLUMN tags TEXT DEFAULT '[]';
ALTER TABLE stories ADD COLUMN last_active_at DATETIME;
```

旧 `tasks` テーブルのデータは以下の手順で移行:
1. 各 Task を Story として INSERT
2. Task に PR 情報はないので、レイヤーは作成しない（レイヤー0個の Story = 単タスク）
3. `context_entries` テーブルの `task_id` を `story_id` にリネーム（`ALTER TABLE context_entries RENAME COLUMN task_id TO story_id`）
4. 移行完了後、`tasks` テーブルは DROP（または残しておいてもよい）

### ベクトル化の変更

現在 Task 単位で Qdrant に upsert している箇所を Story 単位に変更:
- `api.Server.handleCreateTask` → `handleCreateStory` に統合
- `source_type: "task"` → `source_type: "story"` に変更
- `SourceID` は Story.ID を使用
- ContextEntry のベクトル化は `task_id` → `story_id` に変更するだけ
- 削除時のクリーンアップも同様

### API の変更

Task 系の API を廃止し、Story 系に統合:

```
廃止:
  GET/POST /api/tasks
  GET/PUT/DELETE /api/tasks/{id}
  POST/GET /api/tasks/{id}/context

統合後（既存 Story API を拡張）:
  GET/POST /api/stories               ← 既存（priority, project, tags, links を追加）
  GET/PUT/DELETE /api/stories/{id}     ← 既存
  POST /api/stories/{id}/layers        ← 既存
  PUT/DELETE /api/stories/{storyId}/layers/{layerId} ← 既存
  POST/GET /api/stories/{id}/context   ← 新規（旧 tasks/{id}/context を移行）
  POST /api/stories/refresh-prs        ← 既存
  POST /api/stories/{storyId}/layers/{layerId}/refresh-pr ← 既存
```

### フロントエンド UI の変更

**廃止するもの:**
- `nav-tabs`（タスク/ストーリー切替タブ）
- `TaskList` コンポーネント
- `TaskForm` コンポーネント
- `App.tsx` の `mainView` state と切替ロジック

**統合ビューの設計:**
- 左パネルにストーリー一覧を表示（旧 Task も含む）
- 単タスク（レイヤー0個）: タイトル + ステータスバッジのみ表示（パイプラインなし）
- 複数レイヤー: 現在のパイプラインビューをそのまま表示
- 作成フォーム: StoryForm を拡張。レイヤーチェーン未選択 = 単タスク扱い
- フィルタ: project / tags / status でフィルタリング（旧 TaskList のフィルタバーを統合）
- priority の表示: ストーリーカード上に表示（ソート順にも使用）

### プロンプト生成の変更

`context/manager.go` の変更:
- `filterActiveTasks` + `formatTasks` を廃止
- `formatStories` に priority/project/tags 情報を追加
- `buildSuggestPrompt` / `buildChatPrompt` から Task 取得ロジックを削除し、Story のみに統一
- `buildSuggestSystemPrompt` のタスク一覧セクションをストーリー一覧セクションに変更

### 実装順序（推奨）

1. **バックエンド: Story モデル拡張 + DB マイグレーション**
   - `model.go`: Story 構造体に priority/project/links/tags/last_active_at を追加
   - `store.go`: マイグレーション追加、Story CRUD クエリを新カラム対応に更新
   - `store.go`: ContextEntry の story_id 対応（旧 task_id からの移行）

2. **バックエンド: Task → Story データ移行**
   - `store.go`: マイグレーション内で tasks テーブルのデータを stories に移行するロジック
   - 移行は起動時マイグレーションで自動実行（tasks テーブルが存在する場合のみ）

3. **バックエンド: API 統合**
   - Task 系ハンドラを削除、Story ハンドラに context エンドポイントを追加
   - ベクトル化ロジックを Story ベースに変更

4. **バックエンド: プロンプト生成の統合**
   - `context/manager.go`: Task 関連ロジックを削除、Story のみに統一

5. **フロントエンド: 型定義 + API クライアント**
   - `types.ts`: Task 型を削除、Story 型に新フィールド追加
   - `client.ts`: Task 系メソッドを削除、Story 系メソッドに context を追加

6. **フロントエンド: 統合ビュー**
   - `App.tsx`: タブ切替を廃止、統合ビューに変更
   - `StoryPipeline.tsx`: 単タスク（レイヤー0個）の表示対応を追加
   - `TaskList.tsx`, `TaskForm.tsx`: 削除
   - `StoryForm.tsx`: priority/project/tags 入力を追加

7. **動作確認 + クリーンアップ**
   - 既存データの移行確認
   - 全 UI フローの確認
   - 不要コードの削除

### 注意事項

- `tasks` テーブルに既存データがある場合の移行を安全に行う（マイグレーション内で `SELECT * FROM tasks` → `INSERT INTO stories` のバッチ処理）
- `context_entries.task_id` → `story_id` のリネームは SQLite では `ALTER TABLE RENAME COLUMN` が使える（SQLite 3.25.0+、modernc.org/sqlite は対応済み）
- WebSocket イベントの `task_*` 系を廃止し `story_*` 系に統一
- Qdrant の既存ベクトル（`source_type: "task"`）は、検索時に `source_type` でフィルタしていなければそのまま残しても問題ない。気になるなら移行時に `source_type` を書き換える

## Session Log

### 2026-04-02: 初回セッション - 計画策定 + Phase 1 MVP 実装 + 動作確認
- プロジェクトの要件定義と議論
- 技術選定の決定 (Go + React + Ollama + SQLite + Qdrant)
- アーキテクチャ設計、データモデル設計、API 設計
- フェーズ分けとロードマップ策定
- プロジェクト名を maidirector に決定
- Phase 1 MVP のフルスタック実装を完了:
  - Go backend: config, store(SQLite), vectorstore(Qdrant), llm(Ollama), task, context, api, ws
  - React frontend: TaskList, TaskForm, SuggestPanel, ChatPanel, WebSocket hook
  - embed.FS で単一バイナリ化 (16MB)
  - Makefile でビルドフロー統合
- 動作確認完了:
  - サーバー起動 (localhost:8080) + ダッシュボード表示確認
  - タスクCRUD (API + UI両方で動作確認済み)
  - 「次にやること」提案機能 (Ollama qwen2.5:7b でSSEストリーミング動作)
  - チャット機能 (タスク状況を踏まえた応答を確認)
  - Qdrant コレクション自動作成 + 会話のベクトル化疎通
- Phase 2 の設計議論:
  - レイヤー依存グラフの設計（proto → microservice → gateway → frontend）
  - gh コマンドによる GitHub 連携方針の決定（イベント駆動、ポーリングなし）
  - チャットからのタスク操作の方向性整理
- 確認時に qwen2.5:7b を使用（14bは後日pullして切替）

### 2026-04-02: Phase 1 残作業の実装
- タスク作成/更新時のベクトル化を実装:
  - `api.Server` に `vectorStore` を注入し、`handleCreateTask`/`handleUpdateTask` で goroutine でベクトル化
  - `vectorstore.Upsert` を改修: `SourceID` が設定されていればそれをポイントIDとして使用（べき等更新可能に）
  - タスクのタイトル+説明+プロジェクト名をベクトル化コンテンツとして構成
- context_entry（メモ）のベクトル化を実装:
  - `handleAddContext` で goroutine でベクトル化、`source_type: "context_entry"` として Qdrant に保存
  - ドキュメントには「context_entry 追加時にベクトル化」と記載されていたが、実際のコードでは未実装だったため修正
- UI の楽観的更新を実装:
  - `TaskForm` が `createTask` のレスポンス（作成されたタスク）を `onCreated` 経由で親に返すように変更
  - `App.tsx` でレスポンスのタスクを即座に state に追加（サーバーからの再取得を待たない）
  - WebSocket の `task_created` による重複リフレッシュを 500ms 間抑制
  - 送信中の二重送信防止（`submitting` state）を追加
- favicon を maidirector 用の SVG に差し替え
- タスク削除時の Qdrant ベクトルクリーンアップを実装:
  - `vectorstore.DeletePoints`（ポイントID指定削除）と `vectorstore.DeleteByFilter`（フィルタ削除）を追加
  - `handleDeleteTask` で goroutine により非同期でタスク本体 + 紐づく context_entry のベクトルを削除
- 動作確認完了（サーバー起動 + 実際の Ollama/Qdrant 連携）:
  - タスク作成 → Qdrant に `source_type: "task"` でベクトル保存、ポイントID = タスクID
  - タスク更新 → 同一ポイントIDでべき等上書き（ポイント数が増えない）
  - コンテキスト追加 → Qdrant に `source_type: "context_entry"` でベクトル保存、`task_id` 付き
  - タスク削除 → タスクベクトル + 紐づく context_entry ベクトルが削除、会話ベクトルは残留
  - UI 楽観的更新 → タスク追加ボタンが「追加中...」に変化（二重送信防止）、レスポンス後に即座にリスト反映
  - ベクトル化失敗時（Ollama停止中）→ タスクCRUD自体はブロックされず、warning ログのみ

### 2026-04-02: Phase 2a - ストーリー/レイヤーのデータモデル + CRUD + パイプラインビュー
- Config に `layer_chains` 設定を追加:
  - `LayerChainConfig` 型（name + layers []string）を `config.go` に追加
  - デフォルト設定に `default`（proto→microservice→gateway→frontend）と `simple`（backend→frontend）を定義
  - `maidirector.yml` にも `layer_chains` セクションを追加
- データモデル追加:
  - `task/model.go` に `Story`（id, title, description, layer_chain, status）と `StoryLayer`（id, story_id, layer_name, status, pr_url, pr_number, pr_state, sort_order）を追加
  - `StoryStatus`（active/completed/archived）、`LayerStatus`（pending/in_progress/review/merged）の型定義
- SQLite マイグレーション追加:
  - `stories` テーブル + `story_layers` テーブル（ON DELETE CASCADE）
  - `idx_stories_status` / `idx_story_layers_story_id` インデックス
- Store に Story/StoryLayer CRUD メソッドを実装:
  - `CreateStory`/`GetStory`/`ListStories`/`UpdateStory`/`DeleteStory`
  - `CreateStoryLayer`/`GetStoryLayer`/`ListStoryLayers`/`UpdateStoryLayer`/`DeleteStoryLayer`
  - `GetStory`/`ListStories` はレイヤーを自動的に JOIN して返す
  - レイヤー追加/更新/削除時に親ストーリーの `updated_at` を更新
- `StoryStore` インターフェースと `StoryService` を `task/service.go` に追加
- API ルートとハンドラを追加:
  - `GET/POST /api/stories`, `GET/PUT/DELETE /api/stories/{id}`
  - `POST /api/stories/{id}/layers`, `PUT/DELETE /api/stories/{storyId}/layers/{layerId}`
  - `GET /api/layer-chains`（設定ファイルのレイヤーチェーン定義を返す）
  - WebSocket で `story_created`/`story_updated`/`story_deleted` をブロードキャスト
- フロントエンド実装:
  - `types.ts` に `Story`/`StoryLayer`/`LayerChain`/`CreateStoryRequest`/`CreateLayerRequest` 型追加
  - `client.ts` に Story CRUD + Layer CRUD + LayerChains 取得の API メソッド追加
  - `StoryForm` コンポーネント: ストーリー作成フォーム（レイヤーチェーン選択付き）
  - `StoryPipeline` コンポーネント: パイプラインビュー（レイヤー横並び表示、ステータス色分け、ブロック判定、レイヤー管理UI）
  - `App.tsx` にタスク/ストーリー切替のナビゲーションタブを追加
  - WebSocket で `story_*` イベント受信時にストーリー一覧を自動リフレッシュ
- Suggest/Chat プロンプトにストーリー情報を統合:
  - `ConversationStore` インターフェースに `ListStories` を追加
  - `buildSuggestPrompt` でアクティブなストーリーのパイプライン状況をプロンプトに注入
  - `buildChatPrompt` にもストーリー情報セクションを追加
  - 判断基準に「上流がマージ済みで下流が未着手の場合、着手を提案」を追加
  - `formatStories` でブロック状態を含むレイヤーサマリーを生成
- コンパイル確認: Go build + TypeScript type check ともにエラーなし
- 動作確認完了:
  - サーバー起動 → stories / story_layers テーブルのマイグレーション成功
  - ストーリー CRUD（curl）: POST 201 / GET 200 / PUT 200 / DELETE 204 すべて正常
  - レイヤー CRUD（curl）: POST 201（sort_order 自動連番）/ PUT 200（status 変更+PR情報）/ DELETE 204
  - GET /api/stories/{id} でレイヤーが sort_order 順に JOIN されて返ることを確認
  - レイヤー追加/更新/削除時に親ストーリーの updated_at が更新されることを確認
  - GET /api/layer-chains で config の layer_chain 定義が JSON で返ることを確認
  - フロントエンドビルド: `tsc -b && vite build` 成功（25 modules, 64.77kB gzip）
  - ブラウザ UI 確認:
    - タスク/ストーリーのナビゲーションタブ切替が動作
    - ストーリー作成フォーム: タイトル入力 → 「追加中...」→ カード表示（楽観的更新）
    - レイヤー管理 UI: ⚙ボタンでレイヤーチェーン定義に基づくクイック追加ボタン表示
    - パイプラインビュー: 4レイヤー（proto→microservice→gateway→frontend）が矢印で横並び表示
    - ブロック判定: proto=未着手時に下流3レイヤーが「ブロック中」（破線+opacity低下）で表示
    - ステータス変更: proto を「マージ済」に変更 → microservice のブロックが解除（正常判定）
    - WebSocket によるストーリーのリアルタイム更新が動作
- バグ修正: `LayerChainConfig` に JSON タグが欠如していた問題を修正（`Name`→`name`, `Layers`→`layers`）

### 2026-04-02: Phase 2b - GitHub 連携（gh コマンド）
- `internal/github` パッケージ新規作成:
  - `Client` 構造体: `gh pr view <url> --json ...` を `os/exec` で実行するラッパー
  - `GetPRInfo(prURL)`: PR の number, title, state, mergeable, reviewDecision, url を取得
  - `IsAvailable()`: gh CLI の認証状態を確認（サーバー起動時に利用可能性をチェック）
  - `ParsePRURL()`: GitHub PR URL から owner/repo/number を抽出するユーティリティ
- データモデル拡張:
  - `StoryLayer` に `PRTitle` / `PRReviewDecision` フィールドを追加
  - SQLite マイグレーション: `ALTER TABLE story_layers ADD COLUMN pr_title/pr_review_decision`
  - ALTER TABLE の「duplicate column」エラーをスキップする安全なマイグレーション処理
  - Store の全 StoryLayer CRUD クエリ（INSERT/SELECT/UPDATE）を新カラム対応に更新
- API 拡張:
  - `Server` 構造体に `ghClient *github.Client` を追加、`NewServer` の引数に追加
  - `handleUpdateLayer`: PR URL が新規設定・変更された場合、`applyPRInfo` で gh コマンドを呼び PR 情報を自動取得
  - `POST /api/stories/{storyId}/layers/{layerId}/refresh-pr`: 個別レイヤーの PR 情報を手動リフレッシュ
  - `applyPRInfo`: gh から取得した PR 情報を layer に適用、PR state が MERGED なら layer status も自動で merged に更新
  - `refreshPRStates`: 全アクティブストーリーの PR 紐付きレイヤーを一括リフレッシュ
  - `handleSuggest` / `handleChat`: 実行前に `refreshPRStates` を呼び出し、最新の PR 状態をプロンプトに反映
- Suggest/Chat プロンプト強化:
  - `formatStories` で PR の state, reviewDecision, title をプロンプトに含める
  - 判断基準に「PRのレビューステータスがAPPROVEDで未マージならマージを促す」「CHANGES_REQUESTEDなら修正対応を促す」を追加
- DI 統合:
  - `main.go` で `github.NewClient()` を生成、`IsAvailable()` で認証状態をチェック
  - 未認証の場合は nil を渡して PR 連携を無効化（タスク/ストーリー CRUD は影響なし）
- フロントエンド実装:
  - `StoryLayer` 型に `pr_title`, `pr_review_decision` を追加
  - `api.refreshLayerPR()` メソッドを追加
  - `StoryPipeline` コンポーネントを大幅強化:
    - `PipelineStage` サブコンポーネントに分離
    - PR URL 入力フォーム（「+ PR」ボタン → URL入力 → OK で保存）
    - PR 情報表示: PR番号リンク、タイトル、state バッジ（OPEN/MERGED/CLOSED 色分け）、reviewDecision
    - ↻ ボタンで PR 情報を個別リフレッシュ
    - LayerManager に PR 状態のサマリー表示を追加
  - CSS 追加: PR 情報表示用スタイル（state バッジ色分け、review 色分け、PR フォーム等）
- コンパイル確認: Go build + TypeScript type check ともにエラーなし
- 動作確認完了:
  - サーバー起動 → 「GitHub CLI (gh) 認証済み: PR連携が有効です」ログ確認
  - レイヤー更新で PR URL 設定 → gh 自動呼び出し: pr_number=2003, pr_state="OPEN", pr_title, pr_review_decision="REVIEW_REQUIRED" が取得・保存されることを確認
  - 存在しない PR URL → warning ログのみでエラーにならないことを確認
  - `POST .../refresh-pr` → 最新の PR 情報で更新されることを確認
  - ストーリー GET → layers に新カラム（pr_title, pr_review_decision）が含まれることを確認
  - フロントエンドビルド成功（11.04kB CSS, 210.80kB JS gzip 65.38kB）
  - ブラウザ UI 確認（後追い）:
    - ストーリータブ: パイプラインビューに PR 情報（PR番号リンク、↻ボタン、タイトル、OPEN バッジ、Review Required）が正しく表示
    - ブロック判定: proto が未着手のとき下流レイヤーが全てブロック中（破線+opacity低下）で表示
    - 「+ PR」ボタン: クリックで PR URL 入力フォーム表示、✕ で閉じてボタンに戻る動作を確認
    - WebSocket によるリアルタイム更新が正常動作
- 開発環境一括起動スクリプト `dev.sh` を追加:
  - コマンド存在確認（go/node/npm/docker/ollama/gh）→ 不足時は brew install ヒント表示
  - Qdrant Docker コンテナ起動（起動済みならスキップ）
  - Ollama 起動 + モデル確認（qwen2.5:7b, nomic-embed-text）
  - gh CLI 認証確認（未認証でも続行可）
  - npm install（node_modules 存在時スキップ）
  - バックエンド/フロントエンド dev server 起動（ポート使用中ならスキップ）
  - 全てべき等: 2回目以降は起動済みサービスを全スキップ

### Phase 2b' 実装（GitHub 連携強化）
- `gh pr view` の取得項目に `isDraft`, `statusCheckRollup` を追加
  - `checkRollupItem` 構造体で StatusCheckRollup の各 check を解析
  - `summarizeChecks()`: SUCCESS/FAILURE/PENDING を集約して1ステータスに
- データモデル拡張:
  - `PRInfo` に `IsDraft bool`, `CheckStatus string` を追加
  - `StoryLayer` に `PRIsDraft bool`, `PRCheckStatus string` を追加
  - SQLite マイグレーション: `pr_is_draft INTEGER DEFAULT 0`, `pr_check_status TEXT DEFAULT ''`
  - `scanStoryLayer()` ヘルパー関数で bool ↔ INTEGER 変換を共通化
- API 拡張:
  - `POST /api/stories/refresh-prs`: 全アクティブストーリーの PR 情報を一括更新
  - `applyPRInfo()` に isDraft, checkStatus の反映を追加
- フロントエンド:
  - PR ステータスラベルを GitHub 準拠に: マージ済み（紫）/ オープン（緑）/ オープン（ドラフト）（灰）/ 未着手（灰）
  - ブロック状態はラベルに影響せず opacity + dashed border のみで表現
  - CI ステータス表示: ✓ CI pass（緑）/ ✗ CI fail（赤）/ ⏳ CI 実行中（黄）
  - 「↻ 全PR情報を更新」グローバルリフレッシュボタンを追加
  - `api.refreshAllPRs()` メソッド追加
- LLM プロンプト:
  - `formatStories()` に draft, ci 情報を追加
  - 判断基準に CI FAILURE → 修正促進、CI PENDING → 待ち通知を追加
- 最適化:
  - `refreshPRStates()` でマージ済みレイヤー（`status == merged`）をスキップ（不要な gh API コール削減）
- コンパイル確認: Go build + TypeScript type check ともにエラーなし
