package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Marco3jp/maidirector/internal/llm"
	"github.com/google/uuid"
)

const vectorDimension = 768

type VectorStore struct {
	endpoint   string
	collection string
	llmClient  llm.Provider
	httpClient *http.Client
}

type PointPayload struct {
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	TaskID     string `json:"task_id,omitempty"`
	Project    string `json:"project,omitempty"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

type SearchResult struct {
	ID      string       `json:"id"`
	Score   float32      `json:"score"`
	Payload PointPayload `json:"payload"`
}

func New(endpoint, collection string, llmClient llm.Provider) (*VectorStore, error) {
	vs := &VectorStore{
		endpoint:   strings.TrimRight(endpoint, "/"),
		collection: collection,
		llmClient:  llmClient,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	if err := vs.ensureCollection(); err != nil {
		return nil, err
	}

	return vs, nil
}

func (vs *VectorStore) ensureCollection() error {
	resp, err := vs.httpClient.Get(vs.endpoint + "/collections/" + vs.collection)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	createBody := map[string]any{
		"vectors": map[string]any{
			"size":     vectorDimension,
			"distance": "Cosine",
		},
	}

	data, _ := json.Marshal(createBody)
	createResp, err := vs.httpClient.Do(vs.newRequest("PUT", "/collections/"+vs.collection, data))
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(createResp.Body)
		return fmt.Errorf("failed to create collection (status %d): %s", createResp.StatusCode, string(body))
	}

	return nil
}

func (vs *VectorStore) Upsert(ctx context.Context, payload PointPayload) error {
	embedding, err := vs.llmClient.Embed(ctx, payload.Content)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	pointID := payload.SourceID
	if pointID == "" {
		pointID = uuid.New().String()
	}

	body := map[string]any{
		"points": []map[string]any{
			{
				"id":      pointID,
				"vector":  embedding,
				"payload": payload,
			},
		},
	}

	data, _ := json.Marshal(body)
	resp, err := vs.httpClient.Do(vs.newRequest("PUT", "/collections/"+vs.collection+"/points", data))
	if err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (vs *VectorStore) Search(ctx context.Context, query string, limit int, filter map[string]any) ([]SearchResult, error) {
	embedding, err := vs.llmClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query failed: %w", err)
	}

	searchBody := map[string]any{
		"vector":       embedding,
		"limit":        limit,
		"with_payload": true,
	}

	if filter != nil {
		searchBody["filter"] = filter
	}

	data, _ := json.Marshal(searchBody)
	resp, err := vs.httpClient.Do(vs.newRequest("POST", "/collections/"+vs.collection+"/points/search", data))
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var searchResp struct {
		Result []struct {
			ID      string          `json:"id"`
			Score   float32         `json:"score"`
			Payload json.RawMessage `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(searchResp.Result))
	for _, r := range searchResp.Result {
		var payload PointPayload
		json.Unmarshal(r.Payload, &payload)
		results = append(results, SearchResult{
			ID:      r.ID,
			Score:   r.Score,
			Payload: payload,
		})
	}

	return results, nil
}

func (vs *VectorStore) newRequest(method, path string, body []byte) *http.Request {
	req, _ := http.NewRequest(method, vs.endpoint+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
