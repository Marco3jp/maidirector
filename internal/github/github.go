package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
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

var prURLPattern = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)

func ParsePRURL(url string) (owner, repo string, number int, ok bool) {
	m := prURLPattern.FindStringSubmatch(url)
	if len(m) != 4 {
		return "", "", 0, false
	}
	n, _ := strconv.Atoi(m[3])
	return m[1], m[2], n, true
}
