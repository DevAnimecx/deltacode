package symbols

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexDirectory(t *testing.T) {
	idx := NewIndexer(".")
	if err := idx.IndexDirectory("..", func(path string) bool {
		return false
	}); err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}
	if idx.Graph().Count() == 0 {
		t.Fatal("expected symbols indexed, got 0")
	}
	if len(idx.Graph().GetAllSymbols()) == 0 {
		t.Fatal("GetAllSymbols returned empty")
	}
}

func TestIndexGoFile(t *testing.T) {
	idx := NewIndexer(".")
	code := `package foo

// Add adds two ints.
func Add(a, b int) int { return a + b }

type Point struct{ X, Y int }

type Shape interface{ Area() float64 }

const Max = 100
`
	file := filepath.Join(t.TempDir(), "example.go")
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(file); err != nil {
		t.Fatal(err)
	}
	syms := idx.Graph().GetSymbolsByFile(file)
	if len(syms) == 0 {
		t.Fatal("no symbols for test file")
	}
	lookup := idx.Graph().Lookup("Add")
	if len(lookup) == 0 {
		t.Fatal("Lookup(Add) returned nothing")
	}
	if lookup[0].Signature != "func Add(a, b int) int { return a + b }" {
		t.Errorf("unexpected signature: %q", lookup[0].Signature)
	}
	if lookup[0].DocComment == "" {
		t.Error("expected doc comment extracted")
	}
	if !lookup[0].IsExported {
		t.Error("Add should be exported")
	}
	if syms2 := idx.Graph().GetSymbolsByFile("testdata/nope.go"); len(syms2) != 0 {
		t.Error("expected no symbols for missing file")
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := map[string]Language{
		"a.go":     LangGo,
		"b.py":     LangPython,
		"c.ts":     LangTypeScript,
		"d.rs":     LangRust,
		"e.java":   LangJava,
		"f.txt":    "",
	}
	for path, want := range cases {
		if got := detectLanguage(path); got != want {
			t.Errorf("detectLanguage(%q) = %q, want %q", path, got, want)
		}
	}
}
