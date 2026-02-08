# Prompt Customization

Otter now supports customizable system prompts via templates. You can:

1. Use the built-in default prompt
2. Create custom prompt files
3. Define different prompts for different agent modes
4. Use Go template syntax for dynamic content

## Configuration

Add a `[prompt]` section to your `~/.config/otter/config.toml`:

```toml
[prompt]
# Path to a custom template file (optional)
template_path = "~/.config/otter/prompts/custom.md"

# Or define the template inline (optional)
template = """
You are a helpful assistant running in {{.WorkingDir}}.

Available tools:
{{range .Tools}}- {{.Name}}: {{.Desc}}
{{end}}
"""

# Mode-specific prompts
[prompt.modes.plan]
template_path = "~/.config/otter/prompts/plan.md"

[prompt.modes.explore]
template = """
You are in exploration mode. Be concise.
"""
```

## Template Variables

The following variables are available in your templates:

| Variable | Type | Description |
|----------|------|-------------|
| `{{.WorkingDir}}` | string | Current working directory |
| `{{.OS}}` | string | Operating system (linux, darwin, windows) |
| `{{.Date}}` | string | Current date (YYYY-MM-DD) |
| `{{.Tools}}` | []ToolInfo | List of available tools |
| `{{.MaxSteps}}` | int | Maximum agent steps |

### ToolInfo Fields

Each tool in `{{.Tools}}` has:
- `{{.Name}}` - Tool name
- `{{.Desc}}` - Tool description

## Template Functions

Additional functions available:

- `{{join .Slice ", "}}` - Join a slice with a separator
- `{{now}}` - Get current time
- `{{env "HOME"}}` - Get environment variable

## Agent Modes

You can create specialized prompts for different modes:

### Built-in Modes

- `default` - Standard coding assistant
- `plan` - Read-only exploration for planning
- `explore` - Quick code search and understanding

### Custom Modes

Define your own modes in config:

```toml
[prompt.modes.review]
template_path = "~/.config/otter/prompts/review.md"
```

Then use them in your application code:

```go
agent := NewWithMode(llm, tools, "review")
```

Or switch modes dynamically:

```go
agent.SetMode("plan")
```

## Example Custom Prompt

Create `~/.config/otter/prompts/custom.md`:

```markdown
You are {{env "USER"}}'s personal coding assistant.

Environment:
- Working in: {{.WorkingDir}}
- OS: {{.OS}}
- Date: {{.Date}}

Tools available:
{{range .Tools}}- {{.Name}}: {{.Desc}}
{{end}}

Instructions:
1. Always use the file tool to read before writing
2. Prefer grep for searching code
3. Be concise and direct
```

## Migration from Hardcoded Prompts

If you were previously modifying `agent.go` to change prompts, you can now:

1. Copy the default template from `internal/prompt/prompt.go`
2. Save it to a file (e.g., `~/.config/otter/prompts/my-prompt.md`)
3. Reference it in your config:

```toml
[prompt]
template_path = "~/.config/otter/prompts/my-prompt.md"
```

No more code modifications needed!
