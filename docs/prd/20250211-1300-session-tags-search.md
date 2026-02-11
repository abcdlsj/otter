# Feature: Session Tags & Semantic Search (会话标签与语义搜索)

## 背景与动机

### 现状问题

Otter 目前支持会话持久化，用户可以查看历史会话 (`/sessions`) 和切换会话 (`/switch <id>`)。但随着使用时间增长，用户会积累数十甚至上百个会话，面临以下问题：

1. **难以检索**: `/sessions` 只显示 ID 和时间，难以快速找到"上周那个关于数据库优化的会话"
2. **缺乏组织**: 会话之间没有关联，相关讨论分散在不同会话中
3. **标题局限**: 自动生成的标题只取第一条消息前30字，往往无法准确描述会话内容
4. **搜索困难**: 没有全文搜索能力，必须逐个打开会话查看内容

### 实际场景

```
场景1: 寻找历史方案
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 我记得上周让 Otter 帮我设计过一个缓存方案，
      但现在找不到了...
用户: /sessions
Otter: 显示 50+ 个会话列表，标题都是"帮我优化..."、"这个报错..."
用户: [逐个打开查看，耗时 10 分钟]

场景2: 跨会话知识关联
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
会话 A (3天前): 讨论了 API 设计方案
会话 B (昨天): 实现了认证功能  
会话 C (今天): 用户想结合 A 和 B 的内容
问题: 没有方式标记这些会话都是"用户系统"相关的

场景3: 项目管理混乱
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户同时维护 3 个项目，会话混杂在一起：
- 项目 A 的 bug 修复
- 项目 B 的功能开发
- 项目 A 的性能优化
无法按项目筛选会话
```

### 为什么需要这个功能

1. **效率提升**: 秒级定位历史会话，而非分钟级
2. **知识管理**: 将分散的会话组织成可检索的知识库
3. **连续性**: 跨会话保持话题关联，形成完整的开发轨迹
4. **项目对齐**: 与已有的 Project Context 功能配合，形成"项目-会话"双层组织

### 竞品对比

| 工具 | 会话组织 | 搜索能力 | 说明 |
|------|----------|----------|------|
| Claude Code | ⚠️ 基础 | ❌ 无 | 仅按时间排序 |
| Cursor | ✅ 标签 | ⚠️ 有限 | 支持文件夹组织 |
| ChatGPT | ✅ 归档 | ✅ 搜索 | 支持标题搜索 |
| Windsurf | ⚠️ 基础 | ❌ 无 | 会话列表 |
| Otter | ❌ **待增强** | ❌ **待实现** | 当前仅简单列表 |

---

## 功能描述

### 核心功能

1. **智能标签生成**: 基于会话内容自动生成标签（如 "#database", "#performance", "#bugfix"）
2. **手动标签管理**: 用户可添加、删除、修改标签
3. **全文语义搜索**: 支持按关键词、标签、时间范围搜索会话
4. **会话关联**: 显示相关会话，发现知识关联
5. **增强列表视图**: 会话列表显示标签、消息数、 token 用量

### 标签系统设计

```yaml
标签类型:
  auto:       # 系统自动生成
    - 技术栈:  "#go", "#react", "#postgres"
    - 任务类型: "#refactor", "#bugfix", "#feature"
    - 主题:    "#performance", "#security", "#testing"
    
  manual:     # 用户手动添加
    - 项目:   "#user-service", "#payment-v2"
    - 优先级: "#urgent", "#backlog"
    - 状态:   "#done", "#wip", "#blocked"
    
  inferred:   # 智能推断
    - 基于 Project Context 关联
    - 基于代码文件关联
    - 基于工具调用模式
```

### 使用场景

```
场景1: 快速定位历史会话
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: /search 缓存策略
Otter: 找到 3 个相关会话:
       1. [2天前] Redis 缓存设计 #redis #performance (89% 匹配)
       2. [上周] 数据库查询优化 #database #performance (67% 匹配)
       3. [上周] 缓存穿透问题 #redis #bugfix (45% 匹配)

场景2: 标签筛选
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: /tag go
Otter: 显示所有带 #go 标签的会话（按时间倒序）

用户: /tag performance bugfix
Otter: 显示同时带 #performance 和 #bugfix 的会话

场景3: 手动组织
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: /tag add payment-v2
Otter: 已为当前会话添加标签 #payment-v2

用户: /sessions
Otter: 
  * 20250211_143022 - 支付流程重构 #payment-v2 #refactor
  20250210_090145 - 数据库连接池调优 #performance #go
  20250209_162233 - Redis 缓存设计 #redis #performance

场景4: 会话关联发现
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: /related
Otter: 与当前会话相关的历史讨论:
       - 数据库索引优化 (共享标签: #performance, #database)
       - API 响应时间分析 (共享标签: #performance)
```

