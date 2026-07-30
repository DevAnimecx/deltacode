# Δ Delta Code

### *The Self-Evolving BYOK Coding Agent*

**Code less. Build anything. Own your models.**

[![Go](https://img.shields.io/badge/Go-1.21%2B-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)]()

---

## 🚀 One-Click Install

### Windows

```powershell
# Clone and run (auto-builds on first launch)
git clone https://github.com/DevAnimecx/deltacode.git
cd deltacode
.\run.ps1
```

**OR** use `delta.bat` from CMD:
```cmd
git clone https://github.com/DevAnimecx/deltacode.git
cd deltacode
delta.bat
```

First run automatically launches the **interactive setup wizard** — just pick your AI provider and paste your API key.

### macOS / Linux

```bash
git clone https://github.com/DevAnimecx/deltacode.git
cd deltacode
go build -o delta .
./delta
```

### Quick Add to PATH (Windows)

```powershell
# Run as Administrator
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\deltacode", "User")
```

---

## 🎯 First Run

When you run `delta` for the first time (no arguments), it launches a **beautiful setup wizard**:

```
      ██████╗ ███████╗██╗  ████████╗ █████╗
      ██╔══██╗██╔════╝██║  ╚══██╔══╝██╔══██╗
      ██║  ██║█████╗  ██║     ██║   ███████║
      ██║  ██║██╔══╝  ██║     ██║   ██╔══██║
      ██████╔╝███████╗███████╗██║   ██║  ██║
      ╚═════╝ ╚══════╝╚══════╝╚═╝   ╚═╝  ╚═╝

     Welcome to Δ Delta Code
     The Self-Evolving BYOK Coding Agent
```

1.  **Choose** your AI provider (OpenAI, Anthropic, Google, DeepSeek, Ollama, or custom)
2.  **Paste** your API key
3.  **Test** the connection automatically
4.  **Start coding** immediately

---

## 📋 All Commands

```
Δ Delta Code — 23 commands

FOUNDATION:
  delta                        Launch TUI dashboard or setup wizard
  delta init                   Initialize project context
  delta provider add|list      Manage AI providers (BYOK)
  delta models list|set        Discover and select models
  delta run "<prompt>"         Generate code with streaming
  delta commit [message]       AI-powered git commits
  delta doctor                 System diagnostics

INTELLIGENCE:
  delta explain "<prompt>"     Explain code with full project context
  delta review [file]          AI code review with severity scoring
  delta memory search|sessions Persistent project memory (SQLite + Vector)

AUTONOMY:
  delta fix "<bug>"            Autonomous Plan→Write→Run→Fix loop
  delta architect "<project>"  Multi-model architecture plans
  delta test "<desc>"          Auto-generate tests
  delta docs "<desc>"          Auto-generate documentation
  delta benchmark run|results  Benchmark every connected model (6 task types)

POLISH:
  delta checkpoint save|undo   AI Time Machine — snapshots, rollback, replay
  delta cost estimate|best     Smart cost optimization engine
  delta policy allow|deny      Security permissions and secret vault
  delta tool create|list       Auto-create tools from natural language
  delta pr review <repo> <num> GitHub pull request review
  delta tasks list|run         Long-running background jobs
  delta update                 Self-update from GitHub
```

---

## ⚡ Quick Examples

```bash
# Generate code
delta run "build a Flask REST API with SQLite"

# Autonomous bug fixing
delta fix "the sort function returns wrong order"

# Full multi-model architecture
delta architect "build a Spotify clone with React + Node"

# Code review of current changes
delta review

# Save a checkpoint before risky changes
delta checkpoint save "before refactor"

# Estimate cost
delta cost estimate gpt-4o 1000 500
```

---

## 🧠 Architecture

```
     CLI / TUI (Bubble Tea)
           │
     ┌─────┴─────┐
     │   Engine  │
     └─────┬─────┘
           │
     ┌─────┼─────┬──────┬──────┬──────┐
     │     │     │      │      │      │
   Context Fusion Memory Router Skills
           │
     ┌─────┴─────┐
     │ Providers │  ← OpenAI, Anthropic, Google, DeepSeek, Ollama...
     └───────────┘
```

---

## 🔧 Tech Stack

- **Language:** Go 1.21+
- **CLI Framework:** Cobra + Bubble Tea
- **Storage:** SQLite (modernc.org) + JSON vector memory
- **Encryption:** AES-256-GCM for API keys
- **Streaming:** SSE for real-time model output

---

## 📄 License

MIT — Use it. Modify it. Ship it. Own your models.
