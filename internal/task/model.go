package task

import "time"

type ContextEntry struct {
	ID        string    `json:"id"`
	StoryID   string    `json:"story_id"`
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

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusMerged     TaskStatus = "merged"
	TaskStatusReleased   TaskStatus = "released"
)

type Story struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	LayerChain   string       `json:"layer_chain"`
	Status       StoryStatus  `json:"status"`
	Priority     int          `json:"priority"`
	Project      string       `json:"project"`
	Tags         []string     `json:"tags"`
	SnoozedUntil *time.Time   `json:"snoozed_until"`
	LastActiveAt time.Time    `json:"last_active_at"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	Tasks        []*StoryTask `json:"tasks"`
}

type StoryTask struct {
	ID               string     `json:"id"`
	StoryID          string     `json:"story_id"`
	Name             string     `json:"name"`
	Status           TaskStatus `json:"status"`
	PRUrl            string     `json:"pr_url"`
	PRNumber         int        `json:"pr_number"`
	PRState          string     `json:"pr_state"`
	PRTitle          string     `json:"pr_title"`
	PRReviewDecision string     `json:"pr_review_decision"`
	PRIsDraft        bool       `json:"pr_is_draft"`
	PRCheckStatus    string     `json:"pr_check_status"`
	PRHeadRefName    string     `json:"pr_head_ref_name"`
	PRUpdatedAt      time.Time  `json:"pr_updated_at"`
	PRReleased       bool       `json:"pr_released"`
	SortOrder        int        `json:"sort_order"`
	DependsOn        []string   `json:"depends_on"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
