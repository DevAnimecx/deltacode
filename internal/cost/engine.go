package cost

import (
	"math"
	"strings"
)

type ModelRates struct {
	InputPrice   float64 // per 1K tokens
	OutputPrice  float64
	SpeedScore   float64 // 0-100
	QualityScore float64 // 0-100
}

var modelDatabase = map[string]ModelRates{
	"gpt-4o":           {InputPrice: 0.0025, OutputPrice: 0.01, SpeedScore: 70, QualityScore: 95},
	"gpt-4o-mini":      {InputPrice: 0.00015, OutputPrice: 0.0006, SpeedScore: 85, QualityScore: 80},
	"claude-sonnet-4":  {InputPrice: 0.003, OutputPrice: 0.015, SpeedScore: 65, QualityScore: 97},
	"claude-haiku-3.5": {InputPrice: 0.0008, OutputPrice: 0.004, SpeedScore: 90, QualityScore: 78},
	"gemini-2.0-flash": {InputPrice: 0.0001, OutputPrice: 0.0004, SpeedScore: 95, QualityScore: 75},
	"gemini-1.5-pro":   {InputPrice: 0.00125, OutputPrice: 0.005, SpeedScore: 60, QualityScore: 90},
	"deepseek-chat":    {InputPrice: 0.0005, OutputPrice: 0.002, SpeedScore: 88, QualityScore: 82},
	"deepseek-coder":   {InputPrice: 0.001, OutputPrice: 0.004, SpeedScore: 85, QualityScore: 88},
}

type Estimate struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost"`
	LatencyMs    int     `json:"latency_ms"`
	Quality      float64 `json:"quality"`
	Score        float64 `json:"score"`
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Estimate(model string, inputTokens, outputTokens int) *Estimate {
	rates, found := modelDatabase[normalizeModel(model)]
	if !found {
		for key, rate := range modelDatabase {
			if strings.Contains(strings.ToLower(model), strings.ToLower(key)) {
				rates = rate
				found = true
				break
			}
		}
	}
	if !found {
		rates = ModelRates{InputPrice: 0.001, OutputPrice: 0.003, SpeedScore: 70, QualityScore: 70}
	}

	cost := (float64(inputTokens)/1000)*rates.InputPrice + (float64(outputTokens)/1000)*rates.OutputPrice
	latency := int(float64(outputTokens) / rates.SpeedScore * 100)

	quality := rates.QualityScore
	speed := rates.SpeedScore

	score := (quality*0.5 + speed*0.2 + (100-math.Min(cost*10000, 100))*0.3)

	return &Estimate{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         math.Round(cost*100000) / 100000,
		LatencyMs:    latency,
		Quality:      quality,
		Score:        math.Round(score*10) / 10,
	}
}

func (e *Engine) BestModel(inputTokens, outputTokens int, preferQuality bool) (string, float64) {
	var bestModel string
	var bestScore float64

	for model := range modelDatabase {
		est := e.Estimate(model, inputTokens, outputTokens)
		score := est.Score
		if preferQuality {
			score = est.Quality*0.7 + (100-est.Cost*10000)*0.3
		}
		if score > bestScore {
			bestScore = score
			bestModel = model
		}
	}

	return bestModel, math.Round(bestScore*10) / 10
}

func normalizeModel(model string) string {
	normalized := strings.ToLower(model)
	normalized = strings.TrimPrefix(normalized, "models/")
	return normalized
}
