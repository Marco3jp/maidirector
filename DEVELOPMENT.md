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

### Phase 2: 外部連携 + 自律動作
- GitHub 連携 (PR/Issue/レビューのポーリング)
- Background scheduler (定期的に GitHub 確認、リマインド生成)
- macOS デスクトップ通知 (osascript)
- ローリングサマリーの自動化
- **完了条件**: GitHub の状態変化を検知して自動的にタスク/リマインドが生成される

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

## Known Issues / TODO

- Ollama ストリーミングレスポンスは実装済み（SSE 経由）。実際の動作テストが必要
- Qdrant REST API を直接叩く実装にした（go-client は gRPC 依存が重いため）
- タスク作成時のベクトル化は未実装（context_entry 追加時と会話時のみ）。タスク自体もベクトル化すべきか要検討
- ローリングサマリーの自動化タイミング (Phase 2)
- Chrome SpeechRecognition API のブラウザ制限 (Phase 3)
- `web/dist` の `.gitignore` 設定が必要
- Vite の favicon.svg がデフォルトのまま

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
