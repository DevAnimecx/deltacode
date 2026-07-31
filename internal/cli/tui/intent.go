package tui

import (
	"strings"
)

type IntentType string

const (
	IntentBuild    IntentType = "build"
	IntentFix      IntentType = "fix"
	IntentExplain  IntentType = "explain"
	IntentRefactor IntentType = "refactor"
	IntentTest     IntentType = "test"
	IntentDeploy   IntentType = "deploy"
	IntentResearch IntentType = "research"
	IntentGeneral  IntentType = "general"
)

type Complexity string

const (
	ComplexityLow     Complexity = "low"
	ComplexityMedium  Complexity = "medium"
	ComplexityHigh    Complexity = "high"
	ComplexityUnknown Complexity = "unknown"
)

type Intent struct {
	Type            IntentType
	Complexity      Complexity
	Domain          string
	Tools           []string
	Files           []string
	EstimatedTokens int
	EstimatedCost   float64
	Confidence      float64
}

func (m *model) parseIntent(prompt string) Intent {
	lower := strings.ToLower(prompt)
	intent := Intent{
		Type:       IntentGeneral,
		Complexity: ComplexityMedium,
		Confidence: 0.5,
	}

	if strings.Contains(lower, "build") || strings.Contains(lower, "create") || strings.Contains(lower, "implement") || strings.Contains(lower, "add") {
		intent.Type = IntentBuild
		intent.Confidence = 0.8
	}
	if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") || strings.Contains(lower, "error") || strings.Contains(lower, "broken") {
		intent.Type = IntentFix
		intent.Confidence = 0.85
	}
	if strings.Contains(lower, "explain") || strings.Contains(lower, "what") || strings.Contains(lower, "how") || strings.Contains(lower, "why") {
		intent.Type = IntentExplain
		intent.Confidence = 0.9
	}
	if strings.Contains(lower, "refactor") || strings.Contains(lower, "clean") || strings.Contains(lower, "improve") {
		intent.Type = IntentRefactor
		intent.Confidence = 0.8
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "spec") {
		intent.Type = IntentTest
		intent.Confidence = 0.8
	}
	if strings.Contains(lower, "deploy") || strings.Contains(lower, "release") {
		intent.Type = IntentDeploy
		intent.Confidence = 0.7
	}

	if strings.Contains(lower, "auth") || strings.Contains(lower, "login") || strings.Contains(lower, "jwt") {
		intent.Domain = "authentication"
		intent.Tools = append(intent.Tools, "database", "backend", "middleware")
	} else if strings.Contains(lower, "api") {
		intent.Domain = "api"
		intent.Tools = append(intent.Tools, "backend", "routes", "validation")
	} else if strings.Contains(lower, "ui") || strings.Contains(lower, "frontend") || strings.Contains(lower, "component") {
		intent.Domain = "frontend"
		intent.Tools = append(intent.Tools, "frontend", "components", "styles")
	} else if strings.Contains(lower, "database") || strings.Contains(lower, "db") || strings.Contains(lower, "schema") {
		intent.Domain = "database"
		intent.Tools = append(intent.Tools, "database", "migration")
	} else if strings.Contains(lower, "test") {
		intent.Domain = "testing"
		intent.Tools = append(intent.Tools, "test", "coverage")
	}

	wordCount := len(strings.Fields(prompt))
	if wordCount < 10 {
		intent.Complexity = ComplexityLow
	} else if wordCount < 30 {
		intent.Complexity = ComplexityMedium
	} else {
		intent.Complexity = ComplexityHigh
	}

	intent.EstimatedTokens = 500 + wordCount*10
	intent.EstimatedCost = float64(intent.EstimatedTokens) * 0.000002

	return intent
}
