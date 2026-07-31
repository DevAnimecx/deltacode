package tools

import "time"

// builtin registers a built-in tool with the given manifest fields.
func (r *Registry) builtin(id, name, category, desc string, perms []Permission, timeout int, retry int, run ToolFunc) {
	platform := []string{"all"}
	m := Manifest{
		ID:          id,
		Name:        name,
		Version:     "1.0.0",
		Author:      "delta",
		Description: desc,
		Category:    category,
		Permissions: perms,
		Platform:    platform,
		TimeoutSec:  timeout,
		Retry:       retry,
		TrustLevel:  "trusted",
		Source:      "builtin",
		LastUpdated: time.Now(),
	}
	if m.TimeoutSec == 0 {
		m.TimeoutSec = 30
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = &Tool{Manifest: m, Health: "healthy", Run: run}
	r.aliases[id] = name
}

// registerBuiltins wires the 25 built-in tools into the registry.
func (r *Registry) registerBuiltins() {
	r.builtin("delta.fs", "fs", "filesystem",
		"Read, write, list, tree, copy, move, delete, rename and inspect files",
		[]Permission{PermReadFiles, PermWriteFiles}, 30, 0, fsTool)

	r.builtin("delta.edit", "edit", "filesystem",
		"Edit files with plain text or regex replacement",
		[]Permission{PermWriteFiles}, 30, 0, editTool)

	r.builtin("delta.terminal", "terminal", "shell",
		"Run shell commands (powershell on windows, sh on unix)",
		[]Permission{PermExecProcess}, 120, 0, terminalTool)

	r.builtin("delta.git", "git", "version-control",
		"Git operations: status, diff, log, commit, branch, checkout, merge, rebase, push, pull",
		[]Permission{PermGitOps}, 120, 1, gitTool)

	r.builtin("delta.search", "search", "code-intelligence",
		"Search code by text, regex, filename, fuzzy match or symbols",
		[]Permission{PermReadFiles}, 60, 0, searchTool)

	r.builtin("delta.http", "http", "network",
		"Perform HTTP requests (GET, POST, PUT, DELETE, PATCH) with headers",
		[]Permission{PermNetwork}, 60, 1, httpTool)

	r.builtin("delta.websearch", "websearch", "network",
		"Search the web for information and return result summaries",
		[]Permission{PermNetwork}, 60, 1, webSearchTool)

	r.builtin("delta.browser", "browser", "automation",
		"Automate a headless browser (requires playwright)",
		[]Permission{PermBrowser}, 120, 0, browserTool)

	r.builtin("delta.docs", "docs", "network",
		"Read documentation files: TOC, read, links, generate",
		[]Permission{PermNetwork}, 60, 0, docsTool)

	r.builtin("delta.diff", "diff", "code-intelligence",
		"Compute unified diffs between files or against git HEAD",
		[]Permission{PermReadFiles}, 30, 0, diffTool)

	r.builtin("delta.pkg", "pkg", "package-manager",
		"Package manager wrapper: npm, pip, cargo, go, mvn (install/remove/upgrade/audit)",
		[]Permission{PermExecProcess, PermNetwork}, 300, 1, pkgTool)

	r.builtin("delta.repo-intel", "repo-intel", "code-intelligence",
		"Repo intelligence: symbols, call graph, imports, top files",
		[]Permission{PermReadFiles}, 120, 0, repoIntelTool)

	r.builtin("delta.memory", "memory", "knowledge",
		"Persistent project memory: save, get, search, list, expire, pin",
		[]Permission{PermWriteFiles}, 30, 0, memoryTool)

	r.builtin("delta.skill", "skill", "knowledge",
		"Invoke a registered skill by name with arguments",
		[]Permission{PermReadFiles}, 60, 0, skillTool)

	r.builtin("delta.test", "test", "verification",
		"Run test suites (go test, pytest, npm test, jest, cargo test) with coverage",
		[]Permission{PermExecProcess}, 600, 0, testTool)

	r.builtin("delta.debug", "debug", "verification",
		"Analyze stack traces, logs, crashes and runtime state",
		[]Permission{PermReadFiles}, 60, 0, debugTool)

	r.builtin("delta.lint", "lint", "verification",
		"Lint code: go vet, ruff, eslint, clippy (with --fix)",
		[]Permission{PermExecProcess}, 300, 0, lintTool)

	r.builtin("delta.format", "format", "verification",
		"Format code: gofmt, black, prettier, rustfmt, clang-format",
		[]Permission{PermWriteFiles}, 120, 0, formatTool)

	r.builtin("delta.security", "security", "verification",
		"Scan for secrets, run audits and generate SBOMs",
		[]Permission{PermReadFiles, PermNetwork}, 300, 0, securityTool)

	r.builtin("delta.perf", "perf", "performance",
		"Profile and analyze performance: pprof, cProfile, bundle analysis",
		[]Permission{PermExecProcess}, 180, 0, perfTool)

	r.builtin("delta.scaffold", "scaffold", "scaffolding",
		"Generate project skeletons (go-cli, python-cli, node-cli, go-api)",
		[]Permission{PermWriteFiles}, 60, 0, scaffoldTool)

	r.builtin("delta.workflow", "workflow", "automation",
		"Save, list, run, replay and delete multi-step tool workflows",
		[]Permission{PermExecProcess}, 600, 0, r.workflowTool)

	r.builtin("delta.db", "db", "data",
		"Database operations: sqlite query/schema/tables, explain plans",
		[]Permission{PermDatabase}, 120, 0, dbTool)

	r.builtin("delta.docker", "docker", "infrastructure",
		"Docker container operations: build, run, stop, logs, compose",
		[]Permission{PermExecProcess}, 600, 0, dockerTool)

	r.builtin("delta.mcp", "mcp", "integration",
		"MCP server management: health checks and discovery",
		[]Permission{PermNetwork}, 60, 0, mcpTool)
}
