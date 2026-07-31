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

func TestRemoveSymbolsByFile(t *testing.T) {
	idx := NewIndexer(".")
	dir := t.TempDir()
	code := "package foo\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Caller() { Add(1, 2) }\n"
	file := filepath.Join(dir, "lib.go")
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(file); err != nil {
		t.Fatal(err)
	}
	if len(idx.Graph().GetSymbolsByFile(file)) == 0 {
		t.Fatal("expected symbols before removal")
	}
	var callerID string
	for _, s := range idx.Graph().GetSymbolsByFile(file) {
		if s.Name == "Caller" {
			callerID = s.ID
		}
	}
	if callerID == "" {
		t.Fatal("Caller symbol not found")
	}
	if len(idx.Graph().GetCallees(callerID)) == 0 {
		t.Fatal("expected a call edge before removal")
	}

	idx.Graph().RemoveSymbolsByFile(file)

	if len(idx.Graph().GetSymbolsByFile(file)) != 0 {
		t.Fatal("symbols not removed")
	}
	if len(idx.Graph().Lookup("Add")) != 0 {
		t.Fatal("nameIndex not cleaned")
	}
	if len(idx.Graph().GetCallees(callerID)) != 0 {
		t.Fatal("call edges not removed")
	}
	if idx.Graph().Count() != 0 {
		t.Fatal("count not updated")
	}

	// Re-indexing the same file must restore symbols (incremental path).
	if err := idx.IndexFile(file); err != nil {
		t.Fatal(err)
	}
	if len(idx.Graph().GetSymbolsByFile(file)) == 0 {
		t.Fatal("re-index after removal failed")
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := map[string]Language{
		"a.go":   LangGo,
		"b.py":   LangPython,
		"c.ts":   LangTypeScript,
		"d.rs":   LangRust,
		"e.java": LangJava,
		"f.txt":  "",
	}
	for path, want := range cases {
		if got := detectLanguage(path); got != want {
			t.Errorf("detectLanguage(%q) = %q, want %q", path, got, want)
		}
	}
}
