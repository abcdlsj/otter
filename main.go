package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/abcdlsj/otter/internal/agent"
	"github.com/abcdlsj/otter/internal/config"
	pctx "github.com/abcdlsj/otter/internal/context"
	"github.com/abcdlsj/otter/internal/llm"
	"github.com/abcdlsj/otter/internal/msg"
	"github.com/abcdlsj/otter/internal/tool"
	"github.com/abcdlsj/otter/internal/tui"
)

func main() {
	if err := config.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	llmClient, err := llm.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create LLM client: %v\n", err)
		os.Exit(1)
	}

	// Initialize context manager
	ctxMgr, err := pctx.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init context manager: %v\n", err)
		ctxMgr = nil
	}

	tools := tool.NewSet()
	
	var ag *agent.Agent
	if ctxMgr != nil {
		ag = agent.NewWithContext(llmClient, tools, ctxMgr)
	} else {
		ag = agent.New(llmClient, tools)
	}
	
	bus := msg.NewBus(config.SessionsDir())

	model := tui.New(ag, tools, bus)
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(context.Background()),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
