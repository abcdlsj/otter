# Agentic Go

An AI Agent assistant implemented in Go.

## Core Philosophy

- **Minimalist Aesthetics**: Clean and elegant code, hack when you can, avoid over-engineering
- **Make It Work First**: MVP first, iterate and optimize later

## Code Style

Rob Pike's style Go code:

- Short naming: `i`, `s *Session`
- Use interface sparingly, only when polymorphism is needed
- Composition over inheritance
- Handle errors immediately: `if err != nil { return err }`
- Avoid over-abstraction
- Small functions, fit on one screen
- Names are documentation, only comment complex algorithms

## Architecture

```
main.go                 # Entry point
internal/
  ├── agent/            # ReAct loop - orchestrates LLM and tools
  ├── config/           # Configuration management
  ├── event/            # Event system for async communication
  ├── llm/              # LLM clients (anthropic, openai)
  ├── msg/              # Message model and session persistence
  ├── tui/              # Terminal UI (bubbletea)
  └── tool/             # Builtin tools (shell, file)
```

## Core Concepts

### Message
Unified message model with session persistence (jsonl format)

### Agent
ReAct loop: LLM → Tool Call → Execute → LLM → ... → Response

### Tool
Builtin tools:
- `shell` - Execute shell commands
- `file` - Read/write/list/search files

### TUI
Single terminal interface using bubbletea with:
- Streaming/non-streaming modes (Ctrl+S toggle)
- Session management (/new, /clear)
- Tool execution visualization

## Configuration

Config file at `~/.config/otter/config.toml`:

```toml
provider = "anthropic"
model = "claude-sonnet-4-20250514"
api_key = "your-api-key"
stream = false
max_steps = 100
```

Sessions are persisted to `~/.config/otter/sessions/*.jsonl`
