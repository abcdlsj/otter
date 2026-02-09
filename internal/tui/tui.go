package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/abcdlsj/otter/internal/agent"
	"github.com/abcdlsj/otter/internal/config"
	"github.com/abcdlsj/otter/internal/event"
	"github.com/abcdlsj/otter/internal/llm"
	"github.com/abcdlsj/otter/internal/logger"
	"github.com/abcdlsj/otter/internal/msg"
	"github.com/abcdlsj/otter/internal/tool"
)

const maxToolResultLines = 8

type message struct {
	role    string
	content string
	args    string
}

type Model struct {
	agent    *agent.Agent
	tools    *tool.Set
	bus      *msg.Bus
	input    textarea.Model
	viewport viewport.Model
	spinner  spinner.Model
	messages []message

	inputTokens  int64
	outputTokens int64

	sessionsDir string
	session     string
	thinking    bool
	toolName    string
	autoScroll  bool
	cancel      context.CancelFunc
	events      <-chan event.Event

	mdRenderer *glamour.TermRenderer

	width  int
	height int
	ready  bool
}

func New(a *agent.Agent, t *tool.Set, b *msg.Bus) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask anything..."
	ta.Blur()
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.Prompt = ""

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#888A85"))

	return Model{
		agent:       a,
		tools:       t,
		bus:         b,
		input:       ta,
		spinner:     sp,
		sessionsDir: config.SessionsDir(),
		session:     newSessionID(),
		autoScroll:  true,
	}
}

func newSessionID() string {
	now := time.Now()
	return fmt.Sprintf("%s_%03d", now.Format("20060102_150405"), now.Nanosecond()/1000000)
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

type eventMsg event.Event
type titleMsg struct{}

func generateTitleCmd(a *agent.Agent, bus *msg.Bus, sid string, lg logger.Logger, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		title, err := a.GenerateTitle(ctx, lg, text)
		if err != nil {
			lg.Warn("generate title failed", "err", err)
			return nil
		}
		if title != "" {
			bus.SetSessionTitle(sid, title)
		}
		return titleMsg{}
	}
}