---

## 技术方案

### 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                           TUI                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ /tag 命令    │  │ /search 命令 │  │ 增强 /sessions      │  │
│  │              │  │              │  │ (显示标签)          │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Session Tag Manager                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Tag Generator│  │ Search Engine│  │ Index Manager       │  │
│  │ (LLM/NLP)    │  │ (SQLite FTS) │  │                     │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Storage Layer                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  ~/.config/otter/sessions/                                 │  │
│  │  ├── {session_id}/                                         │  │
│  │  │   ├── session.jsonl       # 消息记录                  │  │
│  │  │   └── meta.json           # 元数据(标签、摘要等)       │  │
│  │  └── search.db                # SQLite FTS 索引           │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 文件改动清单

#### 1. `internal/session/` (新增目录)

**`internal/session/types.go`** - 核心类型定义
```go
package session

import "time"

// Meta 会话元数据
type Meta struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`
    Tags         []string  `json:"tags"`
    AutoTags     []string  `json:"auto_tags"`
    Summary      string    `json:"summary"`       // AI 生成的摘要
    MessageCount int       `json:"message_count"`
    TokenUsed    int64     `json:"token_used"`
    ProjectPath  string    `json:"project_path"`  // 关联的项目
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
    Keywords   string    // 全文搜索词
    Tags       []string  // 标签筛选
    Project    string    // 项目筛选
    DateFrom   time.Time // 时间范围
    DateTo     time.Time
    Limit      int
}

// SearchResult 搜索结果
type SearchResult struct {
    Meta        Meta
    Score       float64 // 相关度分数
    MatchedText string  // 匹配到的文本片段
}
```

**`internal/session/manager.go`** - 会话管理器
```go
package session

// Manager 管理会话元数据和搜索
type Manager struct {
    dir    string
    db     *sql.DB  // SQLite for FTS
}

func NewManager(dir string) (*Manager, error)

// Index 为新会话建立索引
func (m *Manager) Index(sessionID string, messages []msg.Msg) error

// GenerateTags 基于会话内容生成标签
func (m *Manager) GenerateTags(messages []msg.Msg) ([]string, error)

// GenerateSummary 生成会话摘要
func (m *Manager) GenerateSummary(messages []msg.Msg) (string, error)

// AddTags 为会话添加标签
func (m *Manager) AddTags(sessionID string, tags []string) error

// RemoveTags 移除标签
func (m *Manager) RemoveTags(sessionID string, tags []string) error

// Search 全文搜索
func (m *Manager) Search(q SearchQuery) ([]SearchResult, error)

// ListByTags 按标签筛选
func (m *Manager) ListByTags(tags []string) ([]Meta, error)

// GetRelated 获取相关会话
func (m *Manager) GetRelated(sessionID string, limit int) ([]Meta, error)

// List 列出所有会话元数据
func (m *Manager) List() ([]Meta, error)
```

**`internal/session/search.go`** - 搜索引擎实现
```go
package session

// initDB 初始化 SQLite FTS 表
func (m *Manager) initDB() error {
    schema := `
    -- 会话元数据表
    CREATE TABLE IF NOT EXISTS sessions (
        id TEXT PRIMARY KEY,
        title TEXT,
        tags TEXT,          -- JSON array
        summary TEXT,
        message_count INTEGER,
        token_used INTEGER,
        project_path TEXT,
        created_at DATETIME,
        updated_at DATETIME
    );
    
    -- 全文搜索虚拟表
    CREATE VIRTUAL TABLE IF NOT EXISTS sessions_fts USING fts5(
        session_id UNINDEXED,
        title,
        content,
        tokenize='porter unicode61'
    );
    
    -- 标签索引
    CREATE TABLE IF NOT EXISTS tags (
        tag TEXT,
        session_id TEXT,
        PRIMARY KEY (tag, session_id)
    );
    CREATE INDEX IF NOT EXISTS idx_tags ON tags(tag);
    `
    _, err := m.db.Exec(schema)
    return err
}

