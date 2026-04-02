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
	Mergeable      string `json:"mergeable"`
	ReviewDecision string `json:"reviewDecision"`
	URL            string `json:"url"`
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) GetPRInfo(prURL string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "view", prURL,
		"--json", "number,title,state,mergeable,reviewDecision,url")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh pr view failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var info PRInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	return &info, nil
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
