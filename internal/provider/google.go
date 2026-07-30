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

type Google struct {
	cfg models.ProviderConfig
}

func NewGoogle(cfg models.ProviderConfig) *Google {
	return &Google{cfg: cfg}
}

func (p *Google) Name() string {
	return p.cfg.Name
}

func (p *Google) Type() models.ProviderType {
	return p.cfg.Type
}

func (p *Google) ListModels() ([]models.Model, error) {
	req, err := http.NewRequest("GET", p.cfg.BaseURL+"/models?key="+p.cfg.APIKey, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var modelList []models.Model
	for _, m := range result.Items {
		shortID := strings.TrimPrefix(m.Name, "models/")
		modelList = append(modelList, models.Model{
			ID:       shortID,
			Provider: p.cfg.Type,
			Name:     shortID,
		})
	}
	return modelList, nil
}

type googlePart struct {
	Text string `json:"text"`
}

type googleContent struct {
	Role  string        `json:"role,omitempty"`
	Parts []googlePart  `json:"parts"`
}

type googleRequest struct {
	Contents          []googleContent  `json:"contents"`
	SystemInstruction *googleContent   `json:"system_instruction,omitempty"`
	GenerationConfig  struct {
		MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
		Temperature     float64 `json:"temperature,omitempty"`
	} `json:"generationConfig,omitempty"`
}

type googleResponse struct {
	Candidates []struct {
		Content googleContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

func (p *Google) Chat(req models.ChatRequest) (*models.ChatResponse, error) {
	var contents []googleContent
	var system string

	for _, msg := range req.Messages {
		if msg.Role == models.RoleSystem {
			system += msg.Content + "\n"
		} else {
			role := "user"
			if msg.Role == models.RoleAssistant {
				role = "model"
			}
			contents = append(contents, googleContent{
				Role:  role,
				Parts: []googlePart{{Text: msg.Content}},
			})
		}
	}

	gReq := googleRequest{Contents: contents}
	if system != "" {
		gReq.SystemInstruction = &googleContent{
			Parts: []googlePart{{Text: strings.TrimSpace(system)}},
		}
	}
	gReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	gReq.GenerationConfig.Temperature = req.Temperature

	payload, _ := json.Marshal(gReq)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.cfg.BaseURL, req.Model, p.cfg.APIKey)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result googleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned")
	}

	content := ""
	for _, p := range result.Candidates[0].Content.Parts {
		content += p.Text
	}

	cr := &models.ChatResponse{
		Model: req.Model,
		Message: models.Message{
			Role:    models.RoleAssistant,
			Content: content,
		},
	}
	if result.UsageMetadata != nil {
		cr.Usage = models.Usage{
			PromptTokens:     result.UsageMetadata.PromptTokenCount,
			CompletionTokens: result.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      result.UsageMetadata.TotalTokenCount,
		}
	}
	return cr, nil
}

func (p *Google) ChatStream(req models.ChatRequest) (<-chan models.StreamChunk, error) {
	var contents []googleContent
	var system string

	for _, msg := range req.Messages {
		if msg.Role == models.RoleSystem {
			system += msg.Content + "\n"
		} else {
			role := "user"
			if msg.Role == models.RoleAssistant {
				role = "model"
			}
			contents = append(contents, googleContent{
				Role:  role,
				Parts: []googlePart{{Text: msg.Content}},
			})
		}
	}

	gReq := googleRequest{Contents: contents}
	if system != "" {
		gReq.SystemInstruction = &googleContent{
			Parts: []googlePart{{Text: strings.TrimSpace(system)}},
		}
	}
	gReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	gReq.GenerationConfig.Temperature = req.Temperature

	payload, _ := json.Marshal(gReq)
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", p.cfg.BaseURL, req.Model, p.cfg.APIKey)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")

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

			var chunk struct {
				Candidates []struct {
					Content googleContent `json:"content"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			for _, c := range chunk.Candidates {
				for _, p := range c.Content.Parts {
					ch <- models.StreamChunk{Content: p.Text}
				}
			}
		}
		ch <- models.StreamChunk{Done: true}
	}()
	return ch, nil
}
