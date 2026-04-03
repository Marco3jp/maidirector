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
		`ALTER TABLE stories ADD COLUMN tags TEXT DEFAULT '[]'`,
		`ALTER TABLE stories ADD COLUMN last_active_at DATETIME`,
		// Phase 2c: settings テーブル
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Phase 2c': スヌーズカラム
		`ALTER TABLE stories ADD COLUMN snoozed_until DATETIME`,
		// Phase 3: story_tasks テーブル（story_layers をリネーム、name カラムに変更）
		`CREATE TABLE IF NOT EXISTS story_tasks (
			id TEXT PRIMARY KEY,
			story_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			pr_url TEXT DEFAULT '',
			pr_number INTEGER DEFAULT 0,
			pr_state TEXT DEFAULT '',
			pr_title TEXT DEFAULT '',
			pr_review_decision TEXT DEFAULT '',
			pr_is_draft INTEGER DEFAULT 0,
			pr_check_status TEXT DEFAULT '',
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (story_id) REFERENCES stories(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_story_tasks_story_id ON story_tasks(story_id)`,
		// Phase 3: task_dependencies テーブル（任意依存関係の DAG）
		`CREATE TABLE IF NOT EXISTS task_dependencies (
			task_id TEXT NOT NULL,
			depends_on_id TEXT NOT NULL,
			PRIMARY KEY (task_id, depends_on_id),
			FOREIGN KEY (task_id) REFERENCES story_tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (depends_on_id) REFERENCES story_tasks(id) ON DELETE CASCADE
		)`,
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

	if err := s.migrateStoryLayersToTasks(); err != nil {
		return fmt.Errorf("story_layers migration failed: %w", err)
	}

	s.migrateSnoozedUntilToUTC()

	if err := s.migrateReviewStoryLinks(); err != nil {
		return fmt.Errorf("review story links migration failed: %w", err)
	}

	if _, err := s.db.Exec(`ALTER TABLE stories DROP COLUMN links`); err != nil {
		if !strings.Contains(err.Error(), "no such column") {
			return fmt.Errorf("failed to drop links column: %w", err)
		}
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

	rows, err := s.db.Query(`SELECT id, title, description, status, priority, project, tags, created_at, updated_at, last_active_at FROM tasks`)
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
		var id, title, description, status, project, tagsJSON string
		var priority int
		var createdAt, updatedAt, lastActiveAt time.Time

		if err := rows.Scan(&id, &title, &description, &status, &priority, &project, &tagsJSON, &createdAt, &updatedAt, &lastActiveAt); err != nil {
			return err
		}

		storyStatus := "active"
		if status == "done" {
			storyStatus = "completed"
		}

		_, err := tx.Exec(
			`INSERT OR IGNORE INTO stories (id, title, description, layer_chain, status, priority, project, tags, last_active_at, created_at, updated_at)
			 VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?)`,
			id, title, description, storyStatus, priority, project, tagsJSON, lastActiveAt, createdAt, updatedAt,
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

// migrateStoryLayersToTasks は story_layers のデータを story_tasks にコピーし、
// sort_order の順で隣接タスク間の依存関係を task_dependencies に登録する。
// story_tasks に既にデータがある場合はスキップする（冪等）。
func (s *Store) migrateStoryLayersToTasks() error {
	var layerCount int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM story_layers`).Scan(&layerCount)
	if err != nil || layerCount == 0 {
		return nil
	}

	var taskCount int
	s.db.QueryRow(`SELECT COUNT(*) FROM story_tasks`).Scan(&taskCount)
	if taskCount > 0 {
		return nil
	}

	rows, err := s.db.Query(`
		SELECT id, story_id, layer_name, status, pr_url, pr_number, pr_state,
		       pr_title, pr_review_decision, pr_is_draft, pr_check_status,
		       sort_order, created_at, updated_at
		FROM story_layers
		ORDER BY story_id, sort_order ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type layerRow struct {
		id               string
		storyID          string
		name             string
		status           string
		prURL            string
		prNumber         int
		prState          string
		prTitle          string
		prReviewDecision string
		prIsDraft        int
		prCheckStatus    string
		sortOrder        int
		createdAt        time.Time
		updatedAt        time.Time
	}

	var layers []layerRow
	for rows.Next() {
		var l layerRow
		if err := rows.Scan(
			&l.id, &l.storyID, &l.name, &l.status, &l.prURL, &l.prNumber,
			&l.prState, &l.prTitle, &l.prReviewDecision, &l.prIsDraft,
			&l.prCheckStatus, &l.sortOrder, &l.createdAt, &l.updatedAt,
		); err != nil {
			return err
		}
		layers = append(layers, l)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// story_id → sorted task IDs の対応を保持して deps を作る
	storyTasks := map[string][]string{}

	for _, l := range layers {
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO story_tasks
			  (id, story_id, name, status, pr_url, pr_number, pr_state,
			   pr_title, pr_review_decision, pr_is_draft, pr_check_status,
			   sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			l.id, l.storyID, l.name, l.status, l.prURL, l.prNumber, l.prState,
			l.prTitle, l.prReviewDecision, l.prIsDraft, l.prCheckStatus,
			l.sortOrder, l.createdAt, l.updatedAt,
		)
		if err != nil {
			return err
		}
		storyTasks[l.storyID] = append(storyTasks[l.storyID], l.id)
	}

	// 隣接タスク間に依存関係を作成（既存のチェーン動作を再現）
	for _, ids := range storyTasks {
		for i := 1; i < len(ids); i++ {
			if _, err := tx.Exec(`
				INSERT OR IGNORE INTO task_dependencies (task_id, depends_on_id)
				VALUES (?, ?)`, ids[i], ids[i-1],
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *Store) migrateSnoozedUntilToUTC() {
	rows, err := s.db.Query(`SELECT id, snoozed_until FROM stories WHERE snoozed_until IS NOT NULL`)
	if err != nil {
		return
	}
	defer rows.Close()

	type record struct {
		id    string
		until time.Time
	}
	var records []record
	for rows.Next() {
		var id string
		var until time.Time
		if err := rows.Scan(&id, &until); err != nil {
			continue
		}
		records = append(records, record{id, until})
	}
	rows.Close()

	for _, r := range records {
		utcStr := r.until.UTC().Format("2006-01-02 15:04:05")
		s.db.Exec(`UPDATE stories SET snoozed_until = ? WHERE id = ?`, utcStr, r.id)
	}
}

func (s *Store) migrateReviewStoryLinks() error {
	rows, err := s.db.Query(`
		SELECT id, links FROM stories
		WHERE tags LIKE '%"review"%'
		  AND NOT EXISTS (SELECT 1 FROM story_tasks WHERE story_id = stories.id)
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such column") {
			return nil
		}
		return err
	}
	defer rows.Close()

	type record struct {
		id    string
		links []string
	}
	var records []record
	for rows.Next() {
		var id, linksJSON string
		if err := rows.Scan(&id, &linksJSON); err != nil {
			return err
		}
		var links []string
		json.Unmarshal([]byte(linksJSON), &links)
		if len(links) > 0 {
			records = append(records, record{id, links})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for _, r := range records {
		prURL := r.links[0]
		if !strings.Contains(prURL, "github.com/") || !strings.Contains(prURL, "/pull/") {
			continue
		}
		taskID := uuid.New().String()
		now := time.Now()
		_, err := s.db.Exec(`
			INSERT INTO story_tasks (id, story_id, name, status, pr_url, pr_number, pr_state,
				pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at)
			VALUES (?, ?, 'レビュー', 'pending', ?, 0, '', '', '', 0, '', 0, ?, ?)
		`, taskID, r.id, prURL, now, now)
		if err != nil {
			return err
		}
	}
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

func (s *Store) DeleteContextEntry(id string) error {
	_, err := s.db.Exec(`DELETE FROM context_entries WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateContextEntry(e *task.ContextEntry) error {
	_, err := s.db.Exec(
		`UPDATE context_entries SET content = ?, entry_type = ? WHERE id = ?`,
		e.Content, e.EntryType, e.ID,
	)
	return err
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
	if st.Tags == nil {
		st.Tags = []string{}
	}

	tags, _ := json.Marshal(st.Tags)

	_, err := s.db.Exec(
		`INSERT INTO stories (id, title, description, layer_chain, status, priority, project, tags, last_active_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ID, st.Title, st.Description, st.LayerChain, st.Status, st.Priority, st.Project,
		string(tags), st.LastActiveAt, st.CreatedAt, st.UpdatedAt,
	)
	return err
}

func (s *Store) GetStory(id string) (*task.Story, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, layer_chain, status, priority, project, tags, snoozed_until, last_active_at, created_at, updated_at FROM stories WHERE id = ?`, id,
	)
	st, err := scanStoryRow(row)
	if err != nil {
		return nil, err
	}

	tasks, err := s.ListStoryTasks(st.ID)
	if err != nil {
		return nil, err
	}
	st.Tasks = tasks
	return st, nil
}

func (s *Store) ListStories(status string) ([]*task.Story, error) {
	query := `SELECT id, title, description, layer_chain, status, priority, project, tags, snoozed_until, last_active_at, created_at, updated_at FROM stories WHERE 1=1`
	args := []any{}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY CASE WHEN snoozed_until IS NOT NULL AND datetime(snoozed_until) > datetime('now') THEN 1 ELSE 0 END ASC, CASE WHEN priority = 0 THEN 999 ELSE priority END ASC, created_at ASC`

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
		tasks, err := s.ListStoryTasks(st.ID)
		if err != nil {
			return nil, err
		}
		st.Tasks = tasks
		stories = append(stories, st)
	}
	return stories, rows.Err()
}

func (s *Store) UpdateStory(st *task.Story) error {
	st.UpdatedAt = time.Now()
	st.LastActiveAt = st.UpdatedAt

	tags, _ := json.Marshal(st.Tags)

	_, err := s.db.Exec(
		`UPDATE stories SET title=?, description=?, layer_chain=?, status=?, priority=?, project=?, tags=?, last_active_at=?, updated_at=? WHERE id=?`,
		st.Title, st.Description, st.LayerChain, st.Status, st.Priority, st.Project,
		string(tags), st.LastActiveAt, st.UpdatedAt, st.ID,
	)
	return err
}

func (s *Store) DeleteStory(id string) error {
	_, err := s.db.Exec(`DELETE FROM stories WHERE id = ?`, id)
	return err
}

// --- Story Tasks ---

func (s *Store) CreateStoryTask(t *task.StoryTask) error {
	t.ID = uuid.New().String()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	if t.Status == "" {
		t.Status = task.TaskStatusPending
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}

	if t.SortOrder == 0 {
		var maxOrder int
		s.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM story_tasks WHERE story_id = ?`, t.StoryID).Scan(&maxOrder)
		t.SortOrder = maxOrder + 1
	}

	_, err := s.db.Exec(
		`INSERT INTO story_tasks (id, story_id, name, status, pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.StoryID, t.Name, t.Status, t.PRUrl, t.PRNumber, t.PRState, t.PRTitle, t.PRReviewDecision, boolToInt(t.PRIsDraft), t.PRCheckStatus, t.SortOrder, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET updated_at = ? WHERE id = ?`, now, t.StoryID)
	return err
}

func (s *Store) GetStoryTask(id string) (*task.StoryTask, error) {
	row := s.db.QueryRow(
		`SELECT id, story_id, name, status, pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at
		 FROM story_tasks WHERE id = ?`, id,
	)
	t, err := scanStoryTask(row)
	if err != nil {
		return nil, err
	}
	deps, err := s.loadTaskDeps([]string{t.ID})
	if err != nil {
		return nil, err
	}
	if d, ok := deps[t.ID]; ok {
		t.DependsOn = d
	} else {
		t.DependsOn = []string{}
	}
	return t, nil
}

func (s *Store) ListStoryTasks(storyID string) ([]*task.StoryTask, error) {
	rows, err := s.db.Query(
		`SELECT id, story_id, name, status, pr_url, pr_number, pr_state, pr_title, pr_review_decision, pr_is_draft, pr_check_status, sort_order, created_at, updated_at
		 FROM story_tasks WHERE story_id = ? ORDER BY sort_order ASC`, storyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*task.StoryTask
	var ids []string
	for rows.Next() {
		t, err := scanStoryTask(rows)
		if err != nil {
			return nil, err
		}
		t.DependsOn = []string{}
		tasks = append(tasks, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tasks == nil {
		return []*task.StoryTask{}, nil
	}

	deps, err := s.loadTaskDeps(ids)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if d, ok := deps[t.ID]; ok {
			t.DependsOn = d
		}
	}
	return tasks, nil
}

func (s *Store) UpdateStoryTask(t *task.StoryTask) error {
	t.UpdatedAt = time.Now()
	_, err := s.db.Exec(
		`UPDATE story_tasks SET name=?, status=?, pr_url=?, pr_number=?, pr_state=?, pr_title=?, pr_review_decision=?, pr_is_draft=?, pr_check_status=?, sort_order=?, updated_at=? WHERE id=?`,
		t.Name, t.Status, t.PRUrl, t.PRNumber, t.PRState, t.PRTitle, t.PRReviewDecision, boolToInt(t.PRIsDraft), t.PRCheckStatus, t.SortOrder, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET updated_at = ? WHERE id = ?`, t.UpdatedAt, t.StoryID)
	return err
}

func (s *Store) DeleteStoryTask(id string) error {
	var storyID string
	if err := s.db.QueryRow(`SELECT story_id FROM story_tasks WHERE id = ?`, id).Scan(&storyID); err != nil {
		return err
	}

	_, err := s.db.Exec(`DELETE FROM story_tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE stories SET updated_at = ? WHERE id = ?`, time.Now(), storyID)
	return err
}

func (s *Store) AddTaskDep(taskID, dependsOnID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`,
		taskID, dependsOnID,
	)
	return err
}

func (s *Store) RemoveTaskDep(taskID, dependsOnID string) error {
	_, err := s.db.Exec(
		`DELETE FROM task_dependencies WHERE task_id = ? AND depends_on_id = ?`,
		taskID, dependsOnID,
	)
	return err
}

// loadTaskDeps はタスク ID のリストに対して task_dependencies をバッチ取得し、
// taskID → []dependsOnID のマップを返す。
func (s *Store) loadTaskDeps(taskIDs []string) (map[string][]string, error) {
	if len(taskIDs) == 0 {
		return map[string][]string{}, nil
	}

	placeholders := strings.Repeat("?,", len(taskIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		args[i] = id
	}

	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT task_id, depends_on_id FROM task_dependencies WHERE task_id IN (%s)`, placeholders),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]string{}
	for rows.Next() {
		var taskID, depID string
		if err := rows.Scan(&taskID, &depID); err != nil {
			return nil, err
		}
		result[taskID] = append(result[taskID], depID)
	}
	return result, rows.Err()
}

func (s *Store) SnoozeStory(id string, until time.Time) error {
	_, err := s.db.Exec(`UPDATE stories SET snoozed_until = ?, updated_at = ? WHERE id = ?`,
		until.UTC().Format("2006-01-02 15:04:05"), time.Now(), id)
	return err
}

func (s *Store) UnsnoozeStory(id string) error {
	_, err := s.db.Exec(`UPDATE stories SET snoozed_until = NULL, updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

// --- Settings ---

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now(),
	)
	return err
}

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// --- PR URL duplicate check ---

func (s *Store) FindStoryByPRURL(prURL string) (string, error) {
	var id string
	err := s.db.QueryRow(
		`SELECT story_id FROM story_tasks WHERE pr_url = ? LIMIT 1`,
		prURL,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// --- scan helpers ---

func scanStoryTask(sc scannable) (*task.StoryTask, error) {
	t := &task.StoryTask{}
	var isDraft int
	if err := sc.Scan(&t.ID, &t.StoryID, &t.Name, &t.Status, &t.PRUrl, &t.PRNumber, &t.PRState, &t.PRTitle, &t.PRReviewDecision, &isDraft, &t.PRCheckStatus, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	t.PRIsDraft = isDraft != 0
	return t, nil
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
	var tagsJSON string
	var lastActiveAt sql.NullTime
	var snoozedUntil sql.NullTime

	err := sc.Scan(
		&st.ID, &st.Title, &st.Description, &st.LayerChain, &st.Status,
		&st.Priority, &st.Project, &tagsJSON,
		&snoozedUntil, &lastActiveAt, &st.CreatedAt, &st.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if snoozedUntil.Valid {
		st.SnoozedUntil = &snoozedUntil.Time
	}

	if lastActiveAt.Valid {
		st.LastActiveAt = lastActiveAt.Time
	} else {
		st.LastActiveAt = st.UpdatedAt
	}

	json.Unmarshal([]byte(tagsJSON), &st.Tags)

	if st.Tags == nil {
		st.Tags = []string{}
	}

	return st, nil
}