func waitForEvent(ch <-chan event.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			// Channel closed, agent finished
			return nil
		}
		return eventMsg(ev)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.thinking && m.cancel != nil {
				m.cancel()
				m.thinking = false
				m.toolName = ""
				return m, nil
			}
			return m, tea.Quit

		case "enter":
			if m.thinking {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			return m.send(text)

		case "ctrl+j":
			if !m.thinking {
				m.input.InsertString("\n")
			}
			return m, nil

		case "ctrl+s":
			config.C.Stream = !config.C.Stream
			if err := config.Save(); err == nil {
				mode := "OFF"
				if config.C.Stream {
					mode = "ON"
				}
				m.messages = append(m.messages, message{role: "system", content: fmt.Sprintf("Stream mode: %s", mode)})
				m.updateViewport()
			}
			return m, nil

		case "pgup", "pgdown", "up", "down":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.autoScroll = m.viewport.AtBottom()
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.autoScroll = m.viewport.AtBottom()
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1
		footerHeight := 2
		inputLines := min(strings.Count(m.input.Value(), "\n")+1, 5)
		inputHeight := inputLines + 2
		viewportHeight := m.height - headerHeight - inputHeight - footerHeight - 1

		if !m.ready {
			m.viewport = viewport.New(m.width, viewportHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.MouseWheelEnabled = true
			m.ready = true
			cmds = append(cmds, m.input.Focus())
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = viewportHeight
		}
		m.input.SetWidth(m.width - 4)
		m.input.SetHeight(inputLines)
		m.initMarkdownRenderer()
		m.updateViewport()

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case titleMsg:
		return m, nil

	case eventMsg:
		return m.handleEvent(event.Event(msg))
	}

	if !m.thinking && m.ready {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) send(text string) (tea.Model, tea.Cmd) {
	if cmd, handled := m.handleCommand(text); handled {
		return m, cmd
	}

	m.messages = append(m.messages, message{role: "user", content: text})
	m.input.Reset()
	m.thinking = true
	m.autoScroll = true
	m.updateViewport()

	session := m.bus.GetOrCreateSession(m.session)
	isFirstMessage := len(session.Messages) == 0

	history := msgsToLLM(session.Messages)

	m.bus.Pub(msg.User(m.session, text))

	lg := logger.NewFileLogger(logger.SessionLogDir(m.sessionsDir, m.session))

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	rawEvents := m.agent.Run(ctx, lg, history, text)
	m.events = m.bus.HandleEvents(m.session, rawEvents)

	cmds := []tea.Cmd{m.spinner.Tick, waitForEvent(m.events)}
	if isFirstMessage {
		cmds = append(cmds, generateTitleCmd(m.agent, m.bus, m.session, lg, text))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleCommand(text string) (tea.Cmd, bool) {
	parts := strings.Fields(text)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "/") {
		return nil, false
	}

	switch parts[0] {
	case "/new":
		m.session = newSessionID()
		m.messages = nil
		m.input.Reset()
		m.updateViewport()
		return nil, true

	case "/clear":
		m.messages = nil
		m.input.Reset()
		m.updateViewport()
		return nil, true

	case "/models":
		models := config.C.ListModels()
		content := "Available models:\n"
		for _, model := range models {
			marker := "  "
			if model == config.C.CurrentProviderName()+"/"+config.C.CurrentModelName() {
				marker = "* "
			}
			content += marker + model + "\n"
		}
		m.messages = append(m.messages, message{role: "system", content: content})
		m.input.Reset()
		m.updateViewport()
		return nil, true

	case "/model":
		if len(parts) < 2 {
			m.messages = append(m.messages, message{
				role:    "system",
				content: "Usage: /model <provider>/<model> or /model <model-alias>",
			})
			m.input.Reset()
			m.updateViewport()
			return nil, true
		}

		// Parse provider/model
		var providerName, modelName string
		if strings.Contains(parts[1], "/") {
			p := strings.SplitN(parts[1], "/", 2)
			providerName = p[0]
			modelName = p[1]
		} else {
			// Try to find by alias
			found := false
			for _, p := range config.C.Providers {
				for _, m := range p.Models {
					if m.Alias == parts[1] || m.Name == parts[1] {
						providerName = p.Name
						modelName = m.Name
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				m.messages = append(m.messages, message{
					role:    "error",
					content: fmt.Sprintf("Model '%s' not found. Use /models to list available models.", parts[1]),
				})
				m.input.Reset()
				m.updateViewport()
				return nil, true
			}
		}

		if config.C.SetModel(providerName, modelName) {
			// Recreate LLM client
			newLLM, err := llm.New()
			if err != nil {
				m.messages = append(m.messages, message{
					role:    "error",
					content: fmt.Sprintf("Failed to switch model: %v", err),
				})
			} else {
				m.agent = agent.New(newLLM, m.tools)
				m.messages = append(m.messages, message{
					role:    "system",
					content: fmt.Sprintf("Switched to %s/%s", providerName, config.C.CurrentModelName()),
				})
				// Save config to persist the model change
				if err := config.Save(); err != nil {
					m.messages = append(m.messages, message{
						role:    "error",
						content: fmt.Sprintf("Failed to save config: %v", err),
					})
				}
			}
		} else {
			m.messages = append(m.messages, message{
				role:    "error",
				content: fmt.Sprintf("Model '%s/%s' not found. Use /models to list available models.", providerName, modelName),
			})
		}
		m.input.Reset()
		m.updateViewport()
		return nil, true

	case "/sessions":
		sessions := m.bus.ListSessions()
		content := "Sessions:\n"
		for _, s := range sessions {
			marker := "  "
			if s.ID == m.session {
				marker = "* "
			}
			content += marker + s.ID + " - " + s.Title + "\n"
		}
		if len(sessions) == 0 {
			content = "No sessions found."
		}
		m.messages = append(m.messages, message{role: "system", content: content})
		m.input.Reset()
		m.updateViewport()
		return nil, true

	case "/switch":
		if len(parts) < 2 {
			m.messages = append(m.messages, message{
				role:    "system",
				content: "Usage: /switch <session_id>",
			})
			m.input.Reset()
			m.updateViewport()
			return nil, true
		}
		targetID := parts[1]
		session := m.bus.GetSession(targetID)
		if session == nil {
			m.messages = append(m.messages, message{
				role:    "error",
				content: fmt.Sprintf("Session '%s' not found. Use /sessions to list available sessions.", targetID),
			})
			m.input.Reset()
			m.updateViewport()
			return nil, true
		}
		m.session = targetID
		m.messages = nil
		for _, msg := range session.Messages {
			m.messages = append(m.messages, message{role: msg.Role, content: msg.Text})
		}
		m.input.Reset()
		m.updateViewport()
		return nil, true

	case "/compact":
		// Get current session and check if there are messages to compact
		session := m.bus.GetSession(m.session)
		if session == nil || len(session.Messages) < 10 {
			m.messages = append(m.messages, message{
				role:    "system",
				content: "Not enough messages to compact (need at least 10).",
			})
			m.input.Reset()
			m.updateViewport()
			return nil, true
		}

		// Show starting message
		m.messages = append(m.messages, message{
			role:    "compact:start",
			content: fmt.Sprintf("compacting %d messages", len(session.Messages)),
		})
		m.input.Reset()
		m.updateViewport()

		// Trigger compact immediately in a goroutine
		go func() {
			lg := logger.NewFileLogger(logger.SessionLogDir(m.sessionsDir, m.session))
			success, before, after := m.agent.CompactNow(context.Background(), lg, session.Messages)
			if success {
				m.bus.Pub(msg.New(m.session, "system", fmt.Sprintf("Compacted: %d → %d tokens", before, after)))
			} else {
				m.bus.Pub(msg.New(m.session, "system", "Compact failed or not needed (below threshold)"))
			}
		}()
		return nil, true

	case "/help":
		help := `Commands:
  /new      Create new session
  /clear    Clear messages
  /sessions List all sessions
  /switch   Switch session (e.g., /switch <session_id>)
  /models   List available models
  /model    Switch model (e.g., /model kimi/kimi-k2.5)
  /compact  Compact conversation history to reduce tokens
  /help     Show this help

Shortcuts:
   Enter    Send message
   Ctrl+J   New line
   Ctrl+C   Quit`
		m.messages = append(m.messages, message{role: "system", content: help})
		m.input.Reset()
		m.updateViewport()
		return nil, true
	}

	return nil, false
}

func (m Model) handleEvent(ev event.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case event.ToolStart:
		if data, ok := ev.Data.(event.ToolStartData); ok {
			m.toolName = data.Name
			m.messages = append(m.messages, message{
				role:    "tool:start:" + data.Name,
				content: "",
				args:    data.Args,
			})
			m.updateViewport()
		}
		return m, tea.Batch(m.spinner.Tick, waitForEvent(m.events))

	case event.ToolEnd:
		if data, ok := ev.Data.(event.ToolEndData); ok {
			role := "tool:end:" + data.Name
			if data.Error != "" {
				role = "tool:error:" + data.Name
			}
			result := data.Result
			if data.Error != "" {
				result = data.Error
			}
			var args string
			for i := len(m.messages) - 1; i >= 0; i-- {
				if m.messages[i].role == "tool:start:"+data.Name {
					args = m.messages[i].args
					break
				}
			}
			m.messages = append(m.messages, message{role: role, content: result, args: args})
			m.updateViewport()
		}
		return m, tea.Batch(m.spinner.Tick, waitForEvent(m.events))

	case event.CompactStart:
		if data, ok := ev.Data.(event.CompactStartData); ok {
			m.messages = append(m.messages, message{
				role:    "compact:start",
				content: fmt.Sprintf("tokens %d exceeded %d", data.Tokens, data.Threshold),
			})
			m.updateViewport()
		}
		return m, tea.Batch(m.spinner.Tick, waitForEvent(m.events))

	case event.CompactEnd:
		if data, ok := ev.Data.(event.CompactEndData); ok {
			m.messages = append(m.messages, message{
				role:    "compact:end",
				content: fmt.Sprintf("%d → %d tokens", data.Before, data.After),
			})
			m.updateViewport()
		}
		return m, tea.Batch(m.spinner.Tick, waitForEvent(m.events))

	case event.TextDelta:
		if data, ok := ev.Data.(event.TextDeltaData); ok {
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].role == "assistant" {
				m.messages[len(m.messages)-1].content += data.Text
			} else {
				m.messages = append(m.messages, message{role: "assistant", content: data.Text})
			}
			m.updateViewport()
		}
		return m, waitForEvent(m.events)

	case event.Done:
		if data, ok := ev.Data.(event.DoneData); ok {
			m.inputTokens += data.InputTokens
			m.outputTokens += data.OutputTokens
		}
		m.thinking = false
		m.toolName = ""
		m.updateViewport()
		return m, waitForEvent(m.events)

	case event.Error:
		if data, ok := ev.Data.(event.ErrorData); ok {
			m.messages = append(m.messages, message{role: "error", content: data.Message})
		}
		m.thinking = false
		m.toolName = ""
		m.updateViewport()
		return m, nil
	}

	return m, nil
}

func (m *Model) initMarkdownRenderer() {
	w := max(m.width-2, 40)
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(w),
	)
	if err == nil {
		m.mdRenderer = r
	}
}

func (m *Model) renderMarkdown(content string) string {
	if m.mdRenderer == nil {
		return content
	}
	out, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(out, "\n")
}

func (m *Model) updateViewport() {
	var sb strings.Builder
	for i, msg := range m.messages {
		m.renderMsg(&sb, msg)
		if i < len(m.messages)-1 && m.needsGap(msg.role, m.messages[i+1].role) {
			sb.WriteString("\n")
		}
	}
	m.viewport.SetContent(sb.String())
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

var (
	primary   = lipgloss.Color("#729FCF")
	secondary = lipgloss.Color("#FCAF3E")
	success   = lipgloss.Color("#8AE234")
	errColor  = lipgloss.Color("#EF2929")
	fgBase    = lipgloss.Color("#D3D7CF")
	fgMuted   = lipgloss.Color("#BABDB6")
	fgSubtle  = lipgloss.Color("#555753")
	bgSubtle  = lipgloss.Color("#2E2E2E")
)

func (m *Model) renderMsg(sb *strings.Builder, msg message) {
	switch {
	case msg.role == "user":
		sb.WriteString(lipgloss.NewStyle().Foreground(fgMuted).Render("You"))
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(fgBase).
			BorderLeft(true).
			BorderForeground(primary).
			BorderStyle(lipgloss.Border{Left: "▌"}).
			PaddingLeft(1).
			Render(msg.content))
		sb.WriteString("\n")
	case msg.role == "assistant":
		sb.WriteString(lipgloss.NewStyle().Foreground(fgMuted).Render("Assistant"))
		sb.WriteString("\n")
		sb.WriteString(m.renderMarkdown(msg.content))
		sb.WriteString("\n")
	case msg.role == "system":
		sb.WriteString(lipgloss.NewStyle().Foreground(fgSubtle).Italic(true).Render("// " + msg.content))
		sb.WriteString("\n")
	case msg.role == "error":
		sb.WriteString(lipgloss.NewStyle().Foreground(errColor).SetString("✗").String() + " " +
			lipgloss.NewStyle().Foreground(errColor).Render(msg.content))
		sb.WriteString("\n")
	case strings.HasPrefix(msg.role, "tool:start:"):
		m.renderToolStart(sb, strings.TrimPrefix(msg.role, "tool:start:"), msg.args)
	case strings.HasPrefix(msg.role, "tool:end:"):
		m.renderToolEnd(sb, strings.TrimPrefix(msg.role, "tool:end:"), msg.content, msg.args)
	case strings.HasPrefix(msg.role, "tool:error:"):
		m.renderToolError(sb, strings.TrimPrefix(msg.role, "tool:error:"), msg.content, msg.args)
	case msg.role == "compact:start":
		icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#AD7FA8")).SetString("⟳")
		sb.WriteString("  " + icon.String() + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#AD7FA8")).Render("Compacting history") +
			" " + lipgloss.NewStyle().Foreground(fgMuted).Render("("+msg.content+")"))
		sb.WriteString("\n")
	case msg.role == "compact:end":
		icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#AD7FA8")).Bold(true).SetString("⟳")
		sb.WriteString("  " + icon.String() + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#AD7FA8")).Render("Compacted") +
			" " + lipgloss.NewStyle().Foreground(fgMuted).Render("("+msg.content+")"))
		sb.WriteString("\n")
	}
}

func (m *Model) formatToolArgs(args string) string {
	if args == "" {
		return ""
	}
	if len([]rune(args)) > 60 {
		args = string([]rune(args)[:60]) + "..."
	}
	return " " + lipgloss.NewStyle().Foreground(fgMuted).Render("("+args+")")
}

func (m *Model) renderToolStart(sb *strings.Builder, name, args string) {
	icon := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AE234")).SetString("◉")
	label := lipgloss.NewStyle().Foreground(secondary).Render(name)
	sb.WriteString("  " + icon.String() + " Using " + label + m.formatToolArgs(args))
	sb.WriteString("\n")
}

func (m *Model) renderToolEnd(sb *strings.Builder, name, content, args string) {
	icon := lipgloss.NewStyle().Foreground(success).Bold(true).SetString("✓")
	label := lipgloss.NewStyle().Foreground(secondary).Render(name)
	sb.WriteString("  " + icon.String() + " Used " + label + m.formatToolArgs(args))
	if content == "" {
		sb.WriteString("\n")
		return
	}
	sb.WriteString("\n")
	lines := strings.Split(content, "\n")
	shown := lines
	truncated := 0
	if len(lines) > maxToolResultLines {
		shown = lines[:maxToolResultLines]
		truncated = len(lines) - maxToolResultLines
	}
	for _, line := range shown {
		if line != "" {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(fgMuted).
				Background(bgSubtle).
				BorderLeft(true).
				BorderForeground(fgSubtle).
				BorderStyle(lipgloss.Border{Left: "╎"}).
				PaddingLeft(2).
				Padding(0, 1).
				Render("  " + line))
			sb.WriteString("\n")
		}
	}
	if truncated > 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(fgMuted).Render(fmt.Sprintf("    ... %d more lines", truncated)))
		sb.WriteString("\n")
	}
}

