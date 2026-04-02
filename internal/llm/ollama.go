package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OllamaClient struct {
	endpoint       string
	chatModel      string
	embeddingModel string
	httpClient     *http.Client
}

func NewOllamaClient(endpoint, chatModel, embeddingModel string) *OllamaClient {
	return &OllamaClient{
		endpoint:       strings.TrimRight(endpoint, "/"),
		chatModel:      chatModel,
		embeddingModel: embeddingModel,
		httpClient:     &http.Client{},
	}
}

type ollamaChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ollamaChatResponse struct {
	Message      ChatMessage `json:"message"`
	Done         bool        `json:"done"`
	TotalDuration int64     `json:"total_duration"`
}

func (c *OllamaClient) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	reqBody := ollamaChatRequest{
		Model:    c.chatModel,
		Messages: messages,
		Stream:   false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	return &ChatResponse{Content: ollamaResp.Message.Content}, nil
}

func (c *OllamaClient) ChatStream(ctx context.Context, messages []ChatMessage, onChunk func(string)) (*ChatResponse, error) {
	reqBody := ollamaChatRequest{
		Model:    c.chatModel,
		Messages: messages,
		Stream:   true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama chat stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var fullContent strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk ollamaChatResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if chunk.Message.Content != "" {
			fullContent.WriteString(chunk.Message.Content)
			if onChunk != nil {
				onChunk(chunk.Message.Content)
			}
		}

		if chunk.Done {
			break
		}
	}

	return &ChatResponse{Content: fullContent.String()}, nil
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model: c.embeddingModel,
		Input: text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/embed", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	if len(ollamaResp.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama returned empty embeddings")
	}

	return ollamaResp.Embeddings[0], nil
}
