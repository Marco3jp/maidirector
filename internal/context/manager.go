package context

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	"github.com/Marco3jp/maidirector/internal/llm"
	"github.com/Marco3jp/maidirector/internal/task"
	"github.com/Marco3jp/maidirector/internal/vectorstore"
)

type ConversationStore interface {
	CreateConversation(c *task.Conversation) error
	ListRecentConversations(limit int) ([]*task.Conversation, error)
	GetLatestSummary(scope string) (*task.Summary, error)
	CreateSummary(s *task.Summary) error
	ListTasks(status string, project string) ([]*task.Task, error)
	ListStories(status string) ([]*task.Story, error)
}

type Manager struct {
	store       ConversationStore
	vectorStore *vectorstore.VectorStore
	llmClient   llm.Provider
}

func NewManager(store ConversationStore, vs *vectorstore.VectorStore, llmClient llm.Provider) *Manager {
	return &Manager{
		store:       store,
		vectorStore: vs,
		llmClient:   llmClient,
	}
}

type SuggestResponse struct {
	Suggestion string `json:"suggestion"`
}

func (m *Manager) Suggest(ctx gocontext.Context) (*SuggestResponse, error) {
	messages, err := m.buildSuggestPrompt(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := m.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM suggest failed: %w", err)
	}

	return &SuggestResponse{Suggestion: resp.Content}, nil
}

func (m *Manager) SuggestStream(ctx gocontext.Context, onChunk func(string)) (*SuggestResponse, error) {
	messages, err := m.buildSuggestPrompt(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := m.llmClient.ChatStream(ctx, messages, onChunk)
	if err != nil {
		return nil, fmt.Errorf("LLM suggest stream failed: %w", err)
	}

	return &SuggestResponse{Suggestion: resp.Content}, nil
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
}

func (m *Manager) Chat(ctx gocontext.Context, userMessage string) (*ChatResponse, error) {
	if err := m.store.CreateConversation(&task.Conversation{Role: "user", Content: userMessage}); err != nil {
		return nil, err
	}

	if err := m.vectorStore.Upsert(ctx, vectorstore.PointPayload{
		SourceType: "conversation",
		Content:    userMessage,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}); err != nil {
		// ベクトル化失敗はログだけで続行
		fmt.Printf("warning: failed to vectorize conversation: %v\n", err)
	}

	messages, err := m.buildChatPrompt(ctx, userMessage)
	if err != nil {
		return nil, err
	}

	resp, err := m.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	if err := m.store.CreateConversation(&task.Conversation{Role: "assistant", Content: resp.Content}); err != nil {
		return nil, err
	}

	return &ChatResponse{Reply: resp.Content}, nil
}

func (m *Manager) ChatStream(ctx gocontext.Context, userMessage string, onChunk func(string)) (*ChatResponse, error) {
	if err := m.store.CreateConversation(&task.Conversation{Role: "user", Content: userMessage}); err != nil {
		return nil, err
	}

	_ = m.vectorStore.Upsert(ctx, vectorstore.PointPayload{
		SourceType: "conversation",
		Content:    userMessage,
		CreatedAt:  time.Now().Format(time.RFC3339),
	})

	messages, err := m.buildChatPrompt(ctx, userMessage)
	if err != nil {
		return nil, err
	}

	resp, err := m.llmClient.ChatStream(ctx, messages, onChunk)
	if err != nil {
		return nil, fmt.Errorf("LLM chat stream failed: %w", err)
	}

	if err := m.store.CreateConversation(&task.Conversation{Role: "assistant", Content: resp.Content}); err != nil {
		return nil, err
	}

	return &ChatResponse{Reply: resp.Content}, nil
}

func (m *Manager) buildSuggestPrompt(ctx gocontext.Context) ([]llm.ChatMessage, error) {
	tasks, err := m.store.ListTasks("", "")
	if err != nil {
		return nil, err
	}

	activeTasks := filterActiveTasks(tasks)
	taskSummary := formatTasks(activeTasks)

	stories, _ := m.store.ListStories("active")
	storySummary := formatStories(stories)

	var relatedContext string
	if len(activeTasks) > 0 {
		query := buildRecentActivityQuery(activeTasks)
		results, err := m.vectorStore.Search(ctx, query, 10, nil)
		if err == nil && len(results) > 0 {
			relatedContext = formatSearchResults(results)
		}
	}

	globalSummary, _ := m.store.GetLatestSummary("global")
	var summaryText string
	if globalSummary != nil {
		summaryText = globalSummary.Content
	}

	systemPrompt := buildSuggestSystemPrompt(taskSummary, storySummary, relatedContext, summaryText)

	return []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "今の状況を踏まえて、次に何をすべきか提案してください。優先度の理由も含めて教えてください。"},
	}, nil
}

