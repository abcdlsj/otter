package tool

import (
	"context"
	"encoding/json"

	"github.com/tmc/langchaingo/llms"
)

type Tool interface {
	Name() string
	Desc() string
	Args() map[string]any
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

func ToLangchain(t Tool) llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        t.Name(),
			Description: t.Desc(),
			Parameters:  t.Args(),
		},
	}
}

type Set struct {
	tools map[string]Tool
}

func NewSet() *Set {
	s := &Set{tools: make(map[string]Tool)}
	s.Add(&Shell{})
	s.Add(&File{})
	s.Add(&Grep{})
	s.Add(&List{})
	s.Add(&View{})
	s.Add(&WebFetch{})
	s.Add(&WebSearch{})
	return s
}

func (s *Set) Add(t Tool)              { s.tools[t.Name()] = t }
func (s *Set) Get(name string) Tool    { return s.tools[name] }
func (s *Set) All() []Tool             {
	var ts []Tool
	for _, t := range s.tools {
		ts = append(ts, t)
	}
	return ts
}

func (s *Set) ToLangchain() []llms.Tool {
	var ts []llms.Tool
	for _, t := range s.tools {
		ts = append(ts, ToLangchain(t))
	}
	return ts
}
