package api

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Marco3jp/maidirector/internal/config"
	"github.com/Marco3jp/maidirector/internal/context"
	"github.com/Marco3jp/maidirector/internal/github"
	"github.com/Marco3jp/maidirector/internal/llm"
	"github.com/Marco3jp/maidirector/internal/task"
	"github.com/Marco3jp/maidirector/internal/vectorstore"
	"github.com/Marco3jp/maidirector/internal/ws"
)

type Server struct {
	cfg         *config.Config
	storySvc    *task.StoryService
	ctxMgr      *context.Manager
	llmClient   llm.Provider
	vectorStore *vectorstore.VectorStore
	ghClient    *github.Client
	wsHub       *ws.Hub
	httpServer  *http.Server
}

func NewServer(cfg *config.Config, storySvc *task.StoryService, ctxMgr *context.Manager, llmClient llm.Provider, vs *vectorstore.VectorStore, ghClient *github.Client, staticFS ...fs.FS) *Server {
	s := &Server{
		cfg:         cfg,
		storySvc:    storySvc,
		ctxMgr:      ctxMgr,
		llmClient:   llmClient,
		vectorStore: vs,
		ghClient:    ghClient,
		wsHub:       ws.NewHub(),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/stories", s.handleListStories)
	mux.HandleFunc("POST /api/stories", s.handleCreateStory)
	mux.HandleFunc("GET /api/stories/{id}", s.handleGetStory)
	mux.HandleFunc("PUT /api/stories/{id}", s.handleUpdateStory)
	mux.HandleFunc("DELETE /api/stories/{id}", s.handleDeleteStory)
	mux.HandleFunc("POST /api/stories/{id}/layers", s.handleAddLayer)
	mux.HandleFunc("PUT /api/stories/{storyId}/layers/{layerId}", s.handleUpdateLayer)
	mux.HandleFunc("DELETE /api/stories/{storyId}/layers/{layerId}", s.handleDeleteLayer)
	mux.HandleFunc("POST /api/stories/{storyId}/layers/{layerId}/refresh-pr", s.handleRefreshLayerPR)
	mux.HandleFunc("POST /api/stories/refresh-prs", s.handleRefreshAllPRs)
	mux.HandleFunc("POST /api/stories/scan-org-prs", s.handleScanOrgPRs)
	mux.HandleFunc("POST /api/stories/{id}/context", s.handleAddContext)
	mux.HandleFunc("GET /api/stories/{id}/context", s.handleListContext)
	mux.HandleFunc("GET /api/layer-chains", s.handleGetLayerChains)

	mux.HandleFunc("POST /api/suggest", s.handleSuggest)
	mux.HandleFunc("POST /api/chat", s.handleChat)

	mux.HandleFunc("GET /ws", s.wsHub.HandleWS)

	if len(staticFS) > 0 && staticFS[0] != nil {
		mux.Handle("/", newSPAHandler(staticFS[0]))
	} else {
		mux.Handle("/", http.FileServer(http.Dir("web/dist")))
	}

	handler := corsMiddleware(mux)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: handler,
	}

	return s
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// --- Story handlers ---

func (s *Server) handleListStories(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	stories, err := s.storySvc.List(status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stories == nil {
		stories = []*task.Story{}
	}
	writeJSON(w, http.StatusOK, stories)
}

func (s *Server) handleCreateStory(w http.ResponseWriter, r *http.Request) {
	var st task.Story
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if st.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if err := s.storySvc.Create(&st); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	st.Layers = []*task.StoryLayer{}

	go s.vectorizeStory(&st)

	s.wsHub.Broadcast(ws.Message{Type: "story_created", Payload: st})
	writeJSON(w, http.StatusCreated, st)
}

func (s *Server) handleGetStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := s.storySvc.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "story not found")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdateStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.storySvc.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "story not found")
		return
	}

	var updates task.Story
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing.Title = updates.Title
	existing.Description = updates.Description
	existing.LayerChain = updates.LayerChain
	existing.Status = updates.Status
	existing.Priority = updates.Priority
	existing.Project = updates.Project
	existing.Links = updates.Links
	existing.Tags = updates.Tags

	if err := s.storySvc.Update(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.vectorizeStory(existing)

	s.wsHub.Broadcast(ws.Message{Type: "story_updated", Payload: existing})
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.storySvc.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.deleteStoryVectors(id)

	s.wsHub.Broadcast(ws.Message{Type: "story_deleted", Payload: map[string]string{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddLayer(w http.ResponseWriter, r *http.Request) {
	storyID := r.PathValue("id")
	if _, err := s.storySvc.Get(storyID); err != nil {
		writeError(w, http.StatusNotFound, "story not found")
		return
	}

	var layer task.StoryLayer
	if err := json.NewDecoder(r.Body).Decode(&layer); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if layer.LayerName == "" {
		writeError(w, http.StatusBadRequest, "layer_name is required")
		return
	}

	layer.StoryID = storyID
	if err := s.storySvc.AddLayer(&layer); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(ws.Message{Type: "story_updated", Payload: map[string]string{"story_id": storyID}})
	writeJSON(w, http.StatusCreated, layer)
}

func (s *Server) handleUpdateLayer(w http.ResponseWriter, r *http.Request) {
	layerID := r.PathValue("layerId")
	existing, err := s.storySvc.GetLayer(layerID)
	if err != nil {
		writeError(w, http.StatusNotFound, "layer not found")
		return
	}

	var updates task.StoryLayer
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	oldPRUrl := existing.PRUrl

	existing.LayerName = updates.LayerName
	existing.Status = updates.Status
	existing.PRUrl = updates.PRUrl
	existing.PRNumber = updates.PRNumber
	existing.PRState = updates.PRState
	existing.PRTitle = updates.PRTitle
	existing.PRReviewDecision = updates.PRReviewDecision
	if updates.SortOrder != 0 {
		existing.SortOrder = updates.SortOrder
	}

	if existing.PRUrl != "" && existing.PRUrl != oldPRUrl && s.ghClient != nil {
		s.applyPRInfo(existing)
	}

	if err := s.storySvc.UpdateLayer(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(ws.Message{Type: "story_updated", Payload: map[string]string{"story_id": existing.StoryID}})
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteLayer(w http.ResponseWriter, r *http.Request) {
	storyID := r.PathValue("storyId")
	layerID := r.PathValue("layerId")

	if err := s.storySvc.DeleteLayer(layerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(ws.Message{Type: "story_updated", Payload: map[string]string{"story_id": storyID}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetLayerChains(w http.ResponseWriter, _ *http.Request) {
	chains := s.cfg.LayerChains
	if chains == nil {
		chains = []config.LayerChainConfig{}
	}
	writeJSON(w, http.StatusOK, chains)
}

// --- Context handlers ---

func (s *Server) handleAddContext(w http.ResponseWriter, r *http.Request) {
	storyID := r.PathValue("id")

	var entry task.ContextEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entry.StoryID = storyID

	if err := s.storySvc.AddContext(&entry); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.vectorizeContextEntry(&entry)

	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleListContext(w http.ResponseWriter, r *http.Request) {
	storyID := r.PathValue("id")

	entries, err := s.storySvc.ListContext(storyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if entries == nil {
		entries = []*task.ContextEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

// --- PR refresh ---

func (s *Server) handleRefreshLayerPR(w http.ResponseWriter, r *http.Request) {
	layerID := r.PathValue("layerId")
	existing, err := s.storySvc.GetLayer(layerID)
	if err != nil {
		writeError(w, http.StatusNotFound, "layer not found")
		return
	}

	if existing.PRUrl == "" {
		writeError(w, http.StatusBadRequest, "layer has no PR URL")
		return
	}

	if s.ghClient == nil {
		writeError(w, http.StatusServiceUnavailable, "GitHub CLI not available")
		return
	}

	s.applyPRInfo(existing)

	if err := s.storySvc.UpdateLayer(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.wsHub.Broadcast(ws.Message{Type: "story_updated", Payload: map[string]string{"story_id": existing.StoryID}})
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) applyPRInfo(layer *task.StoryLayer) {
	if s.ghClient == nil || layer.PRUrl == "" {
		return
	}
	info, err := s.ghClient.GetPRInfo(layer.PRUrl)
	if err != nil {
		log.Printf("warning: failed to fetch PR info for %s: %v", layer.PRUrl, err)
		return
	}
	layer.PRNumber = info.Number
	layer.PRState = info.State
	layer.PRTitle = info.Title
	layer.PRReviewDecision = info.ReviewDecision
	layer.PRIsDraft = info.IsDraft
	layer.PRCheckStatus = info.CheckStatus
	if info.State == "MERGED" && layer.Status != task.LayerStatusMerged {
		layer.Status = task.LayerStatusMerged
	}
}

func (s *Server) refreshPRStates() {
	if s.ghClient == nil {
		return
	}
	stories, err := s.storySvc.List("active")
	if err != nil {
		log.Printf("warning: failed to list stories for PR refresh: %v", err)
		return
	}
	for _, story := range stories {
		for _, layer := range story.Layers {
			if layer.PRUrl == "" || layer.Status == task.LayerStatusMerged {
				continue
			}
			s.applyPRInfo(layer)
			if err := s.storySvc.UpdateLayer(layer); err != nil {
				log.Printf("warning: failed to update layer after PR refresh: %v", err)
			}
		}
	}
}

func (s *Server) handleRefreshAllPRs(w http.ResponseWriter, r *http.Request) {
	s.refreshPRStates()
	stories, err := s.storySvc.List("active")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stories)
}

// --- Org PR Scan ---

const settingLastScanTime = "last_org_pr_scan_time"

func (s *Server) handleScanOrgPRs(w http.ResponseWriter, r *http.Request) {
	org := s.cfg.GitHub.Org
	if org == "" {
		writeError(w, http.StatusBadRequest, "github.org is not configured")
		return
	}
	if s.ghClient == nil {
		writeError(w, http.StatusServiceUnavailable, "GitHub CLI not available")
		return
	}

	since := time.Now().AddDate(0, 0, -7)

	lastScan, err := s.storySvc.GetSetting(settingLastScanTime)
	if err != nil {
		log.Printf("warning: failed to get last scan time: %v", err)
	}
	if lastScan != "" {
		if t, err := time.Parse(time.RFC3339, lastScan); err == nil {
			if t.After(since) {
				since = t
			}
		}
	}

	prs, err := s.ghClient.SearchOrgPRs(org, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to search PRs: %v", err))
		return
	}

	excludeSet := make(map[string]bool)
	for _, author := range s.cfg.GitHub.ExcludeAuthors {
		excludeSet[strings.ToLower(author)] = true
	}

	botPolicy := s.cfg.GitHub.BotPolicy
	if botPolicy == "" {
		botPolicy = "exclude"
	}
	botPriority := 0
	botInclude := false
	if botPolicy != "exclude" {
		if p, err := strconv.Atoi(botPolicy); err == nil {
			botInclude = true
			botPriority = p
		}
	}

	created := 0
	skipped := 0

	for _, pr := range prs {
		if excludeSet[strings.ToLower(pr.Author)] {
			skipped++
			continue
		}

		isBot := github.IsBotAuthor(pr.Author)
		if isBot && !botInclude {
			skipped++
			continue
		}

		existingID, err := s.storySvc.FindByPRURL(pr.URL)
		if err != nil {
			log.Printf("warning: failed to check existing story for PR %s: %v", pr.URL, err)
			continue
		}
		if existingID != "" {
			skipped++
			continue
		}

		title := fmt.Sprintf("[Review] %s #%d: %s", pr.Repo, pr.Number, pr.Title)
		st := &task.Story{
			Title:  title,
			Status: task.StoryStatusActive,
			Links:  []string{pr.URL},
			Tags:   []string{"review"},
		}
		if isBot {
			st.Priority = botPriority
			st.Tags = []string{"review", "bot"}
		}
		if err := s.storySvc.Create(st); err != nil {
			log.Printf("warning: failed to create review story for PR %s: %v", pr.URL, err)
			continue
		}
		st.Layers = []*task.StoryLayer{}

		go s.vectorizeStory(st)
		s.wsHub.Broadcast(ws.Message{Type: "story_created", Payload: st})
		created++
	}

	if err := s.storySvc.SetSetting(settingLastScanTime, time.Now().Format(time.RFC3339)); err != nil {
		log.Printf("warning: failed to save last scan time: %v", err)
	}

	result := map[string]any{
		"scanned": len(prs),
		"created": created,
		"skipped": skipped,
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Suggest / Chat handlers ---

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	s.refreshPRStates()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	result, err := s.ctxMgr.SuggestStream(r.Context(), func(chunk string) {
		data, _ := json.Marshal(map[string]string{"chunk": chunk})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		log.Printf("suggest error: %v", err)
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	data, _ := json.Marshal(map[string]string{"done": result.Suggestion})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req context.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.refreshPRStates()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	result, err := s.ctxMgr.ChatStream(r.Context(), req.Message, func(chunk string) {
		data, _ := json.Marshal(map[string]string{"chunk": chunk})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		log.Printf("chat error: %v", err)
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	data, _ := json.Marshal(map[string]string{"done": result.Reply})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// --- vectorization ---

func (s *Server) vectorizeStory(st *task.Story) {
	content := st.Title
	if st.Description != "" {
		content += "\n" + st.Description
	}
	if st.Project != "" {
		content = "[" + st.Project + "] " + content
	}

	if err := s.vectorStore.Upsert(gocontext.Background(), vectorstore.PointPayload{
		SourceType: "story",
		SourceID:   st.ID,
		Project:    st.Project,
		Content:    content,
		CreatedAt:  st.CreatedAt.Format(time.RFC3339),
	}); err != nil {
		log.Printf("warning: failed to vectorize story %s: %v", st.ID, err)
	}
}

func (s *Server) vectorizeContextEntry(e *task.ContextEntry) {
	content := fmt.Sprintf("[%s] %s", e.EntryType, e.Content)

	if err := s.vectorStore.Upsert(gocontext.Background(), vectorstore.PointPayload{
		SourceType: "context_entry",
		SourceID:   e.ID,
		StoryID:    e.StoryID,
		Content:    content,
		CreatedAt:  e.CreatedAt.Format(time.RFC3339),
	}); err != nil {
		log.Printf("warning: failed to vectorize context entry %s: %v", e.ID, err)
	}
}

func (s *Server) deleteStoryVectors(storyID string) {
	if err := s.vectorStore.DeletePoints([]string{storyID}); err != nil {
		log.Printf("warning: failed to delete story vector %s: %v", storyID, err)
	}

	filter := map[string]any{
		"must": []map[string]any{
			{"key": "story_id", "match": map[string]any{"value": storyID}},
		},
	}
	if err := s.vectorStore.DeleteByFilter(filter); err != nil {
		log.Printf("warning: failed to delete context vectors for story %s: %v", storyID, err)
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
