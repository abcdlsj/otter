package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abcdlsj/otter/internal/config"
)

// Manager handles project context lifecycle
type Manager struct {
	store   *Store
	current *ProjectContext
}

// NewManager creates manager with default store
func NewManager() (*Manager, error) {
	store, err := NewStore(DefaultStorePath())
	if err != nil {
		return nil, err
	}
	return &Manager{store: store}, nil
}

// NewManagerWithStore creates manager with custom store
func NewManagerWithStore(store *Store) *Manager {
	return &Manager{store: store}
}

// Close cleans up resources
func (m *Manager) Close() error {
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

// Identify detects and builds context for git project at path
func (m *Manager) Identify(path string) (*ProjectContext, error) {
	if !isGitRepo(path) {
		return nil, fmt.Errorf("not a git repository: %s", path)
	}

	name := filepath.Base(path)
	pType := DetectType(path)
	
	p := New(path, name)
	p.Type = pType
	p.TechStack = AnalyzeStructure(path)
	p.Commands = DetectCommands(path, pType)
	p.CLAUDESpec = parseCLAUDE(path)
	
	return p, nil
}

// Load retrieves stored context for path
func (m *Manager) Load(path string) (*ProjectContext, error) {
	return m.store.Load(path)
}

// Save persists context
func (m *Manager) Save(p *ProjectContext) error {
	if m.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return m.store.Save(p)
}

// Build creates fresh context and saves it
func (m *Manager) Build(path string) (*ProjectContext, error) {
	p, err := m.Identify(path)
	if err != nil {
		return nil, err
	}
	if err := m.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetOrBuild loads existing or builds new context
func (m *Manager) GetOrBuild(path string) (*ProjectContext, error) {
	if p, err := m.Load(path); err == nil && p != nil {
		return p, nil
	}
	return m.Build(path)
}

// Inject formats context for system prompt
func (m *Manager) Inject(p *ProjectContext) string {
	if p == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Project Context\n\n")
	b.WriteString(fmt.Sprintf("- **Name**: %s\n", p.Name))
	b.WriteString(fmt.Sprintf("- **Path**: %s\n", p.Path))
	b.WriteString(fmt.Sprintf("- **Type**: %s\n", p.Type))

	if len(p.TechStack.Languages) > 0 {
		b.WriteString(fmt.Sprintf("- **Languages**: %s\n", strings.Join(p.TechStack.Languages, ", ")))
	}
	if len(p.TechStack.Frameworks) > 0 {
		b.WriteString(fmt.Sprintf("- **Frameworks**: %s\n", strings.Join(p.TechStack.Frameworks, ", ")))
	}
	if len(p.TechStack.Runtimes) > 0 {
		b.WriteString(fmt.Sprintf("- **Runtimes**: %s\n", strings.Join(p.TechStack.Runtimes, ", ")))
	}

	// Add commands
	cmds := []string{}
	cmds = append(cmds, p.Commands.Build...)
	cmds = append(cmds, p.Commands.Test...)
	cmds = append(cmds, p.Commands.Dev...)
	if len(cmds) > 0 {
		b.WriteString(fmt.Sprintf("- **Common Commands**: %s\n", strings.Join(cmds[:min(len(cmds), 5)], ", ")))
	}

	// Add CLAUDE.md spec if present
	if p.CLAUDESpec.RawContent != "" {
		b.WriteString("\n### Project Style Guide\n")
		b.WriteString(p.CLAUDESpec.RawContent)
		b.WriteString("\n")
	}

	return b.String()
}

// Current returns active project context
func (m *Manager) Current() *ProjectContext {
	return m.current
}

// SetCurrent sets active context
func (m *Manager) SetCurrent(p *ProjectContext) {
	m.current = p
}

// AutoDetect attempts to detect and load context for current directory
func (m *Manager) AutoDetect() (*ProjectContext, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	
	// Find git root
	root := findGitRoot(wd)
	if root == "" {
		return nil, nil
	}
	
	return m.GetOrBuild(root)
}

// List returns all known projects
func (m *Manager) List() ([]*ProjectContext, error) {
	return m.store.List()
}

// Forget removes context
func (m *Manager) Forget(path string) error {
	return m.store.Delete(path)
}

// Learn adds a topic to current context
func (m *Manager) Learn(topic string) error {
	if m.current == nil {
		return fmt.Errorf("no active project context")
	}
	m.current.AddTopic(topic)
	return m.Save(m.current)
}

// isGitRepo checks if path is git repository
func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// findGitRoot finds nearest .git ancestor
func findGitRoot(path string) string {
	for {
		if isGitRepo(path) {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return ""
}

// parseCLAUDE reads CLAUDE.md if exists
func parseCLAUDE(path string) CLAUDESpec {
	var spec CLAUDESpec
	
	for _, name := range []string{"CLAUDE.md", "claude.md", ".claude.md"} {
		data, err := os.ReadFile(filepath.Join(path, name))
		if err == nil {
			spec.RawContent = string(data)
			break
		}
	}
	
	return spec
}

// ContextsDir returns directory for context storage
func ContextsDir() string {
	return filepath.Join(config.Home(), "contexts")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