// indexContent 索引会话内容
func (m *Manager) indexContent(sessionID string, messages []msg.Msg) error {
    // 提取所有文本内容
    var contents []string
    for _, m := range messages {
        if m.Text != "" {
            contents = append(contents, m.Text)
        }
    }
    content := strings.Join(contents, " ")
    
    // 插入/更新 FTS 表
    _, err := m.db.Exec(
        "INSERT OR REPLACE INTO sessions_fts (session_id, content) VALUES (?, ?)",
        sessionID, content,
    )
    return err
}

// fullTextSearch 执行全文搜索
func (m *Manager) fullTextSearch(keyword string, limit int) ([]SearchResult, error) {
    query := `
    SELECT s.*, rank
    FROM sessions s
    JOIN sessions_fts fts ON s.id = fts.session_id
    WHERE sessions_fts MATCH ?
    ORDER BY rank
    LIMIT ?
    `
    rows, err := m.db.Query(query, keyword, limit)
    // ... parse rows
}
```

**`internal/session/tagger.go`** - 智能标签生成器
```go
package session

// Tagger 基于内容生成标签
type Tagger struct {
    llm *llm.LLM
}

// 预定义标签库
var tagLibrary = []string{
    // 技术栈
    "go", "python", "javascript", "typescript", "rust",
    "react", "vue", "angular", "svelte",
    "postgres", "mysql", "redis", "mongodb", "elasticsearch",
    "docker", "kubernetes", "aws", "gcp", "azure",
    
    // 任务类型
    "refactor", "bugfix", "feature", "test", "docs",
    "optimize", "setup", "deploy", "migrate",
    
    // 主题
    "performance", "security", "database", "api", "ui",
    "auth", "cache", "queue", "logging", "monitoring",
    "testing", "ci-cd", "architecture", "design-pattern",
}

// Generate 分析会话内容并返回相关标签
func (t *Tagger) Generate(messages []msg.Msg) ([]string, error) {
    // 提取关键内容
    var content strings.Builder
    for _, m := range messages {
        if m.Role == "user" || m.Role == "assistant" {
            content.WriteString(m.Text)
            content.WriteString(" ")
        }
    }
    
    // 本地规则匹配（快速路径）
    tags := t.matchRules(content.String())
    
    // 如果内容足够长，使用 LLM 生成更精确的标签
    if len(content.String()) > 200 && t.llm != nil {
        llmTags, err := t.generateWithLLM(content.String())
        if err == nil {
            tags = mergeTags(tags, llmTags)
        }
    }
    
    return uniqueTags(tags), nil
}

func (t *Tagger) matchRules(content string) []string {
    var tags []string
    lower := strings.ToLower(content)
    
    // 技术栈检测
    if containsAny(lower, []string{"go ", "golang", "go.mod"}) {
        tags = append(tags, "go")
    }
    if containsAny(lower, []string{"postgres", "postgresql", "psql"}) {
        tags = append(tags, "postgres")
    }
    if containsAny(lower, []string{"redis", "cache", "caching"}) {
        tags = append(tags, "redis")
    }
    
    // 任务类型检测
    if containsAny(lower, []string{"refactor", "重构"}) {
        tags = append(tags, "refactor")
    }
    if containsAny(lower, []string{"bug", "fix", "修复", "报错"}) {
        tags = append(tags, "bugfix")
    }
    if containsAny(lower, []string{"optimize", "performance", "slow", "瓶颈"}) {
        tags = append(tags, "performance")
    }
    
    return tags
}

func (t *Tagger) generateWithLLM(content string) ([]string, error) {
    prompt := fmt.Sprintf(`分析以下对话内容，生成 3-5 个最相关的标签。
从以下类别中选择：
- 编程语言: go, python, javascript, typescript, rust...
- 技术: postgres, redis, docker, kubernetes, react...
- 任务类型: refactor, bugfix, feature, optimize, test...
- 主题: performance, security, database, api, auth...

只返回标签列表，每行一个，不带 # 号：

%s`, truncate(content, 2000))

    resp, err := t.llm.Complete(context.Background(), prompt)
    if err != nil {
        return nil, err
    }
    
    var tags []string
    for _, line := range strings.Split(resp, "\n") {
        tag := strings.TrimSpace(strings.ToLower(line))
        tag = strings.TrimPrefix(tag, "#")
        tag = strings.TrimPrefix(tag, "-")
        tag = strings.TrimSpace(tag)
        if tag != "" && !strings.Contains(tag, " ") {
            tags = append(tags, tag)
        }
    }
    return tags, nil
}
```

#### 2. `internal/msg/msg.go` (修改)

在 Session 结构中添加元数据引用：
```go
type Session struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Messages  []Msg     `json:"messages"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    
    // 新增: 元数据（不持久化到 jsonl，单独存）
    Meta      *session.Meta `json:"-"`
}
```

