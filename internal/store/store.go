package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}

// --- Tasks ---

func (s *Store) CreateTask(t *task.Task) error {
	t.ID = uuid.New().String()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	t.LastActiveAt = now

	if t.Status == "" {
		t.Status = task.StatusTodo
	}
	if t.Priority == 0 {
		t.Priority = 3
	}

	links, _ := json.Marshal(t.Links)
	tags, _ := json.Marshal(t.Tags)
	blockedBy, _ := json.Marshal(t.BlockedBy)

	_, err := s.db.Exec(
		`INSERT INTO tasks (id, title, description, status, priority, project, links, tags, blocked_by, created_at, updated_at, last_active_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Status, t.Priority, t.Project,
		string(links), string(tags), string(blockedBy),
		t.CreatedAt, t.UpdatedAt, t.LastActiveAt,
	)
	return err
}

func (s *Store) GetTask(id string) (*task.Task, error) {
	row := s.db.QueryRow(`SELECT id, title, description, status, priority, project, links, tags, blocked_by, created_at, updated_at, last_active_at FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (s *Store) ListTasks(status string, project string) ([]*task.Task, error) {
	query := `SELECT id, title, description, status, priority, project, links, tags, blocked_by, created_at, updated_at, last_active_at FROM tasks WHERE 1=1`
	args := []any{}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}

	query += ` ORDER BY priority ASC, updated_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*task.Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTask(t *task.Task) error {
	t.UpdatedAt = time.Now()
	t.LastActiveAt = t.UpdatedAt

	links, _ := json.Marshal(t.Links)
	tags, _ := json.Marshal(t.Tags)
	blockedBy, _ := json.Marshal(t.BlockedBy)

	_, err := s.db.Exec(
		`UPDATE tasks SET title=?, description=?, status=?, priority=?, project=?, links=?, tags=?, blocked_by=?, updated_at=?, last_active_at=? WHERE id=?`,
		t.Title, t.Description, t.Status, t.Priority, t.Project,
		string(links), string(tags), string(blockedBy),
		t.UpdatedAt, t.LastActiveAt, t.ID,
	)
	return err
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// --- Context Entries ---

func (s *Store) CreateContextEntry(e *task.ContextEntry) error {
	e.ID = uuid.New().String()
	e.CreatedAt = time.Now()

	_, err := s.db.Exec(
		`INSERT INTO context_entries (id, task_id, content, entry_type, created_at) VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.TaskID, e.Content, e.EntryType, e.CreatedAt,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE tasks SET last_active_at = ? WHERE id = ?`, time.Now(), e.TaskID)
	return err
}

func (s *Store) ListContextEntries(taskID string) ([]*task.ContextEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, content, entry_type, created_at FROM context_entries WHERE task_id = ? ORDER BY created_at DESC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*task.ContextEntry
	for rows.Next() {
		e := &task.ContextEntry{}
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Content, &e.EntryType, &e.CreatedAt); err != nil {
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

// --- scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanTaskFromRow(sc scannable) (*task.Task, error) {
	t := &task.Task{}
	var linksJSON, tagsJSON, blockedByJSON string

	err := sc.Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Project,
		&linksJSON, &tagsJSON, &blockedByJSON,
		&t.CreatedAt, &t.UpdatedAt, &t.LastActiveAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(linksJSON), &t.Links)
	json.Unmarshal([]byte(tagsJSON), &t.Tags)
	json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy)

	if t.Links == nil {
		t.Links = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	if t.BlockedBy == nil {
		t.BlockedBy = []string{}
	}

	return t, nil
}

func scanTask(row *sql.Row) (*task.Task, error) {
	return scanTaskFromRow(row)
}

func scanTaskRows(rows *sql.Rows) (*task.Task, error) {
	return scanTaskFromRow(rows)
}