func (m *Manager) buildChatPrompt(ctx gocontext.Context, userMessage string) ([]llm.ChatMessage, error) {
	recentConvs, _ := m.store.ListRecentConversations(10)

	results, _ := m.vectorStore.Search(ctx, userMessage, 5, nil)
	var relatedContext string
	if len(results) > 0 {
		relatedContext = formatSearchResults(results)
	}

	tasks, _ := m.store.ListTasks("", "")
	activeTasks := filterActiveTasks(tasks)
	taskSummary := formatTasks(activeTasks)

	stories, _ := m.store.ListStories("active")
	storySummary := formatStories(stories)

	var storySection string
	if storySummary != "" {
		storySection = fmt.Sprintf("\n## ストーリー（施策）のパイプライン状況\n%s", storySummary)
	}

	systemPrompt := fmt.Sprintf(`あなたは「maidirector」、開発者の作業を支援するAIアシスタントです。
タスクの状況を把握し、的確なアドバイスを提供してください。

## 現在のタスク一覧
%s
%s
## 関連する過去の文脈
%s

回答は簡潔かつ具体的にお願いします。`, taskSummary, storySection, relatedContext)

	messages := []llm.ChatMessage{{Role: "system", Content: systemPrompt}}

	for i := len(recentConvs) - 1; i >= 0; i-- {
		messages = append(messages, llm.ChatMessage{
			Role:    recentConvs[i].Role,
			Content: recentConvs[i].Content,
		})
	}

	messages = append(messages, llm.ChatMessage{Role: "user", Content: userMessage})
	return messages, nil
}

func filterActiveTasks(tasks []*task.Task) []*task.Task {
	var active []*task.Task
	for _, t := range tasks {
		if t.Status != task.StatusDone {
			active = append(active, t)
		}
	}
	return active
}

func formatTasks(tasks []*task.Task) string {
	if len(tasks) == 0 {
		return "（タスクなし）"
	}

	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(fmt.Sprintf("- [%s] P%d: %s", t.Status, t.Priority, t.Title))
		if t.Project != "" {
			b.WriteString(fmt.Sprintf(" (project: %s)", t.Project))
		}
		if t.Description != "" {
			b.WriteString(fmt.Sprintf("\n  %s", t.Description))
		}
		if len(t.BlockedBy) > 0 {
			b.WriteString(fmt.Sprintf("\n  blocked by: %s", strings.Join(t.BlockedBy, ", ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatSearchResults(results []vectorstore.SearchResult) string {
	if len(results) == 0 {
		return "（関連文脈なし）"
	}

	var b strings.Builder
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- [%s] (score: %.2f) %s\n", r.Payload.SourceType, r.Score, r.Payload.Content))
	}
	return b.String()
}

func buildRecentActivityQuery(tasks []*task.Task) string {
	var titles []string
	for _, t := range tasks {
		if t.Status == task.StatusInProgress {
			titles = append(titles, t.Title)
		}
	}
	if len(titles) == 0 {
		for i, t := range tasks {
			if i >= 3 {
				break
			}
			titles = append(titles, t.Title)
		}
	}
	return strings.Join(titles, " ")
}

func formatStories(stories []*task.Story) string {
	if len(stories) == 0 {
		return ""
	}

	var b strings.Builder
	for _, s := range stories {
		b.WriteString(fmt.Sprintf("### %s (chain: %s)\n", s.Title, s.LayerChain))
		if s.Description != "" {
			b.WriteString(fmt.Sprintf("  %s\n", s.Description))
		}
		if len(s.Layers) == 0 {
			b.WriteString("  レイヤー: なし\n")
			continue
		}
		for i, l := range s.Layers {
			blocked := ""
			if i > 0 && s.Layers[i-1].Status != task.LayerStatusMerged {
				blocked = " [ブロック中]"
			}
			prInfo := ""
			if l.PRUrl != "" {
				prParts := []string{l.PRUrl}
				if l.PRState != "" {
					prParts = append(prParts, "state:"+l.PRState)
				}
				if l.PRIsDraft {
					prParts = append(prParts, "draft:true")
				}
				if l.PRReviewDecision != "" {
					prParts = append(prParts, "review:"+l.PRReviewDecision)
				}
				if l.PRCheckStatus != "" {
					prParts = append(prParts, "ci:"+l.PRCheckStatus)
				}
				if l.PRTitle != "" {
					prParts = append(prParts, l.PRTitle)
				}
				prInfo = fmt.Sprintf(" (PR: %s)", strings.Join(prParts, ", "))
			}
			b.WriteString(fmt.Sprintf("  %d. %s: %s%s%s\n", i+1, l.LayerName, l.Status, blocked, prInfo))
		}
	}
	return b.String()
}

func buildSuggestSystemPrompt(taskSummary, storySummary, relatedContext, globalSummary string) string {
	var b strings.Builder
	b.WriteString(`あなたは「maidirector」、開発者の作業を支援するAIアシスタントです。
以下の情報を基に、開発者が次に取り組むべきタスクを優先度付きで提案してください。

## 判断基準
1. ブロッカーの解消が最優先
2. ストーリーのパイプラインで上流PRがマージ済みなのに下流が未着手の場合、着手を提案
3. PRのレビューステータスがAPPROVEDで未マージの場合、マージを促す
4. PRがCHANGES_REQUESTEDの場合、修正対応を促す
5. PRのCIステータスがFAILUREの場合、CI修正を促す
6. PRのCIステータスがPENDINGの場合、CI完了待ちであることを通知
7. 進行中のタスクの完了を優先
8. 優先度の高いタスクを優先
9. 長時間放置されているタスクに注意を促す

## 現在のタスク一覧
`)
	b.WriteString(taskSummary)

	if storySummary != "" {
		b.WriteString("\n## ストーリー（施策）のパイプライン状況\n")
		b.WriteString(storySummary)
	}

	if relatedContext != "" {
		b.WriteString("\n## 関連する過去の文脈\n")
		b.WriteString(relatedContext)
	}

	if globalSummary != "" {
		b.WriteString("\n## 全体の状況サマリー\n")
		b.WriteString(globalSummary)
	}

	b.WriteString("\n提案は日本語で、具体的なアクション付きでお願いします。")
	return b.String()
}