#### 3. `internal/tui/tui.go` (修改)

添加新的命令处理：
```go
func (m *Model) handleCommand(text string) (tea.Cmd, bool) {
    // ... 现有命令 ...
    
    case "/tag":
        return m.handleTagCommand(parts)
        
    case "/tags":
        return m.handleListTags()
        
    case "/search":
        return m.handleSearchCommand(parts)
        
    case "/related":
        return m.handleRelatedCommand()
}

func (m *Model) handleTagCommand(parts []string) (tea.Cmd, bool) {
    if len(parts) < 2 {
        m.messages = append(m.messages, message{
            role: "system",
            content: `Usage:
  /tag <tag>           - 添加标签
  /tag add <tag>       - 同上
  /tag remove <tag>    - 移除标签
  /tag clear           - 清除所有手动标签`,
        })
        return nil, true
    }
    
    if m.sessionMgr == nil {
        m.messages = append(m.messages, message{
            role: "error",
            content: "Session manager not initialized",
        })
        return nil, true
    }
    
    action := parts[1]
    switch action {
    case "add":
        if len(parts) < 3 {
            m.showError("Usage: /tag add <tag>")
            return nil, true
        }
        tags := parts[2:]
        if err := m.sessionMgr.AddTags(m.session, tags); err == nil {
            m.showSystem(fmt.Sprintf("Added tags: %s", strings.Join(tags, ", ")))
        }
        
    case "remove", "rm":
        if len(parts) < 3 {
            m.showError("Usage: /tag remove <tag>")
            return nil, true
        }
        tags := parts[2:]
        if err := m.sessionMgr.RemoveTags(m.session, tags); err == nil {
            m.showSystem(fmt.Sprintf("Removed tags: %s", strings.Join(tags, ", ")))
        }
        
    case "clear":
        if err := m.sessionMgr.ClearManualTags(m.session); err == nil {
            m.showSystem("Cleared all manual tags")
        }
        
    default:
        // 默认添加标签
        tags := parts[1:]
        if err := m.sessionMgr.AddTags(m.session, tags); err == nil {
            m.showSystem(fmt.Sprintf("Added tags: %s", strings.Join(tags, ", ")))
        }
    }
    return nil, true
}

func (m *Model) handleSearchCommand(parts []string) (tea.Cmd, bool) {
    if len(parts) < 2 {
        m.messages = append(m.messages, message{
            role: "system",
            content: `Usage:
  /search <keyword>         - 全文搜索
  /search #tag1 #tag2       - 按标签搜索
  /search <keyword> #tag    - 组合搜索`,
        })
        return nil, true
    }
    
    // 解析查询
    var keywords []string
    var tags []string
    for _, p := range parts[1:] {
        if strings.HasPrefix(p, "#") {
            tags = append(tags, strings.TrimPrefix(p, "#"))
        } else {
            keywords = append(keywords, p)
        }
    }
    
    query := session.SearchQuery{
        Keywords: strings.Join(keywords, " "),
        Tags:     tags,
        Limit:    10,
    }
    
    results, err := m.sessionMgr.Search(query)
    if err != nil {
        m.showError(fmt.Sprintf("Search failed: %v", err))
        return nil, true
    }
    
    var content strings.Builder
    content.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))
    
    for i, r := range results {
        marker := "  "
        if r.Meta.ID == m.session {
            marker = "* "
        }
        
        title := r.Meta.Title
        if title == "" {
            title = "Untitled"
        }
        
        tags := ""
        if len(r.Meta.Tags) > 0 {
            tags = " " + formatTags(r.Meta.Tags)
        }
        
        content.WriteString(fmt.Sprintf("%s%d. %s%s\n", marker, i+1, title, tags))
        content.WriteString(fmt.Sprintf("   ID: %s | %s | %.0f%% match\n", 
            r.Meta.ID, 
            r.Meta.UpdatedAt.Format("01-02 15:04"),
            r.Score*100))
        
        if r.MatchedText != "" {
            snippet := truncate(r.MatchedText, 80)
            content.WriteString(fmt.Sprintf("   \"%s...\"\n", snippet))
        }
        content.WriteString("\n")
    }
    
    m.messages = append(m.messages, message{
        role:    "system",
        content: content.String(),
    })
    m.input.Reset()
    m.updateViewport()
    return nil, true
}

