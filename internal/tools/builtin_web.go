package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------- HTTP/API Tool ----------

func httpTool(args ...string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("http: method and url required (optional: body, --header K=V)")
	}
	method := strings.ToUpper(args[0])
	url := args[1]
	var body string
	var headers []string
	for i := 2; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--header") && i+1 < len(args) {
			headers = append(headers, args[i+1])
			i++
			continue
		}
		if body == "" {
			body = strings.Join(args[i:], " ")
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		kv := strings.SplitN(h, "=", 2)
		if len(kv) == 2 {
			req.Header.Set(kv[0], kv[1])
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// Pretty-print JSON when possible.
	content := string(data)
	if json.Valid(data) {
		var pretty bytes.Buffer
		if json.Indent(&pretty, data, "", "  ") == nil {
			content = pretty.String()
		}
	}
	return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, truncateErr(content)), nil
}

// ---------- Web Search Tool ----------

func webSearchTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("websearch: query required")
	}
	query := strings.Join(args, " ")
	return searchDuckDuckGo(query)
}

func searchDuckDuckGo(query string) (string, error) {
	req, err := http.NewRequest("GET", "https://html.duckduckgo.com/html/?q="+urlEncode(query), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DeltaCode/0.2.6)")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web search unavailable: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return extractDDGResults(string(data), 8), nil
}

func urlEncode(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "&", "%26"), "?", "%3F")
}

func extractDDGResults(html string, limit int) string {
	var results []string
	// Simple extraction of result titles/links from the HTML page.
	for _, line := range strings.Split(html, "\n") {
		if !strings.Contains(line, "result__a") {
			continue
		}
		title := stripTags(line)
		if title == "" {
			continue
		}
		href := ""
		if i := strings.Index(line, "href=\""); i != -1 {
			rest := line[i+len("href=\""):]
			if j := strings.Index(rest, "\""); j != -1 {
				href = rest[:j]
			}
		}
		results = append(results, fmt.Sprintf("%s\n  %s", title, href))
		if len(results) >= limit {
			break
		}
	}
	if len(results) == 0 {
		return "no results"
	}
	return strings.Join(results, "\n\n")
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// ---------- Browser Tool (health-checked stub) ----------

func browserTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("browser: action required (open|navigate|click|fill|submit|screenshot|text)")
	}
	return "", fmt.Errorf("browser tool requires Playwright; run `npm i -g playwright` and `playwright install chromium` to enable")
}

// ---------- Docs Tool ----------

func docsTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("docs: action required (read|toc|links|generate)")
	}
	switch args[0] {
	case "read":
		if len(args) < 2 {
			return "", fmt.Errorf("docs read: path required")
		}
		data, err := readAny(args[1])
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "toc":
		if len(args) < 2 {
			return "", fmt.Errorf("docs toc: path required")
		}
		data, err := osReadFile(args[1])
		if err != nil {
			return "", err
		}
		return markdownTOC(string(data)), nil
	case "links":
		if len(args) < 2 {
			return "", fmt.Errorf("docs links: path required")
		}
		return validateMarkdownLinks(args[1])
	case "generate":
		return "", fmt.Errorf("docs generate: use the DocWriter agent for doc generation")
	default:
		return "", fmt.Errorf("docs: unknown action %q", args[0])
	}
}

// ---------- Diff Tool ----------

func diffTool(args ...string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("diff: file_a and file_b required (or file_a with --against-git)")
	}
	a := args[0]
	b := args[1]
	if b == "--against-git" || b == "-g" {
		out, err := gitRun("diff", "--no-color", "--", a)
		return out, err
	}
	dataA, err := osReadFile(a)
	if err != nil {
		return "", err
	}
	dataB, err := osReadFile(b)
	if err != nil {
		return "", err
	}
	return unifiedDiff(a, b, string(dataA), string(dataB)), nil
}

