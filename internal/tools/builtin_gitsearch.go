package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ---------- Git Tool ----------

func gitTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("git: subcommand required (status|diff|commit|branch|merge|rebase|cherry-pick|tag|restore|blame|log|stash|worktree)")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not installed")
	}
	switch args[0] {
	case "status":
		return gitRun(args[1:]...)
	case "diff":
		return gitRun(append([]string{"diff"}, args[1:]...)...)
	case "log":
		n := 10
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &n)
		}
		return gitRun("log", "--oneline", fmt.Sprintf("-%d", n))
	case "commit":
		msg := "update"
		if len(args) > 1 {
			msg = strings.Join(args[1:], " ")
		}
		return gitRun("commit", "-m", msg)
	case "branch":
		return gitRun(append([]string{"branch"}, args[1:]...)...)
	case "checkout":
		return gitRun(append([]string{"checkout"}, args[1:]...)...)
	case "merge":
		return gitRun(append([]string{"merge"}, args[1:]...)...)
	case "rebase":
		return gitRun(append([]string{"rebase"}, args[1:]...)...)
	case "cherry-pick":
		return gitRun(append([]string{"cherry-pick"}, args[1:]...)...)
	case "tag":
		return gitRun(append([]string{"tag"}, args[1:]...)...)
	case "restore":
		return gitRun(append([]string{"restore"}, args[1:]...)...)
	case "blame":
		return gitRun(append([]string{"blame"}, args[1:]...)...)
	case "stash":
		return gitRun(append([]string{"stash"}, args[1:]...)...)
	case "worktree":
		return gitRun(append([]string{"worktree"}, args[1:]...)...)
	case "add":
		return gitRun(append([]string{"add"}, args[1:]...)...)
	case "push":
		return gitRun(append([]string{"push"}, args[1:]...)...)
	case "pull":
		return gitRun(append([]string{"pull"}, args[1:]...)...)
	case "remote":
		return gitRun(append([]string{"remote"}, args[1:]...)...)
	default:
		return gitRun(args...)
	}
}

func gitRun(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	result := strings.TrimSpace(out.String())
	if errBuf.Len() > 0 {
		msg := strings.TrimSpace(errBuf.String())
		if result == "" {
			result = msg
		} else {
			result += "\n" + msg
		}
	}
	if err != nil {
		return result, fmt.Errorf("git %s: %s", strings.Join(args, " "), truncateErr(result))
	}
	return result, nil
}

// ---------- Search Tool ----------

func searchTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("search: pattern required (optional: path, --regex, --fuzzy, --name, --symbol)")
	}
	pattern := args[0]
	root := "."
	var isRegex, fuzzy, byName, bySymbol bool
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--regex":
			isRegex = true
		case "--fuzzy":
			fuzzy = true
		case "--name":
			byName = true
		case "--symbol":
			bySymbol = true
		default:
			if !strings.HasPrefix(args[i], "--") {
				root = args[i]
			}
		}
	}

	if byName {
		return searchFilenames(root, pattern)
	}
	if bySymbol {
		return searchSymbols(root, pattern)
	}
	if fuzzy {
		return searchFuzzy(root, pattern)
	}
	return searchText(root, pattern, isRegex)
}

func searchText(root, pattern string, isRegex bool) (string, error) {
	re, err := compileRegex(pattern)
	if err != nil {
		if isRegex {
			return "", fmt.Errorf("invalid regex: %w", err)
		}
		re = nil
	}
	var matches []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if skipDirOrFile(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			hit := false
			if re != nil {
				hit = re.MatchString(line)
			} else {
				hit = strings.Contains(line, pattern)
			}
			if hit {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(truncateErr(line))))
			}
		}
		return nil
	})
	if len(matches) == 0 {
		return "no matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

func searchFilenames(root, pattern string) (string, error) {
	var matches []string
	isGlob := strings.ContainsAny(pattern, "*?[")
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirOrFile(path) {
				return filepath.SkipDir
			}
			return nil
		}
		hit := false
		if isGlob {
			if ok, _ := filepath.Match(pattern, info.Name()); ok {
				hit = true
			}
		} else if strings.Contains(strings.ToLower(info.Name()), strings.ToLower(pattern)) {
			hit = true
		}
		if hit {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 0 {
		return "no filename matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

func searchFuzzy(root, pattern string) (string, error) {
	lower := strings.ToLower(pattern)
	var matches []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if skipDirOrFile(path) {
			return nil
		}
		if fuzzyMatch(strings.ToLower(path), lower) {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 0 {
		return "no fuzzy matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

func fuzzyMatch(text, pattern string) bool {
	ti, pi := 0, 0
	for pi < len(pattern) && ti < len(text) {
		if pattern[pi] == text[ti] {
			pi++
		}
		ti++
	}
	return pi == len(pattern)
}

// searchSymbols uses the regex-based symbol indexer on the repo.
func searchSymbols(root, pattern string) (string, error) {
	idx := newSymbolIndexer(root)
	if err := idx.IndexDirectory(root, nil); err != nil {
		return "", err
	}
	var out []string
	for _, s := range idx.Graph().GetAllSymbols() {
		if strings.Contains(strings.ToLower(s.Name), strings.ToLower(pattern)) {
			out = append(out, fmt.Sprintf("%s:%d: %s %s", s.FilePath, s.Line, s.Kind, s.Name))
		}
	}
	if len(out) == 0 {
		return "no symbol matches", nil
	}
	return strings.Join(out, "\n"), nil
}

func skipDirOrFile(path string) bool {
	name := filepath.Base(path)
	if name == ".git" || name == "node_modules" || name == "vendor" ||
		name == "dist" || name == "build" || name == "target" ||
		name == ".cache" || name == ".delta" || name == "delta.exe" {
		return true
	}
	return false
}
