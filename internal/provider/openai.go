package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
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

func parseSSELine(line string) (string, bool) {
	if !strings.HasPrefix(line, "data: ") {
		return "", false
	}
	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return "", true
	}
	return data, false
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	// Handle both SSE-wrapped and plain JSON responses.
	if strings.HasPrefix(bodyStr, "data: ") {
		var content strings.Builder
		var reasoning strings.Builder
		var usage *openAIUsage
		finalID := ""
		finalModel := ""
		scanner := bufio.NewScanner(strings.NewReader(bodyStr))
		for scanner.Scan() {
			line := scanner.Text()
			data, done := parseSSELine(line)
			if done {
				break
			}
			if data == "" {
				continue
			}
			var chunk openAIChatResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.ID != "" {
				finalID = chunk.ID
			}
			if chunk.Model != "" {
				finalModel = chunk.Model
			}
			for _, choice := range chunk.Choices {
				if choice.Message.Content != "" {
					content.WriteString(choice.Message.Content)
				}
				if choice.Message.ReasoningContent != "" {
					reasoning.WriteString(choice.Message.ReasoningContent)
				}
				if choice.Delta.Content != "" {
					content.WriteString(choice.Delta.Content)
				}
				if choice.Delta.ReasoningContent != "" {
					reasoning.WriteString(choice.Delta.ReasoningContent)
				}
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
		}

		msg := content.String()
		if msg == "" {
			msg = reasoning.String()
		}
		cr := &models.ChatResponse{
			ID:    finalID,
			Model: finalModel,
			Message: models.Message{
				Role:    models.RoleAssistant,
				Content: msg,
			},
		}
		if usage != nil {
			cr.Usage = models.Usage{
				PromptTokens:     usage.PromptTokens,
				CompletionTokens: usage.CompletionTokens,
				TotalTokens:      usage.TotalTokens,
			}
		}
		return cr, nil
	}

	var result openAIChatResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	content := result.Choices[0].Message.Content
	if content == "" {
		content = result.Choices[0].Message.ReasoningContent
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

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	httpReq = httpReq.WithContext(ctx)

	ch := make(chan models.StreamChunk, 64)

	go func() {
		defer cancel()
		defer close(ch)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			ch <- models.StreamChunk{Error: fmt.Errorf("API request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ch <- models.StreamChunk{Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

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
				content := choice.Delta.Content
				reasoning := choice.Delta.ReasoningContent
				if content == "" && reasoning != "" {
					content = reasoning
				}
				sc := models.StreamChunk{
					Model:     chunk.Model,
					Reasoning: reasoning,
				}
				if content == "" && (choice.FinishReason != "stop" || chunk.Usage == nil) {
					continue
				}
				if content != "" {
					sc.Content = content
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

		if err := scanner.Err(); err != nil {
			ch <- models.StreamChunk{Error: fmt.Errorf("stream read error: %w", err)}
			return
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
