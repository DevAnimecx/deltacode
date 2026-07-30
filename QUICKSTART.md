# Delta Code — Quick Start

## 1. Add to PATH (one-time)

### PowerShell (Run as Administrator)
```powershell
# Add to PATH for current user
$deltaPath = "C:\Users\india\OneDrive\Documents\Delta Code"
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$deltaPath", "User")
```

### Or just use the full path
```powershell
& "C:\Users\india\OneDrive\Documents\Delta Code\delta.exe" <command>
```

---

## 2. First-Time Setup

```powershell
# Check health
delta doctor

# Add your first AI provider (interactive)
delta provider add

# Or quickly set one with values:
delta provider add
# → enter: openai
# → enter: 1
# → enter: (default URL)
# → enter: sk-...
# → enter: gpt-4o,gpt-4o-mini

# Verify it works
delta provider verify openai

# Test with a prompt
delta run "write hello world in python"
```

---

## 3. All Commands

### Phase 1 — Foundation
| Command | Description |
|---------|-------------|
| `delta init` | Initialize project |
| `delta provider add\|remove\|list\|verify\|export\|import` | Manage AI providers |
| `delta models list\|sync\|set` | Manage models |
| `delta run <prompt>` | Generate code with streaming |
| `delta commit [message]` | AI commit message |
| `delta doctor` | System diagnostics |

### Phase 2 — Intelligence
| Command | Description |
|---------|-------------|
| `delta explain <prompt>` | Explain code with full project context |
| `delta review [file]` | AI code review with scoring |
| `delta memory sessions\|search\|decisions` | Persistent project memory |

### Phase 3 — Autonomy
| Command | Description |
|---------|-------------|
| `delta fix <bug>` | Autonomous bug fixing loop |
| `delta architect <project>` | Multi-model architecture plans |
| `delta test <desc>` | Auto-generate tests |
| `delta docs <desc>` | Auto-generate documentation |
| `delta benchmark run\|best\|results` | Model benchmarking |
| `delta tool create\|list` | Auto-create tools |

### Phase 4 — Polish
| Command | Description |
|---------|-------------|
| `delta checkpoint save\|undo\|replay\|compare\|branch\|log` | AI Time Machine |
| `delta cost estimate\|best\|models` | Cost optimization |
| `delta policy show\|allow\|deny` | Security permissions |
| `delta update` | Self-update |
| `delta pr review <repo> <num>` | GitHub PR review |
| `delta tasks list\|run` | Background jobs |
| `delta` | Launch TUI Dashboard |

---

## 4. Environment Variables

| Variable | Required? | Description |
|----------|-----------|-------------|
| `GITHUB_TOKEN` | For `delta pr review` | GitHub API token |

---

## 5. Quick Examples

```powershell
# Generate code
delta run "build a rest api in python with flask"

# Autonomous fix
delta fix "the sort function returns wrong order"

# Architecture with multi-model orchestration
delta architect "build a spotify clone with react and node"

# Review current changes
delta review

# Full autonomous build
delta run "create a todo app with react frontend and express backend"
```