func unifiedDiff(nameA, nameB, a, b string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")
	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", nameA, nameB))
	maxLen := len(aLines)
	if len(bLines) > maxLen {
		maxLen = len(bLines)
	}
	for i := 0; i < maxLen; i++ {
		hasA := i < len(aLines)
		hasB := i < len(bLines)
		if hasA && hasB {
			if aLines[i] == bLines[i] {
				out.WriteString(" " + aLines[i] + "\n")
			} else {
				out.WriteString("-" + aLines[i] + "\n")
				out.WriteString("+" + bLines[i] + "\n")
			}
			continue
		}
		if hasA {
			out.WriteString("-" + aLines[i] + "\n")
		}
		if hasB {
			out.WriteString("+" + bLines[i] + "\n")
		}
	}
	return out.String()
}

func markdownTOC(md string) string {
	var toc []string
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		level := 0
		for level < 6 && level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level == 0 {
			continue
		}
		title := strings.TrimSpace(trimmed[level:])
		if title == "" {
			continue
		}
		toc = append(toc, fmt.Sprintf("%s- %s", strings.Repeat("  ", level-1), title))
	}
	if len(toc) == 0 {
		return "no headings"
	}
	return strings.Join(toc, "\n")
}

func validateMarkdownLinks(path string) (string, error) {
	data, err := osReadFile(path)
	if err != nil {
		return "", err
	}
	md := string(data)
	baseDir := dirOf(path)
	var issues []string
	for _, line := range strings.Split(md, "\n") {
		rest := line
		for {
			open := strings.Index(rest, "](")
			if open == -1 {
				break
			}
			start := open + 2
			end := strings.Index(rest[start:], ")")
			if end == -1 {
				break
			}
			link := rest[start : start+end]
			rest = rest[start+end+1:]
			if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") ||
				strings.HasPrefix(link, "#") || strings.HasPrefix(link, "mailto:") {
				continue
			}
			clean := strings.SplitN(link, "#", 2)[0]
			if clean == "" {
				continue
			}
			target := joinPath(baseDir, clean)
			if !fileExists(target) {
				issues = append(issues, fmt.Sprintf("broken link: %s (in %s)", link, path))
			}
		}
	}
	if len(issues) == 0 {
		return "all local links valid", nil
	}
	return strings.Join(issues, "\n"), nil
}

// ---------- Package Manager Tool ----------

func pkgTool(args ...string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("pkg: manager and action required, e.g. pkg npm install lodash | pkg pip install requests | pkg go install ./... | pkg cargo build")
	}
	manager := args[0]
	action := args[1]
	rest := args[2:]

	managers := map[string]string{
		"npm": "npm", "pnpm": "pnpm", "yarn": "yarn", "bun": "bun",
		"pip": "pip", "uv": "uv", "cargo": "cargo", "go": "go",
		"composer": "composer", "maven": "mvn", "gradle": "gradle",
	}
	bin, ok := managers[manager]
	if !ok {
		return "", fmt.Errorf("pkg: unsupported manager %q", manager)
	}

	var sub []string
	switch action {
	case "install", "add", "get":
		switch manager {
		case "go":
			sub = append([]string{"get"}, rest...)
		case "cargo":
			sub = append([]string{"add"}, rest...)
		default:
			sub = append([]string{"install"}, rest...)
		}
	case "remove", "uninstall", "rm":
		switch manager {
		case "go":
			return "", fmt.Errorf("pkg: go has no remove; remove imports manually")
		case "cargo":
			sub = append([]string{"remove"}, rest...)
		default:
			sub = append([]string{"uninstall"}, rest...)
		}
	case "upgrade", "update":
		switch manager {
		case "go":
			sub = append([]string{"get", "-u"}, rest...)
		case "cargo":
			sub = append([]string{"update"}, rest...)
		default:
			sub = append([]string{"update"}, rest...)
		}
	case "audit":
		if manager == "go" {
			return "", fmt.Errorf("pkg: go audit not available; use govulncheck")
		}
		sub = append([]string{"audit"}, rest...)
	case "verify", "check":
		switch manager {
		case "go":
			sub = append([]string{"mod", "verify"}, rest...)
		case "cargo":
			sub = append([]string{"check"}, rest...)
		default:
			sub = append([]string{"ls"}, rest...)
		}
	default:
		sub = append([]string{action}, rest...)
	}
	return runExternal(bin, sub, 180)
}
