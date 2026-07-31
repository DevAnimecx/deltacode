package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/symbols"
)

func osReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func readAny(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func dirOf(path string) string {
	return filepath.Dir(path)
}

func joinPath(dir, name string) string {
	return filepath.Join(dir, name)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func compileRegex(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return re, nil
}

func newSymbolIndexer(root string) *symbols.Indexer {
	return symbols.NewIndexer(root)
}

// capabilityKeywords maps capability queries to tool categories.
func capabilityKeywords(capability string) []string {
	c := strings.ToLower(capability)
	var out []string
	for _, kw := range strings.FieldsFunc(c, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	}) {
		if len(kw) > 2 {
			out = append(out, kw)
		}
	}
	return out
}
