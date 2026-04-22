package github

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PRInfo struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	State          string    `json:"state"`
	IsDraft        bool      `json:"isDraft"`
	Mergeable      string    `json:"mergeable"`
	ReviewDecision string    `json:"reviewDecision"`
	CheckStatus    string    `json:"checkStatus"`
	URL            string    `json:"url"`
	HeadRefName    string    `json:"headRefName"`
	UpdatedAt      time.Time `json:"updatedAt"`
	MergeCommitSHA string    `json:"mergeCommitSha"`
	Released       bool      `json:"released"`
}

type checkRollupItem struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

type ghMergeCommit struct {
	OID string `json:"oid"`
}

type ghRawResponse struct {
	Number            int               `json:"number"`
	Title             string            `json:"title"`
	State             string            `json:"state"`
	IsDraft           bool              `json:"isDraft"`
	Mergeable         string            `json:"mergeable"`
	ReviewDecision    string            `json:"reviewDecision"`
	URL               string            `json:"url"`
	HeadRefName       string            `json:"headRefName"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	MergeCommit       *ghMergeCommit    `json:"mergeCommit"`
	StatusCheckRollup []checkRollupItem `json:"statusCheckRollup"`
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) GetPRInfo(prURL string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "view", prURL,
		"--json", "number,title,state,isDraft,mergeable,reviewDecision,url,statusCheckRollup,headRefName,updatedAt,mergeCommit")

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

	mergeSHA := ""
	if raw.MergeCommit != nil {
		mergeSHA = raw.MergeCommit.OID
	}

	info := &PRInfo{
		Number:         raw.Number,
		Title:          raw.Title,
		State:          raw.State,
		IsDraft:        raw.IsDraft,
		Mergeable:      raw.Mergeable,
		ReviewDecision: raw.ReviewDecision,
		CheckStatus:    summarizeChecks(raw.StatusCheckRollup),
		URL:            raw.URL,
		HeadRefName:    raw.HeadRefName,
		UpdatedAt:      raw.UpdatedAt,
		MergeCommitSHA: mergeSHA,
	}

	if raw.State == "MERGED" && mergeSHA != "" {
		if owner, repo, _, ok := ParsePRURL(prURL); ok {
			released, err := c.checkReleased(owner, repo, mergeSHA)
			if err != nil {
				fmt.Fprintf(logWriter, "warning: checkReleased failed for %s: %v\n", prURL, err)
			}
			info.Released = released
		}
	}

	return info, nil
}

// logWriter はリリース判定関連のログ出力先。デフォルトは stderr。
var logWriter io.Writer = os.Stderr

// checkReleased はマージコミット以降に新しいタグが存在するかを GraphQL で判定する。
// refs を TAG_COMMIT_DATE DESC で取得するので、REST /tags の順序不定問題を回避できる。
func (c *Client) checkReleased(owner, repo, mergeSHA string) (bool, error) {
	query := `query($owner: String!, $name: String!, $oid: GitObjectID!) {
		repository(owner: $owner, name: $name) {
			object(oid: $oid) {
				... on Commit { committedDate }
			}
			refs(refPrefix: "refs/tags/", first: 1, orderBy: {field: TAG_COMMIT_DATE, direction: DESC}) {
				nodes {
					name
					target {
						__typename
						... on Commit { committedDate oid }
						... on Tag { target { ... on Commit { committedDate oid } } }
					}
				}
			}
		}
	}`

	cmd := exec.Command("gh", "api", "graphql",
		"-f", "query="+query,
		"-f", "owner="+owner,
		"-f", "name="+repo,
		"-f", "oid="+mergeSHA,
	)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, fmt.Errorf("gh api graphql failed: %s", string(exitErr.Stderr))
		}
		return false, fmt.Errorf("gh api graphql failed: %w", err)
	}

	var resp struct {
		Data struct {
			Repository struct {
				Object struct {
					CommittedDate time.Time `json:"committedDate"`
				} `json:"object"`
				Refs struct {
					Nodes []struct {
						Name   string `json:"name"`
						Target struct {
							Typename      string    `json:"__typename"`
							CommittedDate time.Time `json:"committedDate"`
							OID           string    `json:"oid"`
							Target        struct {
								CommittedDate time.Time `json:"committedDate"`
								OID           string    `json:"oid"`
							} `json:"target"`
						} `json:"target"`
					} `json:"nodes"`
				} `json:"refs"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return false, fmt.Errorf("failed to parse graphql response: %w", err)
	}

	mergeDate := resp.Data.Repository.Object.CommittedDate
	if mergeDate.IsZero() {
		return false, fmt.Errorf("merge commit %s not found", mergeSHA)
	}

	nodes := resp.Data.Repository.Refs.Nodes
	if len(nodes) == 0 {
		return false, nil
	}
	tagTarget := nodes[0].Target
	tagDate := tagTarget.CommittedDate
	tagOID := tagTarget.OID
	if tagTarget.Typename == "Tag" {
		tagDate = tagTarget.Target.CommittedDate
		tagOID = tagTarget.Target.OID
	}

	if tagOID == mergeSHA {
		return true, nil
	}
	return tagDate.After(mergeDate), nil
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
