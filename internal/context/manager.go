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
	stories, _ := m.store.ListStories("")
	storySummary := formatStories(stories)

	var relatedContext string
	activeStories := filterActiveStories(stories)
	if len(activeStories) > 0 {
		query := buildRecentActivityQuery(activeStories)
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

	systemPrompt := buildSuggestSystemPrompt(storySummary, relatedContext, summaryText)

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

	stories, _ := m.store.ListStories("")
	storySummary := formatStories(stories)

	systemPrompt := fmt.Sprintf(`あなたは「maidirector」、開発者の作業を支援するAIアシスタントです。
タスクの状況を把握し、的確なアドバイスを提供してください。

## 現在のストーリー一覧
%s

## 関連する過去の文脈
%s

回答は簡潔かつ具体的にお願いします。`, storySummary, relatedContext)

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

func filterActiveStories(stories []*task.Story) []*task.Story {
	var active []*task.Story
	for _, s := range stories {
		if s.Status == task.StoryStatusActive {
			active = append(active, s)
		}
	}
	return active
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

func buildRecentActivityQuery(stories []*task.Story) string {
	var titles []string
	for i, s := range stories {
		if i >= 5 {
			break
		}
		titles = append(titles, s.Title)
	}
	return strings.Join(titles, " ")
}

func formatStories(stories []*task.Story) string {
	if len(stories) == 0 {
		return "（ストーリーなし）"
	}

	var b strings.Builder
	for _, s := range stories {
		priorityStr := ""
		if s.Priority > 0 {
			priorityStr = fmt.Sprintf(" P%d", s.Priority)
		}
		projectStr := ""
		if s.Project != "" {
			projectStr = fmt.Sprintf(" (project: %s)", s.Project)
		}
		statusStr := string(s.Status)
		b.WriteString(fmt.Sprintf("### [%s]%s%s %s", statusStr, priorityStr, projectStr, s.Title))
		if s.LayerChain != "" {
			b.WriteString(fmt.Sprintf(" (chain: %s)", s.LayerChain))
		}
		b.WriteString("\n")
		if s.Description != "" {
			b.WriteString(fmt.Sprintf("  %s\n", s.Description))
		}
		if len(s.Tags) > 0 {
			b.WriteString(fmt.Sprintf("  tags: %s\n", strings.Join(s.Tags, ", ")))
		}
		if len(s.Tasks) == 0 {
			b.WriteString("  タスク: なし（単タスク）\n")
			continue
		}
		taskByID := map[string]*task.StoryTask{}
		for _, t := range s.Tasks {
			taskByID[t.ID] = t
		}
		for i, t := range s.Tasks {
			blocked := ""
			for _, depID := range t.DependsOn {
				if dep, ok := taskByID[depID]; ok && dep.Status != task.TaskStatusMerged {
					blocked = " [ブロック中]"
					break
				}
			}
			prInfo := ""
			if t.PRUrl != "" {
				prParts := []string{t.PRUrl}
				if t.PRState != "" {
					prParts = append(prParts, "state:"+t.PRState)
				}
				if t.PRIsDraft {
					prParts = append(prParts, "draft:true")
				}
				if t.PRReviewDecision != "" {
					prParts = append(prParts, "review:"+t.PRReviewDecision)
				}
				if t.PRCheckStatus != "" {
					prParts = append(prParts, "ci:"+t.PRCheckStatus)
				}
				if t.PRTitle != "" {
					prParts = append(prParts, t.PRTitle)
				}
				prInfo = fmt.Sprintf(" (PR: %s)", strings.Join(prParts, ", "))
			}
			b.WriteString(fmt.Sprintf("  %d. %s: %s%s%s\n", i+1, t.Name, t.Status, blocked, prInfo))
		}
	}
	return b.String()
}

func buildSuggestSystemPrompt(storySummary, relatedContext, globalSummary string) string {
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
8. 優先度の高いタスク（P値が小さい）を優先
9. 長時間放置されているタスクに注意を促す

## 現在のストーリー一覧（単タスクを含む）
`)
	b.WriteString(storySummary)

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
