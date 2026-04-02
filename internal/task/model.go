package task

import "time"

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDone       Status = "done"
)

type Task struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       Status    `json:"status"`
	Priority     int       `json:"priority"`
	Project      string    `json:"project"`
	Links        []string  `json:"links"`
	Tags         []string  `json:"tags"`
	BlockedBy    []string  `json:"blocked_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastActiveAt time.Time `json:"last_active_at"`
}

type ContextEntry struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Content   string    `json:"content"`
	EntryType string    `json:"entry_type"` // note, progress, blocker
	CreatedAt time.Time `json:"created_at"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // user, assistant
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Summary struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"` // global, project:{name}, task:{id}
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type StoryStatus string

const (
	StoryStatusActive    StoryStatus = "active"
	StoryStatusCompleted StoryStatus = "completed"
	StoryStatusArchived  StoryStatus = "archived"
)

type LayerStatus string

const (
	LayerStatusPending    LayerStatus = "pending"
	LayerStatusInProgress LayerStatus = "in_progress"
	LayerStatusReview     LayerStatus = "review"
	LayerStatusMerged     LayerStatus = "merged"
)

type Story struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	LayerChain  string        `json:"layer_chain"`
	Status      StoryStatus   `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Layers      []*StoryLayer `json:"layers"`
}

type StoryLayer struct {
	ID               string      `json:"id"`
	StoryID          string      `json:"story_id"`
	LayerName        string      `json:"layer_name"`
	Status           LayerStatus `json:"status"`
	PRUrl            string      `json:"pr_url"`
	PRNumber         int         `json:"pr_number"`
	PRState          string      `json:"pr_state"`
	PRTitle          string      `json:"pr_title"`
	PRReviewDecision string      `json:"pr_review_decision"`
	PRIsDraft        bool        `json:"pr_is_draft"`
	PRCheckStatus    string      `json:"pr_check_status"`
	SortOrder        int         `json:"sort_order"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}
