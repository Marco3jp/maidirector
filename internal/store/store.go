package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Marco3jp/maidirector/internal/task"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT DEFAULT 'todo',
			priority INTEGER DEFAULT 3,
			project TEXT DEFAULT '',
			links TEXT DEFAULT '[]',
			tags TEXT DEFAULT '[]',
			blocked_by TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_active_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS context_entries (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			content TEXT NOT NULL,
			entry_type TEXT DEFAULT 'note',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS summaries (
			id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project)`,
		`CREATE INDEX IF NOT EXISTS idx_context_entries_task_id ON context_entries(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_created_at ON conversations(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_summaries_scope ON summaries(scope)`,
		`CREATE TABLE IF NOT EXISTS stories (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			layer_chain TEXT DEFAULT 'default',
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS story_layers (
			id TEXT PRIMARY KEY,
			story_id TEXT NOT NULL,
			layer_name TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			pr_url TEXT DEFAULT '',
			pr_number INTEGER DEFAULT 0,
			pr_state TEXT DEFAULT '',
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (story_id) REFERENCES stories(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(status)`,
		`CREATE INDEX IF NOT EXISTS idx_story_layers_story_id ON story_layers(story_id)`,
		`ALTER TABLE story_layers ADD COLUMN pr_title TEXT DEFAULT ''`,
		`ALTER TABLE story_layers ADD COLUMN pr_review_decision TEXT DEFAULT ''`,
		`ALTER TABLE story_layers ADD COLUMN pr_is_draft INTEGER DEFAULT 0`,
		`ALTER TABLE story_layers ADD COLUMN pr_check_status TEXT DEFAULT ''`,
		// Phase 2.5: Story にフィールド追加
		`ALTER TABLE stories ADD COLUMN priority INTEGER DEFAULT 0`,
		`ALTER TABLE stories ADD COLUMN project TEXT DEFAULT ''`,
		`ALTER TABLE stories ADD COLUMN links TEXT DEFAULT '[]'`,
		`ALTER TABLE stories ADD COLUMN tags TEXT DEFAULT '[]'`,
		`ALTER TABLE stories ADD COLUMN last_active_at DATETIME`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			if strings.Contains(m, "ALTER TABLE") && strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	if err := s.migrateTasksToStories(); err != nil {
		return fmt.Errorf("task migration failed: %w", err)
	}

	return nil
}

func (s *Store) migrateTasksToStories() error {
	var tableExists int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tasks'`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		return nil
	}

	var taskCount int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&taskCount)
	if err != nil || taskCount == 0 {
		s.migrateContextEntryColumn()
		return nil
	}

	rows, err := s.db.Query(`SELECT id, title, description, status, priority, project, links, tags, created_at, updated_at, last_active_at FROM tasks`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for rows.Next() {
		var id, title, description, status, project, linksJSON, tagsJSON string
		var priority int
		var createdAt, updatedAt, lastActiveAt time.Time

		if err := rows.Scan(&id, &title, &description, &status, &priority, &project, &linksJSON, &tagsJSON, &createdAt, &updatedAt, &lastActiveAt); err != nil {
			return err
		}

		storyStatus := "active"
		if status == "done" {
			storyStatus = "completed"
		}

		_, err := tx.Exec(
			`INSERT OR IGNORE INTO stories (id, title, description, layer_chain, status, priority, project, links, tags, last_active_at, created_at, updated_at)
			 VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, title, description, storyStatus, priority, project, linksJSON, tagsJSON, lastActiveAt, createdAt, updatedAt,
		)
		if err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.migrateContextEntryColumn()

	s.db.Exec(`DROP TABLE IF EXISTS tasks`)

	return nil
}

func (s *Store) migrateContextEntryColumn() {
	var hasStoryID bool
	colRows, err := s.db.Query(`PRAGMA table_info(context_entries)`)
	if err != nil {
		return
	}
	defer colRows.Close()
	for colRows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		colRows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk)
		if name == "story_id" {
			hasStoryID = true
		}
	}

	if !hasStoryID {
		s.db.Exec(`ALTER TABLE context_entries RENAME COLUMN task_id TO story_id`)
	}
}

// --- Context Entries ---

func (s *Store) CreateContextEntry(e *task.ContextEntry) error {
	e.ID = uuid.New().String()
	e.CreatedAt = time.Now()

	_, err := s.db.Exec(
		`INSERT INTO context_entries (story_id, id, content, entry_type, created_at) VALUES (?, ?, ?, ?, ?)`,
		e.StoryID, e.ID, e.Content, e.EntryType, e.CreatedAt,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET last_active_at = ? WHERE id = ?`, time.Now(), e.StoryID)
	return err
}

func (s *Store) ListContextEntries(storyID string) ([]*task.ContextEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, story_id, content, entry_type, created_at FROM context_entries WHERE story_id = ? ORDER BY created_at DESC`,
		storyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*task.ContextEntry
	for rows.Next() {
		e := &task.ContextEntry{}
		if err := rows.Scan(&e.ID, &e.StoryID, &e.Content, &e.EntryType, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- Conversations ---

func (s *Store) CreateConversation(c *task.Conversation) error {
	c.ID = uuid.New().String()
	c.CreatedAt = time.Now()

	_, err := s.db.Exec(
		`INSERT INTO conversations (id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		c.ID, c.Role, c.Content, c.CreatedAt,
	)
	return err
}

func (s *Store) ListRecentConversations(limit int) ([]*task.Conversation, error) {
	rows, err := s.db.Query(
		`SELECT id, role, content, created_at FROM conversations ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []*task.Conversation
	for rows.Next() {
		c := &task.Conversation{}
		if err := rows.Scan(&c.ID, &c.Role, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

// --- Summaries ---

func (s *Store) CreateSummary(sm *task.Summary) error {
	sm.ID = uuid.New().String()
	sm.CreatedAt = time.Now()

	_, err := s.db.Exec(
		`INSERT INTO summaries (id, scope, content, created_at) VALUES (?, ?, ?, ?)`,
		sm.ID, sm.Scope, sm.Content, sm.CreatedAt,
	)
	return err
}

func (s *Store) GetLatestSummary(scope string) (*task.Summary, error) {
	row := s.db.QueryRow(
		`SELECT id, scope, content, created_at FROM summaries WHERE scope = ? ORDER BY created_at DESC LIMIT 1`,
		scope,
	)
	sm := &task.Summary{}
	err := row.Scan(&sm.ID, &sm.Scope, &sm.Content, &sm.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sm, nil
}

// --- Stories ---

func (s *Store) CreateStory(st *task.Story) error {
	st.ID = uuid.New().String()
	now := time.Now()
	st.CreatedAt = now
	st.UpdatedAt = now
	st.LastActiveAt = now

	if st.Status == "" {
		st.Status = task.StoryStatusActive
	}
	if st.Links == nil {
		st.Links = []string{}
	}
	if st.Tags == nil {
		st.Tags = []string{}
	}

	links, _ := json.Marshal(st.Links)
	tags, _ := json.Marshal(st.Tags)

	_, err := s.db.Exec(
		`INSERT INTO stories (id, title, description, layer_chain, status, priority, project, links, tags, last_active_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ID, st.Title, st.Description, st.LayerChain, st.Status, st.Priority, st.Project,
		string(links), string(tags), st.LastActiveAt, st.CreatedAt, st.UpdatedAt,
	)
	return err
}

func (s *Store) GetStory(id string) (*task.Story, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, layer_chain, status, priority, project, links, tags, last_active_at, created_at, updated_at FROM stories WHERE id = ?`, id,
	)
	st, err := scanStoryRow(row)
	if err != nil {
		return nil, err
	}

	layers, err := s.ListStoryLayers(st.ID)
	if err != nil {
		return nil, err
	}
	st.Layers = layers
	return st, nil
}

func (s *Store) ListStories(status string) ([]*task.Story, error) {
	query := `SELECT id, title, description, layer_chain, status, priority, project, links, tags, last_active_at, created_at, updated_at FROM stories WHERE 1=1`
	args := []any{}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY priority ASC, updated_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []*task.Story
	for rows.Next() {
		st, err := scanStoryRow(rows)
		if err != nil {
			return nil, err
		}
		layers, err := s.ListStoryLayers(st.ID)
		if err != nil {
			return nil, err
		}
		st.Layers = layers
		stories = append(stories, st)
	}
	return stories, rows.Err()
}

func (s *Store) UpdateStory(st *task.Story) error {
	st.UpdatedAt = time.Now()
	st.LastActiveAt = st.UpdatedAt

	links, _ := json.Marshal(st.Links)
	tags, _ := json.Marshal(st.Tags)

	_, err := s.db.Exec(
		`UPDATE stories SET title=?, description=?, layer_chain=?, status=?, priority=?, project=?, links=?, tags=?, last_active_at=?, updated_at=? WHERE id=?`,
		st.Title, st.Description, st.LayerChain, st.Status, st.Priority, st.Project,
		string(links), string(tags), st.LastActiveAt, st.UpdatedAt, st.ID,
	)
	return err
}

func (s *Store) DeleteStory(id string) error {
	_, err := s.db.Exec(`DELETE FROM stories WHERE id = ?`, id)
	return err
}

// --- Story Layers ---

func (s *Store) CreateStoryLayer(l *task.StoryLayer) error {
	l.ID = uuid.New().String()
	now := time.Now()
	l.CreatedAt = now
	l.UpdatedAt = now

	if l.Status == "" {
		l.Status = task.LayerStatusPending
	}

	if l.SortOrder == 0 {
		var maxOrder int
		s.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM story_layers WHERE story_id = ?`, l.StoryID).Scan(&maxOrder)
		l.SortOrder = maxOrder + 1
	}

	_, err := s.db.Exec(
		`INSERT INTO story_layers (id, story_id, layer_name, status, pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.StoryID, l.LayerName, l.Status, l.PRUrl, l.PRNumber, l.PRState, l.PRTitle, l.PRReviewDecision, boolToInt(l.PRIsDraft), l.PRCheckStatus, l.SortOrder, l.CreatedAt, l.UpdatedAt,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET updated_at = ? WHERE id = ?`, now, l.StoryID)
	return err
}

func (s *Store) ListStoryLayers(storyID string) ([]*task.StoryLayer, error) {
	rows, err := s.db.Query(
		`SELECT id, story_id, layer_name, status, pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at
		 FROM story_layers WHERE story_id = ? ORDER BY sort_order ASC`, storyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var layers []*task.StoryLayer
	for rows.Next() {
		l, err := scanStoryLayer(rows)
		if err != nil {
			return nil, err
		}
		layers = append(layers, l)
	}
	if layers == nil {
		layers = []*task.StoryLayer{}
	}
	return layers, rows.Err()
}

func (s *Store) UpdateStoryLayer(l *task.StoryLayer) error {
	l.UpdatedAt = time.Now()
	_, err := s.db.Exec(
		`UPDATE story_layers SET layer_name=?, status=?, pr_url=?, pr_number=?, pr_state=?, pr_title=?, pr_review_decision=?, pr_is_draft=?, pr_check_status=?, sort_order=?, updated_at=? WHERE id=?`,
		l.LayerName, l.Status, l.PRUrl, l.PRNumber, l.PRState, l.PRTitle, l.PRReviewDecision, boolToInt(l.PRIsDraft), l.PRCheckStatus, l.SortOrder, l.UpdatedAt, l.ID,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET updated_at = ? WHERE id = ?`, l.UpdatedAt, l.StoryID)
	return err
}

func (s *Store) DeleteStoryLayer(id string) error {
	var storyID string
	if err := s.db.QueryRow(`SELECT story_id FROM story_layers WHERE id = ?`, id).Scan(&storyID); err != nil {
		return err
	}

	_, err := s.db.Exec(`DELETE FROM story_layers WHERE id = ?`, id)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET updated_at = ? WHERE id = ?`, time.Now(), storyID)
	return err
}

func (s *Store) GetStoryLayer(id string) (*task.StoryLayer, error) {
	row := s.db.QueryRow(
		`SELECT id, story_id, layer_name, status, pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at
		 FROM story_layers WHERE id = ?`, id,
	)
	return scanStoryLayer(row)
}

// --- scan helpers ---

func scanStoryLayer(sc scannable) (*task.StoryLayer, error) {
	l := &task.StoryLayer{}
	var isDraft int
	if err := sc.Scan(&l.ID, &l.StoryID, &l.LayerName, &l.Status, &l.PRUrl, &l.PRNumber, &l.PRState, &l.PRTitle, &l.PRReviewDecision, &isDraft, &l.PRCheckStatus, &l.SortOrder, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return nil, err
	}
	l.PRIsDraft = isDraft != 0
	return l, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type scannable interface {
	Scan(dest ...any) error
}

func scanStoryRow(sc scannable) (*task.Story, error) {
	st := &task.Story{}
	var linksJSON, tagsJSON string
	var lastActiveAt sql.NullTime

	err := sc.Scan(
		&st.ID, &st.Title, &st.Description, &st.LayerChain, &st.Status,
		&st.Priority, &st.Project, &linksJSON, &tagsJSON,
		&lastActiveAt, &st.CreatedAt, &st.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if lastActiveAt.Valid {
		st.LastActiveAt = lastActiveAt.Time
	} else {
		st.LastActiveAt = st.UpdatedAt
	}

	json.Unmarshal([]byte(linksJSON), &st.Links)
	json.Unmarshal([]byte(tagsJSON), &st.Tags)

	if st.Links == nil {
		st.Links = []string{}
	}
	if st.Tags == nil {
		st.Tags = []string{}
	}

	return st, nil
}