// 更新 /sessions 命令显示标签
func (m *Model) handleSessions() (tea.Cmd, bool) {
    sessions := m.bus.ListSessions()
    metas, _ := m.sessionMgr.List()
    
    // 构建 ID -> Meta 映射
    metaMap := make(map[string]*session.Meta)
    for i := range metas {
        metaMap[metas[i].ID] = &metas[i]
    }
    
    var content strings.Builder
    content.WriteString("Sessions:\n")
    
    for _, s := range sessions {
        marker := "  "
        if s.ID == m.session {
            marker = "* "
        }
        
        title := s.Title
        if title == "" {
            title = "Untitled"
        }
        
        // 添加标签信息
        tags := ""
        if meta, ok := metaMap[s.ID]; ok && len(meta.Tags) > 0 {
            tags = " " + formatTags(meta.Tags[:min(3, len(meta.Tags))])
        }
        
        content.WriteString(fmt.Sprintf("%s%s - %s%s\n", marker, s.ID, title, tags))
    }
    
    if len(sessions) == 0 {
        content.WriteString("No sessions found.")
    }
    
    m.messages = append(m.messages, message{role: "system", content: content.String()})
    m.input.Reset()
    m.updateViewport()
    return nil, true
}
```

#### 4. `internal/config/config.go` (修改)

添加会话管理配置：
```toml
[session]
auto_tag = true          # 自动生成标签
max_tags = 8             # 最大标签数
search_limit = 20        # 默认搜索结果数
index_on_create = true   # 创建会话时自动索引
```

#### 5. `main.go` (修改)

初始化 Session Manager：
```go
func main() {
    // ... 现有初始化 ...
    
    // 初始化会话管理器
    sessionMgr, err := session.NewManager(config.SessionsDir())
    if err != nil {
        log.Fatal("failed to init session manager:", err)
    }
    
    // 创建 Agent
    a := agent.New(llmClient, tools)
    
    // 创建 TUI
    bus := msg.NewBus(config.SessionsDir())
    m := tui.New(a, tools, bus, sessionMgr)
    
    // ...
}
```

---

## 接口设计

### TUI 命令

| 命令 | 功能 | 示例 |
|------|------|------|
| `/tag <tag>` | 添加标签 | `/tag go performance` |
| `/tag add <tag>` | 同上 | `/tag add bugfix` |
| `/tag remove <tag>` | 移除标签 | `/tag remove wip` |
| `/tag clear` | 清除手动标签 | `/tag clear` |
| `/tags` | 列出所有可用标签 | `/tags` |
| `/search <query>` | 全文搜索 | `/search redis 缓存` |
| `/search #tag` | 按标签搜索 | `/search #go #performance` |
| `/related` | 显示相关会话 | `/related` |
| `/sessions` | 增强列表（显示标签） | `/sessions` |

### 配置项

```toml
[session]
auto_tag = true
max_tags = 8
search_limit = 20
index_on_create = true

# 自定义标签库（可选）
[session.custom_tags]
go = ["golang", "gin", "gorm"]
frontend = ["react", "vue", "css"]
```

### 文件结构

```
~/.config/otter/sessions/
├── search.db                 # SQLite FTS 索引
├── {session_id}/
│   ├── session.jsonl         # 消息记录
│   └── meta.json             # 元数据
│       {
│         "id": "20250211_143022",
│         "title": "Redis 缓存设计",
│         "tags": ["redis", "performance"],
│         "auto_tags": ["go", "cache"],
│         "summary": "讨论了 Redis 缓存策略...",
│         "message_count": 24,
│         "token_used": 15234,
│         "project_path": "/home/user/project",
│         "created_at": "2025-02-11T14:30:22Z",
│         "updated_at": "2025-02-11T15:12:08Z"
│       }
```

---

## 实现细节

### 标签生成流程

