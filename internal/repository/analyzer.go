package repository

import (
	"os"
	"path/filepath"
	"strings"
)

type ProjectInfo struct {
	ProjectType string   `json:"project_type"`
	Languages   []string `json:"languages"`
	FileCount   int      `json:"file_count"`
	EntryPoints []string `json:"entry_points"`
	Frameworks  []string `json:"frameworks"`
	HasGit      bool     `json:"has_git"`
	HasDocker   bool     `json:"has_docker"`
}

type Analyzer struct {
	root string
}

func NewAnalyzer(root string) *Analyzer {
	if root == "" {
		root = "."
	}
	return &Analyzer{root: root}
}

func (a *Analyzer) Analyze() *ProjectInfo {
	info := &ProjectInfo{
		EntryPoints: []string{},
		Frameworks:  []string{},
		Languages:   []string{},
	}

	a.scanDir(a.root, info)

	if _, err := os.Stat(filepath.Join(a.root, ".git")); err == nil {
		info.HasGit = true
	}
	if _, err := os.Stat(filepath.Join(a.root, "Dockerfile")); err == nil {
		info.HasDocker = true
	} else if _, err := os.Stat(filepath.Join(a.root, "docker-compose.yml")); err == nil {
		info.HasDocker = true
	}

	info.ProjectType = a.detectType(info)
	info.EntryPoints = a.findEntryPoints(info)

	return info
}

func (a *Analyzer) scanDir(dir string, info *ProjectInfo) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == ".git" {
			continue
		}

		path := filepath.Join(dir, name)
		if entry.IsDir() {
			a.scanDir(path, info)
			continue
		}

		info.FileCount++
		ext := filepath.Ext(name)
		lang := extToLang(ext)
		if lang != "" && !contains(info.Languages, lang) {
			info.Languages = append(info.Languages, lang)
		}

		if name == "go.mod" {
			info.Frameworks = append(info.Frameworks, "go-modules")
		}
		if name == "package.json" {
			info.Frameworks = append(info.Frameworks, "npm")
		}
		if name == "Cargo.toml" {
			info.Frameworks = append(info.Frameworks, "cargo")
		}
		if name == "requirements.txt" || name == "pyproject.toml" {
			info.Frameworks = append(info.Frameworks, "python")
		}
		if name == "Gemfile" {
			info.Frameworks = append(info.Frameworks, "bundler")
		}
		if name == "composer.json" {
			info.Frameworks = append(info.Frameworks, "composer")
		}
	}
}

func (a *Analyzer) detectType(info *ProjectInfo) string {
	for _, f := range info.Frameworks {
		switch f {
		case "go-modules":
			return "Go"
		case "npm":
			return "Node.js"
		case "cargo":
			return "Rust"
		case "python":
			return "Python"
		case "bundler":
			return "Ruby"
		case "composer":
			return "PHP"
		}
	}
	for _, l := range info.Languages {
		switch l {
		case "Go":
			return "Go"
		case "Python":
			return "Python"
		case "JavaScript", "TypeScript":
			return "Node.js"
		case "Rust":
			return "Rust"
		case "Ruby":
			return "Ruby"
		case "PHP":
			return "PHP"
		case "Java":
			return "Java"
		}
	}
	return "Unknown"
}

func (a *Analyzer) findEntryPoints(info *ProjectInfo) []string {
	var entries []string

	switch info.ProjectType {
	case "Go":
		if _, err := os.Stat(filepath.Join(a.root, "main.go")); err == nil {
			entries = append(entries, "main.go")
		} else if _, err := os.Stat(filepath.Join(a.root, "cmd")); err == nil {
			entries = append(entries, "cmd/")
		}
	case "Node.js":
		if _, err := os.Stat(filepath.Join(a.root, "index.js")); err == nil {
			entries = append(entries, "index.js")
		} else if _, err := os.Stat(filepath.Join(a.root, "index.ts")); err == nil {
			entries = append(entries, "index.ts")
		}
	case "Python":
		if _, err := os.Stat(filepath.Join(a.root, "main.py")); err == nil {
			entries = append(entries, "main.py")
		} else if _, err := os.Stat(filepath.Join(a.root, "app.py")); err == nil {
			entries = append(entries, "app.py")
		}
	}

	if len(entries) == 0 {
		entries = append(entries, "N/A")
	}
	return entries
}

func extToLang(ext string) string {
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc":
		return "C++"
	case ".cs":
		return "C#"
	case ".swift":
		return "Swift"
	case ".kt", ".kts":
		return "Kotlin"
	case ".scala":
		return "Scala"
	case ".html", ".htm":
		return "HTML"
	case ".css", ".scss", ".less":
		return "CSS"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".md":
		return "Markdown"
	case ".sql":
		return "SQL"
	case ".sh", ".bash":
		return "Shell"
	case ".ps1":
		return "PowerShell"
	default:
		return ""
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
