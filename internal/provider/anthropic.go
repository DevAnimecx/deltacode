package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Anthropic struct {
	cfg models.ProviderConfig
}

func NewAnthropic(cfg models.ProviderConfig) *Anthropic {
	return &Anthropic{cfg: cfg}
}

func (p *Anthropic) Name() string {
	return p.cfg.Name
}

func (p *Anthropic) Type() models.ProviderType {
	return p.cfg.Type
}

func (p *Anthropic) ListModels() ([]models.Model, error) {
	return []models.Model{
		{ID: "claude-sonnet-4-20250514", Provider: p.cfg.Type, Name: "Claude Sonnet 4"},
		{ID: "claude-haiku-3-5-20241022", Provider: p.cfg.Type, Name: "Claude Haiku 3.5"},
	}, nil
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []anthropicMessage  `json:"messages"`
	System      string              `json:"system,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	ID        string              `json:"id"`
	Model     string              `json:"model"`
	Content   []anthropicContent  `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage     *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

func (p *Anthropic) Chat(req models.ChatRequest) (*models.ChatResponse, error) {
	var system string
	var msgs []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == models.RoleSystem {
			system += m.Content + "\n"
		} else {
			msgs = append(msgs, anthropicMessage{Role: string(m.Role), Content: m.Content})
		}
	}

	body := anthropicRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}
	if system != "" {
		body.System = strings.TrimSpace(system)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", p.cfg.BaseURL+"/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Anthropic API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	content := ""
	for _, c := range result.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	cr := &models.ChatResponse{
		ID:    result.ID,
		Model: result.Model,
		Message: models.Message{
			Role:    models.RoleAssistant,
			Content: content,
		},
	}
	if result.Usage != nil {
		cr.Usage = models.Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		}
	}
	return cr, nil
}

func (p *Anthropic) ChatStream(req models.ChatRequest) (<-chan models.StreamChunk, error) {
	var system string
	var msgs []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == models.RoleSystem {
			system += m.Content + "\n"
		} else {
			msgs = append(msgs, anthropicMessage{Role: string(m.Role), Content: m.Content})
		}
	}

	body := anthropicRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}
	if system != "" {
		body.System = strings.TrimSpace(system)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", p.cfg.BaseURL+"/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	ch := make(chan models.StreamChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event struct {
				Type  string `json:"type"`
				Delta *struct {
					Text string `json:"text"`
				} `json:"delta,omitempty"`
				ContentBlock *struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content_block,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil {
					ch <- models.StreamChunk{Content: event.Delta.Text}
				}
			case "message_stop":
				ch <- models.StreamChunk{Done: true}
			}
		}
	}()
	return ch, nil
}
