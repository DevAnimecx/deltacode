package prreview

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type PR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"body"`
	Branch      string `json:"head"`
	Base        string `json:"base"`
	Diff        string `json:"diff"`
	Repo        string `json:"repo"`
}

type Review struct {
	PR      int       `json:"pr"`
	Summary string    `json:"summary"`
	Issues  []PRIssue `json:"issues"`
	Approve bool      `json:"approve"`
}

type PRIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type Client struct {
	token    string
	provider models.ProviderConfig
	model    string
}

func NewClient(prov models.ProviderConfig, model, githubToken string) *Client {
	return &Client{
		token:    githubToken,
		provider: prov,
		model:    model,
	}
}

func (c *Client) FetchPR(repo string, prNumber int) (*PR, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", repo, prNumber)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var prData struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prData); err != nil {
		return nil, err
	}

	// Fetch diff separately
	diffReq, _ := http.NewRequest("GET", url, nil)
	diffReq.Header.Set("Authorization", "Bearer "+c.token)
	diffReq.Header.Set("Accept", "application/vnd.github.v3.diff")
	diffResp, err := client.Do(diffReq)
	var diffStr string
	if err == nil {
		defer diffResp.Body.Close()
		body, _ := io.ReadAll(diffResp.Body)
		diffStr = string(body)
	}

	return &PR{
		Number:      prData.Number,
		Title:       prData.Title,
		Description: prData.Body,
		Branch:      prData.Head.Ref,
		Base:        prData.Base.Ref,
		Diff:        diffStr,
		Repo:        repo,
	}, nil
}

func (c *Client) Review(pr *PR) (*Review, error) {
	p, err := provider.NewProvider(c.provider)
	if err != nil {
		return nil, err
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: c.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: `Review this pull request. Return JSON:
{
  "summary": "overall review",
  "issues": [
    {"file": "path", "line": 0, "severity": "critical|major|minor", "description": "issue"}
  ],
  "approve": true/false
}
Return ONLY valid JSON.`},
			{Role: models.RoleUser, Content: fmt.Sprintf("PR: %s\n\nDescription: %s\n\nDiff:\n%s", pr.Title, pr.Description, truncate(pr.Diff, 15000))},
		},
		Temperature: 0.2,
		MaxTokens:   4096,
	})
	if err != nil {
		return nil, err
	}

	content := cleanJSON(resp.Message.Content)
	var review Review
	if err := json.Unmarshal([]byte(content), &review); err != nil {
		return nil, fmt.Errorf("parse error: %w\nRaw: %s", err, content)
	}
	review.PR = pr.Number

	return &review, nil
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
