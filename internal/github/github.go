package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PRInfo struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	State          string `json:"state"`
	IsDraft        bool   `json:"isDraft"`
	Mergeable      string `json:"mergeable"`
	ReviewDecision string `json:"reviewDecision"`
	CheckStatus    string `json:"checkStatus"`
	URL            string `json:"url"`
}

type checkRollupItem struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

type ghRawResponse struct {
	Number             int               `json:"number"`
	Title              string            `json:"title"`
	State              string            `json:"state"`
	IsDraft            bool              `json:"isDraft"`
	Mergeable          string            `json:"mergeable"`
	ReviewDecision     string            `json:"reviewDecision"`
	URL                string            `json:"url"`
	StatusCheckRollup  []checkRollupItem `json:"statusCheckRollup"`
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) GetPRInfo(prURL string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "view", prURL,
		"--json", "number,title,state,isDraft,mergeable,reviewDecision,url,statusCheckRollup")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh pr view failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var raw ghRawResponse
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	return &PRInfo{
		Number:         raw.Number,
		Title:          raw.Title,
		State:          raw.State,
		IsDraft:        raw.IsDraft,
		Mergeable:      raw.Mergeable,
		ReviewDecision: raw.ReviewDecision,
		CheckStatus:    summarizeChecks(raw.StatusCheckRollup),
		URL:            raw.URL,
	}, nil
}

func summarizeChecks(checks []checkRollupItem) string {
	if len(checks) == 0 {
		return ""
	}
	hasFailure := false
	hasPending := false
	for _, c := range checks {
		conclusion := c.Conclusion
		if conclusion == "" {
			conclusion = c.State
		}
		switch conclusion {
		case "FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED":
			hasFailure = true
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			// ok
		default:
			hasPending = true
		}
	}
	if hasFailure {
		return "FAILURE"
	}
	if hasPending {
		return "PENDING"
	}
	return "SUCCESS"
}

func (c *Client) IsAvailable() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

type SearchPRItem struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	IsDraft   bool      `json:"isDraft"`
	URL       string    `json:"url"`
	Repo      string    `json:"repo"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ghSearchRepository struct {
	NameWithOwner string `json:"nameWithOwner"`
	Name          string `json:"name"`
}

type ghSearchAuthor struct {
	Login string `json:"login"`
}

type ghSearchItem struct {
	Number     int                `json:"number"`
	Title      string             `json:"title"`
	State      string             `json:"state"`
	IsDraft    bool               `json:"isDraft"`
	URL        string             `json:"url"`
	Repository ghSearchRepository `json:"repository"`
	Author     ghSearchAuthor     `json:"author"`
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
}

func (c *Client) SearchOrgPRs(org string, since time.Time) ([]*SearchPRItem, error) {
	sinceStr := since.Format("2006-01-02")

	cmd := exec.Command("gh", "search", "prs",
		"--owner", org,
		"--state", "open",
		"--sort", "created",
		"--order", "asc",
		"--updated", ">="+sinceStr,
		"--json", "number,title,state,isDraft,url,repository,author,createdAt,updatedAt",
		"--limit", "100",
	)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh search prs failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh search prs failed: %w", err)
	}

	var items []ghSearchItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("failed to parse gh search output: %w", err)
	}

	result := make([]*SearchPRItem, 0, len(items))
	for _, item := range items {
		repo := item.Repository.Name
		if repo == "" {
			parts := strings.SplitN(item.Repository.NameWithOwner, "/", 2)
			if len(parts) == 2 {
				repo = parts[1]
			}
		}
		result = append(result, &SearchPRItem{
			Number:    item.Number,
			Title:     item.Title,
			State:     item.State,
			IsDraft:   item.IsDraft,
			URL:       item.URL,
			Repo:      repo,
			Author:    item.Author.Login,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	return result, nil
}

func IsBotAuthor(login string) bool {
	return strings.HasSuffix(login, "[bot]") || strings.HasSuffix(login, "-bot")
}

var prURLPattern = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)

func ParsePRURL(url string) (owner, repo string, number int, ok bool) {
	m := prURLPattern.FindStringSubmatch(url)
	if len(m) != 4 {
		return "", "", 0, false
	}
	n, _ := strconv.Atoi(m[3])
	return m[1], m[2], n, true
}
