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

type OpenAICompatible struct {
	cfg models.ProviderConfig
}

func NewOpenAICompatible(cfg models.ProviderConfig) *OpenAICompatible {
	return &OpenAICompatible{cfg: cfg}
}

func (p *OpenAICompatible) Name() string {
	return p.cfg.Name
}

func (p *OpenAICompatible) Type() models.ProviderType {
	return p.cfg.Type
}

func (p *OpenAICompatible) ListModels() ([]models.Model, error) {
	req, err := http.NewRequest("GET", p.cfg.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var modelList []models.Model
	for _, m := range result.Data {
		modelList = append(modelList, models.Model{
			ID:       m.ID,
			Provider: p.cfg.Type,
			Name:     m.ID,
		})
	}
	return modelList, nil
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message,omitempty"`
	Delta        openAIMessage `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIChatResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
}

func (p *OpenAICompatible) Chat(req models.ChatRequest) (*models.ChatResponse, error) {
	body := openAIChatRequest{
		Model:       req.Model,
		Messages:    toOpenAIMessages(req.Messages),
		Stream:      false,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", p.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	cr := &models.ChatResponse{
		ID:    result.ID,
		Model: result.Model,
		Message: models.Message{
			Role:    models.RoleAssistant,
			Content: result.Choices[0].Message.Content,
		},
	}
	if result.Usage != nil {
		cr.Usage = models.Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		}
	}
	return cr, nil
}

func (p *OpenAICompatible) ChatStream(req models.ChatRequest) (<-chan models.StreamChunk, error) {
	body := openAIChatRequest{
		Model:       req.Model,
		Messages:    toOpenAIMessages(req.Messages),
		Stream:      true,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", p.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

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
			if data == "[DONE]" {
				ch <- models.StreamChunk{Done: true}
				return
			}

			var chunk openAIChatResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			for _, choice := range chunk.Choices {
				sc := models.StreamChunk{
					Content: choice.Delta.Content,
					Model:   chunk.Model,
				}
				if choice.FinishReason == "stop" {
					sc.Done = true
				}
				if chunk.Usage != nil && sc.Done {
					sc.Usage = &models.Usage{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					}
				}
				ch <- sc
			}
		}
	}()
	return ch, nil
}

func toOpenAIMessages(msgs []models.Message) []openAIMessage {
	result := make([]openAIMessage, len(msgs))
	for i, m := range msgs {
		result[i] = openAIMessage{Role: string(m.Role), Content: m.Content}
	}
	return result
}
