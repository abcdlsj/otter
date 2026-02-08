package msg

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/abcdlsj/otter/internal/event"
	"github.com/abcdlsj/otter/internal/logger"
	"github.com/abcdlsj/otter/internal/types"
	"github.com/google/uuid"
)

type Msg struct {
	ID          string             `json:"id"`
	Session     string             `json:"session"`
	Role        string             `json:"role"` // user, assistant, system, tool
	Text        string             `json:"text"`
	ToolCalls   []types.ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []types.ToolResult `json:"tool_results,omitempty"`
	Time        time.Time          `json:"time"`
}

func New(session, role, text string) Msg {
	return Msg{
		ID:      uuid.NewString()[:8],
		Session: session,
		Role:    role,
		Text:    text,
		Time:    time.Now(),
	}
}

func User(session, text string) Msg   { return New(session, "user", text) }
func Bot(session, text string) Msg    { return New(session, "assistant", text) }
func System(session, text string) Msg { return New(session, "system", text) }

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Msg     `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Bus struct {
	mu        sync.RWMutex
	subs      map[string][]chan Msg
	sessions  map[string]*Session
	dir       string
	eventSubs map[string][]chan event.Event
}

func NewBus(dir string) *Bus {
	logger.Init(dir)
	b := &Bus{
		subs:      make(map[string][]chan Msg),
		sessions:  make(map[string]*Session),
		dir:       dir,
		eventSubs: make(map[string][]chan event.Event),
	}
	b.load()
	return b
}

func (b *Bus) GetSession(id string) *Session {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sessions[id]
}

func (b *Bus) GetOrCreateSession(id string) *Session {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.sessions[id]; ok {
		return s
	}
	s := &Session{
		ID:        id,
		Title:     "New Chat",
		Messages:  []Msg{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	b.sessions[id] = s
	return s
}

func (b *Bus) ListSessions() []*Session {
	b.mu.RLock()
	defer b.mu.RUnlock()
	list := make([]*Session, 0, len(b.sessions))
	for _, s := range b.sessions {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

func (b *Bus) DeleteSession(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, id)
	if b.dir != "" {
		os.RemoveAll(filepath.Join(b.dir, id))
	}
}

func (b *Bus) SetSessionTitle(id, title string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.sessions[id]; ok {
		s.Title = title
	}
}

func sub[T any](mu sync.Locker, subs map[string][]chan T, session string) <-chan T {
	mu.Lock()
	defer mu.Unlock()
	ch := make(chan T, 64)
	subs[session] = append(subs[session], ch)
	return ch
}

func unsub[T any](mu sync.Locker, subs map[string][]chan T, session string, ch <-chan T) {
	mu.Lock()
	defer mu.Unlock()
	chs := subs[session]
	for i, c := range chs {
		if c == ch {
			subs[session] = append(chs[:i], chs[i+1:]...)
			close(c)
			return
		}
	}
}

func broadcast[T any](mu sync.Locker, subs map[string][]chan T, session string, v T) {
	mu.Lock()
	defer mu.Unlock()
	for _, ch := range subs[session] {
		select {
		case ch <- v:
		default:
		}
	}
}

func (b *Bus) Sub(session string) <-chan Msg            { return sub(&b.mu, b.subs, session) }
func (b *Bus) Unsub(session string, ch <-chan Msg)      { unsub(&b.mu, b.subs, session, ch) }
func (b *Bus) SubEvent(session string) <-chan event.Event { return sub(&b.mu, b.eventSubs, session) }
func (b *Bus) UnsubEvent(session string, ch <-chan event.Event) {
	unsub(&b.mu, b.eventSubs, session, ch)
}

func (b *Bus) Pub(m Msg) {
	b.mu.Lock()
	if s, ok := b.sessions[m.Session]; ok {
		s.Messages = append(s.Messages, m)
		s.UpdatedAt = time.Now()
		b.appendMsg(m)
	}
	subs := make([]chan Msg, len(b.subs[m.Session]))
	copy(subs, b.subs[m.Session])
	b.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- m:
		default:
		}
	}
}

func (b *Bus) pubEvent(session string, ev event.Event) {
	broadcast(&b.mu, b.eventSubs, session, ev)
}

func (b *Bus) HandleEvents(sessionID string, events <-chan event.Event) <-chan event.Event {
	out := make(chan event.Event, 64)
	go func() {
		defer close(out)
		for ev := range events {
			out <- ev
			b.pubEvent(sessionID, ev)
			switch ev.Type {
			case event.CompactEnd:
				if data, ok := ev.Data.(event.CompactEndData); ok {
					b.Pub(New(sessionID, "system",
						fmt.Sprintf("[compact] %d → %d tokens", data.Before, data.After)))
				}
			case event.Done:
				if data, ok := ev.Data.(event.DoneData); ok {
					for _, em := range data.Messages {
						b.Pub(Msg{
							Session:     sessionID,
							Role:        em.Role,
							Text:        em.Content,
							ToolCalls:   em.ToolCalls,
							ToolResults: em.ToolResults,
						})
					}
				}
			}
		}
	}()
	return out
}

func (b *Bus) load() {
	if b.dir == "" {
		return
	}
	os.MkdirAll(b.dir, 0755)

	entries, err := os.ReadDir(b.dir)
	if err != nil {
		logger.Warn("failed to read sessions dir", "err", err)
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sessionID := e.Name()
		msgs := b.loadSession(sessionID)
		if len(msgs) == 0 {
			continue
		}
		s := b.rebuildSession(sessionID, msgs)
		b.sessions[sessionID] = s
	}
	logger.Info("loaded sessions", "count", len(b.sessions))
}

func (b *Bus) loadSession(id string) []Msg {
	f, err := os.Open(filepath.Join(b.dir, id, "session.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var msgs []Msg
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var m Msg
		if json.Unmarshal(scanner.Bytes(), &m) == nil {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

func (b *Bus) rebuildSession(id string, msgs []Msg) *Session {
	s := &Session{
		ID:       id,
		Title:    "New Chat",
		Messages: msgs,
	}
	if len(msgs) > 0 {
		s.CreatedAt = msgs[0].Time
		s.UpdatedAt = msgs[len(msgs)-1].Time
		for _, m := range msgs {
			if m.Role == "user" {
				title := m.Text
				if len([]rune(title)) > 30 {
					title = string([]rune(title)[:30]) + "..."
				}
				s.Title = title
				break
			}
		}
	}
	return s
}

func (b *Bus) appendMsg(m Msg) {
	if b.dir == "" {
		return
	}
	dir := filepath.Join(b.dir, m.Session)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "session.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Warn("failed to open session file", "err", err)
		return
	}
	defer f.Close()

	data, _ := json.Marshal(m)
	f.Write(data)
	f.WriteString("\n")
}
