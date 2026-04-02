# maidirector - Development Note

メイド（maid）+ ディレクター（director）の合成語。
並行する施策開発のコンテキストスイッチ問題を解決するローカル完結の開発AIアシスタント。

## Status

- **現在のフェーズ**: Phase 1 (MVP) - 動作確認完了
- **直近の完了**: Phase 1 MVP のフルスタック実装 + 動作確認（タスクCRUD、LLM提案、チャット全て動作）
- **次にやること**: タスク作成時のベクトル化追加、UIのリフレッシュ改善、Phase 2 の GitHub 連携検討

## Roadmap

### Phase 1: MVP
- Go backend: API server + SQLite + Qdrant + Ollama 連携
- React dashboard: タスク CRUD +「次に何すべき？」ボタン + チャット
- 手動入力のみ、外部連携なし
- 単一バイナリ (embed.FS)
- **完了条件**: タスクを入れたら LLM がセマンティック検索で文脈を理解して次にやることを教えてくれる

### Phase 2: 外部連携 + 構造的依存認識 + 自律動作
- **GitHub 連携 (gh コマンド経由、イベント駆動)**: Suggest/チャット時に PR 状態を取得
- **レイヤー依存グラフ**: proto → microservice → gateway → frontend の構造的ブロッキング自動認識（後述）
- **ストーリー管理**: 施策単位でレイヤーごとの PR 進捗をパイプラインビューで表示
- **チャットからのタスク操作**: LLM tool use でタスクCRUD/ストーリー登録を会話から実行
- macOS デスクトップ通知 (osascript)
- ローリングサマリーの自動化
- **完了条件**: PR URL を貼れば依存構造を認識し、「次にどのPRを見ればいいか」を自動提案できる

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
- 2026-04-02: Chat LLM に Ollama (qwen2.5:14b) を採用。ローカル無料、日本語に強い14Bモデル。Provider インターフェース抽象化で後から差替可能。
- 2026-04-02: Embedding に Ollama (nomic-embed-text, 768次元) を採用。Ollama の /api/embeddings で統一。
- 2026-04-02: 構造化 Storage に SQLite (modernc.org/sqlite, CGO不要) を採用。
- 2026-04-02: ベクトル Storage に既存 Qdrant Docker インスタンス (localhost:6333) を流用。コレクション名は `maidirector`。
- 2026-04-02: プロジェクト名を maidirector に決定。メイドラゴンリスペクト。

## Tech Stack

| 要素 | 選定 | バージョン/備考 |
|------|------|----------------|
| Backend | Go + Cobra CLI + net/http | go 1.22+ |
| Frontend | React + Vite + TypeScript | - |
| Chat LLM | Ollama / qwen2.5:14b | ローカル無料 |
| Embedding | Ollama / nomic-embed-text | 768次元 |
| 構造化DB | SQLite (modernc.org/sqlite) | CGO不要 |
| ベクトルDB | Qdrant | v1.17.0 (既存Docker) |
| Config | YAML (gopkg.in/yaml.v3) | - |

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

### データモデル拡張案

```
stories テーブル
  - id, title, description, layer_chain (参照する依存チェーン名)

story_layers テーブル
  - id, story_id, layer_name, status (pending/in_progress/review/merged)
  - pr_url, pr_number, pr_state
  - blocked_by (上流layer_id)
```

### 「次にやること」への統合

Suggest ロジックに組み込む:
- 上流がマージ済みで下流が未着手 → 「ストーリーAのmicroserviceに取り掛かれます」
- 上流がレビュー中 → 「ストーリーAのproto PRにレビューが必要です」
- 全レイヤー完了 → 自動的にストーリーを完了扱い

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
gh pr view <url> --json state,title,mergeable,reviewDecision,statusCheckRollup

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

Phase 2 の GitHub 連携と同時に着手。まずは手動でストーリー/レイヤーを登録 + URL 貼付けから始め、gh コマンド連携で自動化を進める。

## Known Issues / TODO

- Ollama ストリーミングレスポンスは実装済み（SSE 経由）。実際の動作テストが必要
- Qdrant REST API を直接叩く実装にした（go-client は gRPC 依存が重いため）
- タスク作成時のベクトル化は未実装（context_entry 追加時と会話時のみ）。タスク自体もベクトル化すべきか要検討
- ローリングサマリーの自動化タイミング (Phase 2)
- Chrome SpeechRecognition API のブラウザ制限 (Phase 3)
- `web/dist` の `.gitignore` 設定が必要
- Vite の favicon.svg がデフォルトのまま
- チャットからのタスク操作が未実装（LLM tool use or フロントエンド側パース、Phase 2 で対応予定）

## Session Log

### 2026-04-02: 初回セッション - 計画策定 + Phase 1 MVP 実装
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
  - Qdrant コレクション自動作成 + ベクトル検索疎通
- 確認時に qwen2.5:7b を使用（14bは後日pullして切替）
- 残課題: タスク作成時のベクトル化、UIからのタスク作成後のリストリフレッシュ改善、favicon
