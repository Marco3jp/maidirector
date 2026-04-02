package llm

import "context"

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Content string `json:"content"`
}

type Provider interface {
	Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
	ChatStream(ctx context.Context, messages []ChatMessage, onChunk func(string)) (*ChatResponse, error)
	Embed(ctx context.Context, text string) ([]float32, error)
}