```
会话结束/手动触发
      │
      ▼
提取会话文本内容
      │
      ├── 用户消息
      ├── AI 回复
      └── 工具调用结果
      │
      ▼
规则匹配 (快速)
      │
      ├── 关键词匹配 → go, redis, postgres
      ├── 文件扩展名 → .go, .py, .js
      └── 工具使用 → git, shell, edit
      │
      ▼
LLM 生成 (异步，仅长会话)
      │
      ├── 发送摘要给 LLM
      └── 返回精确标签
      │
      ▼
合并去重
      │
      ▼
存储到 meta.json + SQLite
```

### 搜索算法

```go
// 混合评分算法
func calculateScore(matchType string, recency float64) float64 {
    // 基础分数
    base := 0.0
    switch matchType {
    case "title_match":
        base = 1.0
    case "content_match":
        base = 0.8
    case "tag_match":
        base = 0.6
    }
    
    // 时间衰减 (越新越高)
    timeBoost := math.Exp(-recency / 7) // 7天半衰期
    
    return base * (0.5 + 0.5*timeBoost)
}
```

### 相关会话发现

```sql
-- 基于共享标签找相关会话
WITH session_tags AS (
    SELECT tag FROM tags WHERE session_id = ?
)
SELECT s.*, COUNT(t.tag) as shared_tags
FROM sessions s
JOIN tags t ON s.id = t.session_id
WHERE t.tag IN (SELECT tag FROM session_tags)
  AND s.id != ?
GROUP BY s.id
ORDER BY shared_tags DESC, s.updated_at DESC
LIMIT ?;
```

---

## 验收标准

### 功能测试

- [ ] `/tag go` 为当前会话添加标签
- [ ] `/tag remove go` 移除标签
- [ ] `/search redis` 返回包含 redis 的会话
- [ ] `/search #go` 返回带 #go 标签的会话
- [ ] `/search cache #go` 组合搜索工作正常
- [ ] `/related` 显示与当前会话相关的历史会话
- [ ] `/sessions` 显示会话标签
- [ ] 新会话自动生成标签
- [ ] 标签持久化，重启后保留
- [ ] 支持中英文标签搜索
- [ ] 搜索结果按相关度排序

### 边界测试

- [ ] 空会话处理
- [ ] 超长内容索引不阻塞
- [ ] 特殊字符标签处理
- [ ] 并发修改标签安全
- [ ] 损坏的 meta.json 自动重建
- [ ] 1000+ 会话搜索性能 < 500ms

### 性能测试

- [ ] 标签生成 < 2s（长会话）
- [ ] 全文搜索 < 500ms
- [ ] 索引建立 < 1s（普通会话）
- [ ] 内存占用增长 < 50MB

### 代码质量

- [ ] 遵循 CLAUDE.md 代码风格
- [ ] 单元测试覆盖率 > 70%
- [ ] 错误处理完整
- [ ] SQLite 并发访问安全

---

## 参考

### 相关项目

1. **ChatGPT**: 会话搜索
   - 标题和内容的简单搜索

2. **Cursor**: 标签系统
   - 文件夹 + 标签组织

3. **Notion**: 数据库筛选
   - 多维度筛选设计

### 技术文档

1. **SQLite FTS5**
   - https://www.sqlite.org/fts5.html

2. **Go SQLite Driver**
   - https://github.com/mattn/go-sqlite3

3. **Porter Stemmer** (词干提取)
   - FTS5 内置支持

### 代码参考

FTS 查询示例：
```sql
-- 基础搜索
SELECT * FROM sessions_fts WHERE sessions_fts MATCH 'redis cache';

-- 短语搜索
SELECT * FROM sessions_fts WHERE sessions_fts MATCH '"redis cluster"';

-- 前缀搜索
SELECT * FROM sessions_fts WHERE sessions_fts MATCH 'redis*';
```

---

## 里程碑

### Phase 1: MVP (核心功能)
- [ ] 会话元数据存储
- [ ] 基础标签系统（手动）
- [ ] SQLite FTS 索引
- [ ] `/tag`, `/search` 命令
- [ ] 增强 `/sessions`

### Phase 2: 智能功能
- [ ] 自动标签生成
- [ ] 会话摘要生成
- [ ] 相关会话推荐
- [ ] 标签库扩展

### Phase 3: 优化
- [ ] 搜索性能优化
- [ ] 标签建议
- [ ] 会话统计
- [ ] 导出功能

---

*PRD 版本: 1.0*  
*创建日期: 2025-02-11*  
*作者: otter-dev*
