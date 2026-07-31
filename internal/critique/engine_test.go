package critique

import (
	"strings"
	"testing"
)

func TestParseScore(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"Score: 85", 85},
		{"Overall score: 92.5", 92.5},
		{"Score: 70 out of 100", 70},
		{"no score here", 0},
		{"Score: 150", 0},
		{"The quality is fine.\nScore: 64\nDone.", 64},
	}
	for _, c := range cases {
		if got := parseScore(c.in); got != c.want {
			t.Errorf("parseScore(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseIssues(t *testing.T) {
	out := `1. [critical] SQL injection in query builder
2. [warning] Missing error handling
3. [info] Consider renaming foo
no severity here`
	issues := parseIssues(out, AspectSecurity)
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}
	if issues[0].Severity != "critical" || issues[1].Severity != "warning" || issues[2].Severity != "info" {
		t.Fatalf("unexpected severities: %+v", issues)
	}
	if issues[0].Aspect != AspectSecurity {
		t.Fatalf("aspect not propagated: %+v", issues[0])
	}
}

func TestEngineDefaults(t *testing.T) {
	e := New(nil, "test-model")
	if e == nil {
		t.Fatal("engine nil")
	}
	if len(e.Aspects()) != 6 {
		t.Fatalf("expected 6 aspects, got %d", len(e.Aspects()))
	}
	if !strings.Contains(aspectPrompt(AspectPerformance), "Score 0-100") {
		t.Fatal("aspect prompt missing scoring instruction")
	}
}
