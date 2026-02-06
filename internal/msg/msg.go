package msg

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
)

type Msg struct {
	ID      string    `json:"id"`
	Session string    `json:"session"`
	Role    string    `json:"role"` // user, assistant, system
	Text    string    `json:"text"`
	Time    time.Time `json:"time"`
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
	mu       sync.RWMutex
	subs     map[string][]chan Msg
	sessions map[string]*Session
	dir      string
}

func NewBus(dir string) *Bus {
	b := &Bus{
		subs:     make(map[string][]chan Msg),
		sessions: make(map[string]*Session),
		dir:      dir,
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
	return list
}

func (b *Bus) DeleteSession(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, id)
	if b.dir != "" {
		os.Remove(filepath.Join(b.dir, id+".jsonl"))
	}
}

func (b *Bus) Sub(session string) <-chan Msg {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Msg, 64)
	b.subs[session] = append(b.subs[session], ch)
	return ch
}

func (b *Bus) Unsub(session string, ch <-chan Msg) {
	b.mu.Lock()
	defer b.mu.Unlock()
	chs := b.subs[session]
	for i, c := range chs {
		if c == ch {
			b.subs[session] = append(chs[:i], chs[i+1:]...)
			close(c)
			return
		}
	}
}

func (b *Bus) Pub(m Msg) {
	b.mu.Lock()
	if s, ok := b.sessions[m.Session]; ok {
		s.Messages = append(s.Messages, m)
		s.UpdatedAt = time.Now()
		if len(s.Messages) == 1 && m.Role == "user" {
			title := m.Text
			if len(title) > 30 {
				title = title[:30] + "..."
			}
			s.Title = title
		}
		b.appendMsg(m)
	}
	b.mu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[m.Session] {
		select {
		case ch <- m:
		default:
		}
	}
}

func (b *Bus) load() {
	if b.dir == "" {
		return
	}
	os.MkdirAll(b.dir, 0755)

	entries, err := os.ReadDir(b.dir)
	if err != nil {
		log.Warn("failed to read sessions dir", "err", err)
		return
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		sessionID := e.Name()[:len(e.Name())-6] // remove .jsonl
		msgs := b.loadSession(sessionID)
		if len(msgs) == 0 {
			continue
		}
		s := b.rebuildSession(sessionID, msgs)
		b.sessions[sessionID] = s
	}
	log.Info("loaded sessions", "count", len(b.sessions))
}

func (b *Bus) loadSession(id string) []Msg {
	f, err := os.Open(filepath.Join(b.dir, id+".jsonl"))
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
				if len(title) > 30 {
					title = title[:30] + "..."
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
	path := filepath.Join(b.dir, m.Session+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Warn("failed to open session file", "err", err)
		return
	}
	defer f.Close()

	data, _ := json.Marshal(m)
	f.Write(data)
	f.WriteString("\n")
}