func (m *Model) renderToolError(sb *strings.Builder, name, content, args string) {
	icon := lipgloss.NewStyle().Foreground(errColor).Bold(true).SetString("✗")
	label := lipgloss.NewStyle().Foreground(secondary).Render(name)
	sb.WriteString("  " + icon.String() + " Failed " + label + m.formatToolArgs(args))
	if content != "" {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(errColor).Render("    " + content))
	}
	sb.WriteString("\n")
}

func (m *Model) needsGap(role, nextRole string) bool {
	return (role == "user" || role == "assistant") ||
		(nextRole == "user" || nextRole == "assistant")
}

func (m Model) View() string {
	if !m.ready {
		return lipgloss.NewStyle().Foreground(fgMuted).Render("Loading...")
	}

	sessionLabel := lipgloss.NewStyle().Foreground(fgSubtle).Render("Session ")
	sessionTitle := "New Chat"
	if s := m.bus.GetSession(m.session); s != nil {
		sessionTitle = s.Title
	}
	headerText := lipgloss.NewStyle().Foreground(primary).Render(sessionTitle)
	header := sessionLabel + headerText

	var statusLine string
	if m.thinking {
		status := m.spinner.View() + " "
		if m.toolName != "" {
			status += "Using " + lipgloss.NewStyle().Foreground(secondary).Render(m.toolName) +
				lipgloss.NewStyle().Foreground(fgMuted).Render("...")
		} else {
			status += lipgloss.NewStyle().Foreground(fgMuted).Render("Thinking...")
		}
		statusLine = lipgloss.NewStyle().Foreground(fgMuted).Padding(0, 1).Render(status)
	}

	if !m.viewport.AtBottom() {
		hint := lipgloss.NewStyle().Foreground(fgMuted).Italic(true).Render("  ↓ more")
		if statusLine != "" {
			statusLine += "  " + hint
		} else {
			statusLine = hint
		}
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fgSubtle).
		Background(bgSubtle).
		Padding(0, 1).
		Render(m.input.View())

	streamMode := "OFF"
	if config.C.Stream {
		streamMode = "ON"
	}
	modelInfo := lipgloss.NewStyle().Foreground(secondary).Render(config.C.CurrentProviderName() + "/" + config.C.CurrentModelName())
	tokenInput := lipgloss.NewStyle().Foreground(fgMuted).Render(fmt.Sprintf("input:%d", m.inputTokens))
	tokenOutput := lipgloss.NewStyle().Foreground(fgMuted).Render(fmt.Sprintf("output:%d", m.outputTokens))
	shortcuts := modelInfo +
		lipgloss.NewStyle().Foreground(fgMuted).Render(" | ") +
		tokenInput +
		lipgloss.NewStyle().Foreground(fgMuted).Render(" | ") +
		tokenOutput +
		lipgloss.NewStyle().Foreground(fgMuted).Render(" | ") +
		lipgloss.NewStyle().Foreground(fgBase).Render("Enter") +
		lipgloss.NewStyle().Foreground(fgMuted).Render(" send  ") +
		lipgloss.NewStyle().Foreground(fgBase).Render("Ctrl+J") +
		lipgloss.NewStyle().Foreground(fgMuted).Render(" newline  ") +
		lipgloss.NewStyle().Foreground(fgBase).Render("Ctrl+S") +
		lipgloss.NewStyle().Foreground(fgMuted).Render(fmt.Sprintf(" stream:%s  ", streamMode)) +
		lipgloss.NewStyle().Foreground(fgBase).Render("Ctrl+C") +
		lipgloss.NewStyle().Foreground(fgMuted).Render(" quit")

	content := m.viewport.View()

	var parts []string
	parts = append(parts, header)
	parts = append(parts, content)
	if statusLine != "" {
		parts = append(parts, statusLine)
	}
	parts = append(parts, inputBox)
	parts = append(parts, shortcuts)

	return strings.Join(parts, "\n")
}

func msgsToLLM(msgs []msg.Msg) []llm.Message {
	out := make([]llm.Message, len(msgs))
	for i, m := range msgs {
		out[i] = llm.Message{
			Role:        m.Role,
			Content:     m.Text,
			ToolCalls:   m.ToolCalls,
			ToolResults: m.ToolResults,
		}
	}
	return out
}
