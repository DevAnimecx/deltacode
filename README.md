# Δ Delta Code

<div align="center">

### *The Self-Evolving BYOK Coding Agent*

**Code less. Build anything. Own your models.**

[![Go Report](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](CONTRIBUTING.md)

</div>

---

## What is Delta Code?

Delta Code is a **self-improving AI software engineer** that runs from your terminal. Connect **any provider**, **any model**, **any API key** — no vendor lock-in.

Unlike Claude Code, Codex CLI, or Aider, Delta Code:
- **Routes tasks** to the best model automatically
- **Splits work** across multiple models simultaneously (Fusion Engine)
- **Learns from every session** — builds reusable skills and memory
- **Creates its own tools** when something doesn't exist
- **Cost-optimizes** every request

---

## Quick Install

### Windows (PowerShell)

```powershell
# 1. Download
git clone https://github.com/DevAnimecx/delta-code.git
cd delta-code

# 2. Build (requires Go 1.21+)
go build -o delta.exe .

# 3. Add to PATH (one-time, as Admin)
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$pwd", "User")

# 4. Restart terminal, then add a provider
delta provider add

# 5. Start coding
delta run "build a rest api in python"
```

### macOS / Linux

```bash
# 1. Download
git clone https://github.com/DevAnimecx/delta-code.git
cd delta-code

# 2. Build
go build -o delta .

# 3. Add to PATH
sudo mv delta /usr/local/bin/

# 4. Add a provider
delta provider add

# 5. Start coding
delta run "build a rest api in python"
```

### Without Go installed

Download pre-built binaries from [Releases](https://github.com/DevAnimecx/delta-code/releases).

---

## First Provider Setup

```bash
delta provider add
```

Follow the interactive prompts:
```
Provider name: openai
Type: 1 (OpenAI)
Base URL: (press Enter for default)
API Key: sk-...
Models: gpt-4o,gpt-4o-mini
```

Or add any provider: Anthropic, Google Gemini, DeepSeek, Ollama (local), or any OpenAI-compatible endpoint.

---

## All Commands (23 total)

### Foundation
| Command | Description |
|---------|-------------|
| `delta init` | Initialize project context |
| `delta provider add\|remove\|list\|verify\|export\|import` | Manage AI providers (BYOK) |
| `delta models list\|sync\|set` | Discover and select models |
| `delta run <prompt>` | Generate code with streaming |
| `delta commit [message]` | AI-powered git commits |
| `delta doctor` | System diagnostics |

### Intelligence
| Command | Description |
|---------|-------------|
| `delta explain <prompt>` | Explain code with full project context |
| `delta review [file]` | AI code review with scoring |
| `delta memory sessions\|search\|decisions` | Persistent project memory |

### Autonomy
| Command | Description |
|---------|-------------|
| `delta fix <bug>` | Autonomous Plan→Write→Run→Fix→Commit loop |
| `delta architect <project>` | Multi-model architecture plans |
| `delta test <desc>` | Auto-generate tests |
| `delta docs <desc>` | Auto-generate documentation |
| `delta benchmark run\|best\|results` | Benchmark every connected model |
| `delta tool create\|list` | Auto-create tools from natural language |

### Polish & Ecosystem
| Command | Description |
|---------|-------------|
| `delta checkpoint save\|undo\|replay\|compare\|branch\|log` | AI Time Machine |
| `delta cost estimate\|best\|models` | Smart cost optimization |
| `delta policy show\|allow\|deny` | Security permissions |
| `delta update` | Self-update |
| `delta pr review <repo> <num>` | GitHub PR review |
| `delta tasks list\|run` | Background jobs |
| `delta` | Launch interactive TUI dashboard |

---

## Architecture

```
CLI
 ↓
Command Engine
 ↓
Planner → Fusion Engine (multi-model DAG)
 ↓
Context Engine (git, README, deps, errors)
 ↓
Memory Engine (SQLite + Vector)
 ↓
Skill Engine (learned workflows)
 ↓
Model Router (task classifier)
 ↓
Provider Layer (OpenAI, Anthropic, Google, DeepSeek, Ollama...)
 ↓
Sandbox (temp directory execution)
 ↓
Filesystem + Git + Response Renderer
```

---

## Why Delta Code?

| Feature | Claude Code | Codex CLI | Aider | **Delta Code** |
|---------|:-----------:|:---------:|:-----:|:--------------:|
| Unlimited BYOK | ❌ | ❌ | ✅ | **✅** |
| Multi-Model Fusion | ❌ | ❌ | ❌ | **✅** |
| Auto Skill Evolution | ❌ | ❌ | ❌ | **✅** |
| Auto Tool Creation | ❌ | ❌ | ❌ | **✅** |
| Live Benchmark Engine | ❌ | ❌ | ❌ | **✅** |
| Cost Optimizer | Limited | Limited | ❌ | **✅** |
| AI Time Machine | ❌ | ❌ | ❌ | **✅** |
| Local Memory Graph | Partial | ❌ | Partial | **✅** |
| Interactive TUI | Limited | Limited | ❌ | **✅** |

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GITHUB_TOKEN` | For `delta pr review` | GitHub API token |

---

## Development

```bash
# Build from source
go build -o delta .

# Run tests
go test ./...

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o delta-linux .
GOOS=darwin GOARCH=arm64 go build -o delta-macos .
GOOS=windows GOARCH=amd64 go build -o delta.exe .
```

---

## License

MIT
