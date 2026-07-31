package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFile(t *testing.T) {
	cases := map[string]string{
		"a.go":       "go",
		"b.py":       "python",
		"c.ts":       "typescript",
		"d.cpp":      "cpp",
		"e.rs":       "rust",
		"f.txt":      "",
		"g.json":     "json",
	}
	for path, want := range cases {
		if got := DetectFile(path); got != want {
			t.Errorf("DetectFile(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestCheckSecurity(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.go")
	os.WriteFile(clean, []byte("package main\n\nfunc main() { println(\"hi\") }\n"), 0644)
	bad := filepath.Join(dir, "bad.py")
	os.WriteFile(bad, []byte("password = 'supersecret123'\nimport os\nexec(input())\n"), 0644)

	p := New(dir)
	res := p.CheckSecurity([]File{
		{Path: "clean.go"},
		{Path: "bad.py"},
	})
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	for _, r := range res {
		switch r.File {
		case "clean.go":
			if !r.Passed {
				t.Errorf("clean.go flagged: %s", r.Message)
			}
		case "bad.py":
			if r.Passed {
				t.Errorf("bad.py should be flagged")
			}
			if !containsAny(r.Message, "password", "eval", "injection") {
				t.Errorf("bad.py unexpected message: %s", r.Message)
			}
		}
	}
}

func TestValidateFilesGo(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "main.go")
	os.WriteFile(f, []byte("package main\n\nfunc main() {}\n"), 0644)
	p := New(dir)
	res := p.ValidateFiles([]File{{Path: "main.go"}})
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	for _, r := range res {
		if !r.Passed {
			t.Errorf("unexpected failure: %s", r.String())
		}
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && s != "" && containsFold(s, sub) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if stringsFoldEqual(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func stringsFoldEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
