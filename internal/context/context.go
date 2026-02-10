package context

import (
	"encoding/json"
	"time"
)

// TechStack represents detected technologies
type TechStack struct {
	Languages   []string `json:"languages"`
	Frameworks  []string `json:"frameworks"`
	BuildTools  []string `json:"build_tools"`
	PackageMgrs []string `json:"package_mgrs"`
	Runtimes    []string `json:"runtimes"`
}

// CLAUDESpec represents parsed CLAUDE.md content
type CLAUDESpec struct {
	StyleGuide  string `json:"style_guide,omitempty"`
	Patterns    []string `json:"patterns,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
	RawContent  string `json:"raw_content,omitempty"`
}

// RecentTopic tracks conversation topics
type RecentTopic struct {
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
}

// CommonCommands stores frequently used commands
type CommonCommands struct {
	Build   []string `json:"build,omitempty"`
	Test    []string `json:"test,omitempty"`
	Lint    []string `json:"lint,omitempty"`
	Dev     []string `json:"dev,omitempty"`
	Custom  []string `json:"custom,omitempty"`
}

// ProjectContext holds all context for a git project
type ProjectContext struct {
	Path           string         `json:"path"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	TechStack      TechStack      `json:"tech_stack"`
	Commands       CommonCommands `json:"commands"`
	CLAUDESpec     CLAUDESpec     `json:"claude_spec,omitempty"`
	RecentTopics   []RecentTopic  `json:"recent_topics,omitempty"`
	LastUpdated    time.Time      `json:"last_updated"`
}

// New creates empty context
func New(path, name string) *ProjectContext {
	return &ProjectContext{
		Path:         path,
		Name:         name,
		RecentTopics: []RecentTopic{},
		LastUpdated:  time.Now(),
	}
}

// AddTopic adds a new topic and maintains recency limit
func (p *ProjectContext) AddTopic(topic string) {
	for _, t := range p.RecentTopics {
		if t.Topic == topic {
			return
		}
	}
	p.RecentTopics = append(p.RecentTopics, RecentTopic{
		Topic:     topic,
		Timestamp: time.Now(),
	})
	if len(p.RecentTopics) > 10 {
		p.RecentTopics = p.RecentTopics[len(p.RecentTopics)-10:]
	}
	p.LastUpdated = time.Now()
}

// ToJSON serializes context
func (p *ProjectContext) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON deserializes context
func FromJSON(data []byte) (*ProjectContext, error) {
	var p ProjectContext
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
