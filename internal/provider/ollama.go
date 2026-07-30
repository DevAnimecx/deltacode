package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Ollama struct {
	cfg models.ProviderConfig
}

func NewOllama(cfg models.ProviderConfig) *Ollama {
	return &Ollama{cfg: cfg}
}

func (p *Ollama) Name() string {
	return p.cfg.Name
}

func (p *Ollama) Type() models.ProviderType {
	return p.cfg.Type
}

func (p *Ollama) ListModels() ([]models.Model, error) {
	resp, err := http.Get(p.cfg.BaseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var modelList []models.Model
	for _, m := range result.Models {
		modelList = append(modelList, models.Model{
			ID:       m.Name,
			Provider: p.cfg.Type,
			Name:     m.Name,
		})
	}
	return modelList, nil
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  struct {
		Temperature float64 `json:"temperature,omitempty"`
	} `json:"options,omitempty"`
}

type ollamaResponse struct {
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
	EvalCount int           `json:"eval_count,omitempty"`
}

func (p *Ollama) Chat(req models.ChatRequest) (*models.ChatResponse, error) {
	body := ollamaRequest{
		Model:    req.Model,
		Stream:   false,
		Messages: toOllamaMessages(req.Messages),
	}
	body.Options.Temperature = req.Temperature

	payload, _ := json.Marshal(body)
	resp, err := http.Post(p.cfg.BaseURL+"/api/chat", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &models.ChatResponse{
		Model: req.Model,
		Message: models.Message{
			Role:    models.RoleAssistant,
			Content: result.Message.Content,
		},
	}, nil
}

func (p *Ollama) ChatStream(req models.ChatRequest) (<-chan models.StreamChunk, error) {
	body := ollamaRequest{
		Model:    req.Model,
		Stream:   true,
		Messages: toOllamaMessages(req.Messages),
	}
	body.Options.Temperature = req.Temperature

	payload, _ := json.Marshal(body)
	resp, err := http.Post(p.cfg.BaseURL+"/api/chat", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	ch := make(chan models.StreamChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var chunk ollamaResponse
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				continue
			}
			ch <- models.StreamChunk{
				Content: chunk.Message.Content,
				Done:    chunk.Done,
			}
		}
	}()
	return ch, nil
}

func toOllamaMessages(msgs []models.Message) []ollamaMessage {
	result := make([]ollamaMessage, len(msgs))
	for i, m := range msgs {
		result[i] = ollamaMessage{Role: string(m.Role), Content: m.Content}
	}
	return result
}
