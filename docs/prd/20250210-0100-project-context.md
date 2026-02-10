# Feature: Project Context (项目上下文记忆)

## 背景与动机

### 现状问题

Otter 目前每次对话都是"无状态"的，即使用户在同一个项目中工作多次，每次都需要重新：

1. **重新探索项目结构** - "这是什么项目？使用什么语言？"
2. **重新了解构建命令** - "怎么运行测试？构建命令是什么？"
3. **重新告知编码规范** - "这个项目使用什么代码风格？"
4. **重复定位关键文件** - "配置文件在哪里？入口文件是什么？"

### 实际场景

```
第1天:
用户: 帮我添加一个用户认证功能
Otter: [扫描项目结构] 这是一个 Go + Gin 项目，使用 GORM...
      [15分钟后理解项目]

第3天:
用户: 继续完善认证功能，添加 JWT
Otter: [再次扫描项目结构] 这是一个 Go 项目...
      [又花15分钟重新了解]
```

### 为什么需要这个功能

1. **连续性**: 跨会话保持项目理解，像人类一样"记住"项目
2. **效率**: 减少重复探索，直接基于已有上下文工作
3. **一致性**: 确保编码风格、架构决策在多轮对话中保持一致
4. **个性化**: 学习用户在该项目中的偏好和习惯

### 竞品对比

| 工具 | 项目记忆 | 说明 |
|------|----------|------|
| Claude Code | ⚠️ 有限 | 依赖会话历史，无结构化项目记忆 |
| Cursor | ✅ | 有项目索引，但偏代码搜索 |
| GitHub Copilot | ✅ | 基于项目文件的上下文 |
| Otter | ❌ | **待实现** |

---

## 功能描述

### 核心功能

1. **项目自动识别**: 基于 git 仓库或特定标记文件自动识别项目
2. **上下文持久化**: 将项目关键信息保存到本地数据库
3. **智能注入**: 自动将相关上下文注入到系统 prompt
4. **动态更新**: 随着项目变化自动更新上下文

### 存储的信息类型

```yaml
project_context:
  # 基础信息
  name: "otter"
  path: "/home/user/workspace/otter"
  type: "go"  # 自动检测: go, python, node, rust, etc.
  
  # 技术栈
  tech_stack:
    language: "Go"
    framework: "BubbleTea"
    key_dependencies: ["langchaingo", "toml"]
  
  # 项目结构记忆
  structure:
    entry_points: ["main.go"]
    config_files: ["config.toml", ".env"]
    source_dirs: ["internal/", "cmd/"]
    test_dirs: ["internal/**/*_test.go"]
    important_files:
      - "CLAUDE.md"  # 代码规范
      - "README.md"
  
  # 常用命令
  commands:
    build: "go build -o otter"
    test: "go test ./..."
    lint: "golangci-lint run"
    run: "go run main.go"
  
  # 代码规范
  conventions:
    style_guide: "CLAUDE.md"
    naming: "short_names"  # 从代码中学习的偏好
    patterns:
      - "error handling: immediate return"
      - "interface: use sparingly"
  
  # 会话历史记忆
  recent_topics:
    - date: "2025-02-09"
      summary: "添加了 websearch 和 git 工具"
    - date: "2025-02-08"
      summary: "重构了 tool 接口"
  
  # 用户偏好
  preferences:
    response_style: "concise"  # 用户偏好的回复风格
    verify_after_edit: true    # 是否在修改后自动验证
```

### 使用场景

```
场景1: 连续开发
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
第1天:
用户: 帮我实现一个批量编辑功能
Otter: [学习项目] 这是一个 Go 项目，使用简洁的函数式风格...
      [实现功能]

第3天 (新会话):
用户: 继续完善那个批量编辑功能，添加错误处理
Otter: [自动加载上下文] 
       "继续处理 batch_edit 工具，基于之前的实现添加错误处理...
        检测到项目使用即时错误返回模式，将遵循此风格。"
       [直接开始编码，无需重新了解项目]

场景2: 跨项目切换
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: (在 ~/workspace/otter) 帮我优化代码
Otter: [识别项目: otter, Go CLI 工具]
       "基于 otter 的代码风格（短命名、函数式）进行优化..."

用户: /cd ~/workspace/myweb
用户: 修复这个 bug
Otter: [识别项目: myweb, React + Node.js]
       "切换到 myweb 项目上下文（React Hooks, TypeScript）..."

场景3: 智能建议
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 怎么运行测试？
Otter: [查找上下文] 这个项目使用 `go test ./...`，
       需要我帮你运行吗？

用户: 这个项目的编码规范是什么？
Otter: [读取 CLAUDE.md] 根据 CLAUDE.md:
       - 短命名（i, s *Session）
       - 少用 interface
       - 组合优于继承
       - 立即处理错误
```

---

## 技术方案

### 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                           TUI                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ 用户输入     │  │ 项目检测     │  │ 上下文管理命令      │  │
│  │ /project     │  │ (git/文件)   │  │ /context, /forget   │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Project Context Manager                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ 项目识别     │  │ 上下文组装   │  │ 智能更新            │  │
│  │ Identify()   │  │ Build()      │  │ Update()            │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Context Injector                                         │  │
│  │  自动将上下文注入 System Prompt                           │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Storage Layer                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  ~/.config/otter/contexts/                                 │  │
│  │  ├── projects/                                             │  │
│  │  │   ├── otter_abc123.json    # 项目上下文                │  │
│  │  │   ├── myweb_def456.json                                 │  │
│  │  │   └── ...                                               │  │
│  │  └── index.db                # SQLite 索引                │  │
│  │       (项目路径 → 上下文文件映射)                          │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 文件改动清单

#### 1. `internal/context/` (新增目录)

**`internal/context/context.go`** - 核心接口和类型
```go
package context

import "time"

// ProjectContext 存储单个项目的完整上下文
type ProjectContext struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Path        string                 `json:"path"`
    Type        string                 `json:"type"`  // go, python, node, etc.
    TechStack   TechStack              `json:"tech_stack"`
    Structure   ProjectStructure       `json:"structure"`
    Commands    ProjectCommands        `json:"commands"`
    Conventions CodeConventions        `json:"conventions"`
    Recent      []TopicSummary         `json:"recent"`
    Preferences UserPreferences        `json:"preferences"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
    AccessCount int                    `json:"access_count"`
}

type TechStack struct {
    Language        string   `json:"language"`
    Framework       string   `json:"framework,omitempty"`
    KeyDependencies []string `json:"key_dependencies,omitempty"`
}

type ProjectStructure struct {
    EntryPoints    []string `json:"entry_points"`
    ConfigFiles    []string `json:"config_files"`
    SourceDirs     []string `json:"source_dirs"`
    TestDirs       []string `json:"test_dirs"`
    ImportantFiles []string `json:"important_files"`
}

type ProjectCommands struct {
    Build string `json:"build,omitempty"`
    Test  string `json:"test,omitempty"`
    Lint  string `json:"lint,omitempty"`
    Run   string `json:"run,omitempty"`
}

type CodeConventions struct {
    StyleGuide string   `json:"style_guide,omitempty"`
    Patterns   []string `json:"patterns,omitempty"`
}

type TopicSummary struct {
    Date    string `json:"date"`
    Summary string `json:"summary"`
}

type UserPreferences struct {
    ResponseStyle   string `json:"response_style,omitempty"`
    VerifyAfterEdit bool   `json:"verify_after_edit"`
}
```

**`internal/context/manager.go`** - 上下文管理器
```go
package context

// Manager 管理项目上下文的生命周期
type Manager struct {
    store  *Store
    loader *Loader
    cache  *Cache
}

func NewManager(configDir string) *Manager

// Identify 识别当前目录对应的项目
func (m *Manager) Identify(workDir string) (*ProjectContext, error)

// Load 加载指定项目的上下文
func (m *Manager) Load(projectPath string) (*ProjectContext, error)

// Save 保存项目上下文
func (m *Manager) Save(ctx *ProjectContext) error

// Build 构建/更新项目上下文
func (m *Manager) Build(workDir string) (*ProjectContext, error)

// Inject 生成用于注入到 prompt 的上下文文本
func (m *Manager) Inject(ctx *ProjectContext) string

// List 列出所有已知的项目
func (m *Manager) List() ([]ProjectInfo, error)

// Forget 删除项目上下文
func (m *Manager) Forget(projectID string) error
```

**`internal/context/loader.go`** - 项目分析加载器
```go
package context

// Loader 分析项目结构并提取信息
type Loader struct{}

func (l *Loader) DetectType(workDir string) string
// - 检测 go.mod → "go"
// - 检测 package.json → "node"
// - 检测 requirements.txt/pyproject.toml → "python"
// - 检测 Cargo.toml → "rust"

func (l *Loader) AnalyzeStructure(workDir string, projType string) ProjectStructure
// - 查找入口文件 (main.go, index.js, etc.)
// - 查找配置文件
// - 识别源码目录结构

func (l *Loader) ExtractTechStack(workDir string, projType string) TechStack
// - 读取依赖文件
// - 识别主要框架

func (l *Loader) DetectCommands(workDir string, projType string) ProjectCommands
// - 检测 Makefile
// - 读取 package.json scripts
// - 识别 go test 模式

func (l *Loader) LearnConventions(workDir string) CodeConventions
// - 扫描现有代码风格
// - 检测命名约定
// - 读取 CLAUDE.md 等规范文件
```

**`internal/context/store.go`** - 存储层
```go
package context

import "database/sql"

// Store 处理上下文的持久化
type Store struct {
    db       *sql.DB
    dataDir  string
}

func NewStore(dataDir string) (*Store, error)

func (s *Store) Save(ctx *ProjectContext) error
func (s *Store) Load(projectPath string) (*ProjectContext, error)
func (s *Store) List() ([]ProjectInfo, error)
func (s *Store) Delete(projectID string) error
func (s *Store) UpdateAccess(projectID string) error
```

#### 2. `internal/prompt/prompt.go` (修改)

修改 `Load` 函数，支持注入项目上下文：

```go
func Load(tools *tool.Set, maxSteps int, mode string, projCtx *context.ProjectContext) string {
    // ... 现有代码 ...
    
    contextSection := ""
    if projCtx != nil {
        contextSection = buildContextSection(projCtx)
    }
    
    return fmt.Sprintf(defaultPrompt, 
        wd, runtime.GOOS, date, toolList.String(), 
        maxSteps, contextSection)
}

func buildContextSection(ctx *context.ProjectContext) string {
    var b strings.Builder
    b.WriteString("\n\n## Project Context\n\n")
    b.WriteString(fmt.Sprintf("Working on: **%s** (%s project)\n", 
        ctx.Name, ctx.Type))
    
    if ctx.TechStack.Framework != "" {
        b.WriteString(fmt.Sprintf("Framework: %s\n", ctx.TechStack.Framework))
    }
    
    if len(ctx.Structure.EntryPoints) > 0 {
        b.WriteString(fmt.Sprintf("Entry: %s\n", 
            strings.Join(ctx.Structure.EntryPoints, ", ")))
    }
    
    if ctx.Commands.Test != "" {
        b.WriteString(fmt.Sprintf("Test: `%s`\n", ctx.Commands.Test))
    }
    
    if ctx.Conventions.StyleGuide != "" {
        b.WriteString(fmt.Sprintf("\nStyle: Follow %s\n", ctx.Conventions.StyleGuide))
    }
    
    return b.String()
}
```

#### 3. `internal/agent/agent.go` (修改)

在 Agent 中添加 ContextManager：

```go
type Agent struct {
    llm      *llm.LLM
    tools    *tool.Set
    ctxMgr   *context.Manager  // 新增
    maxSteps int
    mode     string
}

func New(l *llm.LLM, t *tool.Set, ctxMgr *context.Manager) *Agent {
    // ...
}

func (a *Agent) systemPrompt() string {
    // 获取当前项目上下文
    projCtx, _ := a.ctxMgr.Identify(getWorkDir())
    return prompt.Load(a.tools, a.maxSteps, a.mode, projCtx)
}

// AfterTask 在任务完成后更新上下文记忆
func (a *Agent) AfterTask(summary string) {
    projCtx, _ := a.ctxMgr.Identify(getWorkDir())
    if projCtx != nil {
        projCtx.Recent = append([]context.TopicSummary{{
            Date:    time.Now().Format("2006-01-02"),
            Summary: summary,
        }}, projCtx.Recent...)
        if len(projCtx.Recent) > 10 {
            projCtx.Recent = projCtx.Recent[:10]
        }
        a.ctxMgr.Save(projCtx)
    }
}
```

#### 4. `internal/tui/tui.go` (修改)

添加上下文相关命令：

```go
// 新增命令处理
case "/context":
    // 显示当前项目上下文
    m.showContext()
case "/forget":
    // 忘记当前项目上下文
    m.forgetContext()
case "/projects":
    // 列出所有已知项目
    m.listProjects()
case "/learn":
    // 重新学习当前项目
    m.rebuildContext()
```

在 TUI 中添加上下文指示器：
```
┌──────────────────────────────────────────┐
│ 🦦 Otter  │  Project: otter (Go)  📝    │
├──────────────────────────────────────────┤
│                                          │
│   [chat area]                            │
│                                          │
├──────────────────────────────────────────┤
│ > _                                      │
└──────────────────────────────────────────┘
```

#### 5. `internal/config/config.go` (修改)

添加上下文相关配置：

```toml
[context]
enabled = true                    # 启用项目上下文
auto_learn = true                 # 自动学习新项目
max_recent_topics = 10            # 保留多少条近期话题
inject_to_prompt = true           # 是否注入到 prompt
context_file = "CLAUDE.md"        # 优先读取的规范文件
```

---

## 接口设计

### TUI 命令

| 命令 | 功能 |
|------|------|
| `/context` | 显示当前项目上下文信息 |
| `/forget` | 忘记当前项目的上下文记忆 |
| `/projects` | 列出所有已知的项目 |
| `/learn` | 重新分析并学习当前项目 |
| `/switch <project>` | 切换到指定项目的上下文 |

### 配置项

```toml
[context]
enabled = true
auto_learn = true
max_recent_topics = 10
inject_to_prompt = true
context_file = "CLAUDE.md"

# 项目类型检测规则（可扩展）
[context.detectors]
go = ["go.mod"]
node = ["package.json"]
python = ["requirements.txt", "pyproject.toml", "setup.py"]
rust = ["Cargo.toml"]
```

### 环境变量

```bash
export OTTER_CONTEXT_ENABLED=1
export OTTER_CONTEXT_DIR="$HOME/.config/otter/contexts"
```

---

## 实现细节

### 项目识别流程

```
用户启动 Otter 或切换目录
         │
         ▼
检测当前工作目录
         │
    ┌────┴────┐
    ▼         ▼
 是Git仓库   否
    │         │
    ▼         ▼
计算仓库ID   使用路径hash
(git remote + path)
    │
    ▼
查询数据库
    │
 ┌──┴──┐
 ▼     ▼
存在  不存在
 │     │
 ▼     ▼
加载   询问用户
上下文 是否学习
 │     │
 ▼     ▼
注入   调用Build()
到Prompt │
         ▼
        保存
```

### 上下文学习流程

```
Build(workDir)
    │
    ├── 1. DetectType()
    │      ├── 检查 go.mod
    │      ├── 检查 package.json
    │      └── ...
    │
    ├── 2. AnalyzeStructure()
    │      ├── 查找 main 函数
    │      ├── 识别源码目录
    │      └── 查找配置文件
    │
    ├── 3. ExtractTechStack()
    │      ├── 解析依赖文件
    │      └── 识别框架
    │
    ├── 4. DetectCommands()
    │      ├── 检查 Makefile
    │      ├── 检查 package.json
    │      └── 根据类型推断
    │
    └── 5. LearnConventions()
           ├── 查找 CLAUDE.md
           ├── 扫描代码风格
           └── 分析命名模式
```

### 上下文注入格式

```markdown
## Project Context

Working on: **otter** (go project)
Framework: BubbleTea
Entry: main.go
Test: `go test ./...`

Style: Follow CLAUDE.md
- Short naming (i, s *Session)
- Use interface sparingly
- Handle errors immediately

Recent work:
- 2025-02-09: Added websearch and git tools
- 2025-02-08: Refactored tool interface
```

---

## 验收标准

### 功能测试

- [ ] 首次进入项目时自动检测并询问是否学习
- [ ] 学习后的项目在下次进入时自动加载上下文
- [ ] `/context` 命令正确显示项目信息
- [ ] `/forget` 命令清除项目记忆
- [ ] `/projects` 列出所有已知项目
- [ ] 上下文正确注入到 LLM 的系统 prompt
- [ ] 跨会话保持项目理解（第2天进入仍能识别）
- [ ] 支持 Go 项目的自动检测
- [ ] 支持 Node.js 项目的自动检测
- [ ] 支持 Python 项目的自动检测
- [ ] 正确识别项目的入口文件
- [ ] 正确识别项目的测试命令
- [ ] 读取并应用 CLAUDE.md 中的规范

### 边界测试

- [ ] 非项目目录正常工作（无上下文注入）
- [ ] 权限不足时优雅降级
- [ ] 项目移动位置后能重新关联
- [ ] 大型项目（10000+ 文件）学习不阻塞
- [ ] 损坏的上下文文件可自动重建

### 性能测试

- [ ] 上下文加载 < 50ms
- [ ] 项目学习 < 5s（普通项目）
- [ ] 数据库查询 < 10ms
- [ ] 不显著增加内存占用（< 50MB）

### 代码质量

- [ ] 遵循 CLAUDE.md 代码风格
- [ ] 完整错误处理
- [ ] 单元测试覆盖率 > 70%
- [ ] 不引入重量级依赖（只用标准库 + sqlite）

---

## 参考

### 相关项目

1. **Claude Code**: 项目理解实现
   - 基于目录结构的学习

2. **Cursor**: 项目索引
   - 代码库索引和搜索

3. **aider**: 多文件编辑
   - 项目级别的上下文管理

### 技术文档

1. **SQLite in Go**
   - https://pkg.go.dev/modernc.org/sqlite
   - 纯 Go 实现，无 CGO

2. **Git 仓库识别**
   - `git remote get-url origin`
   - `git rev-parse --show-toplevel`

### 代码参考

项目检测逻辑：
```go
func detectProjectType(dir string) string {
    if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
        return "go"
    }
    if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
        return "node"
    }
    // ... more detectors
    return "unknown"
}
```

---

## 里程碑

### Phase 1: MVP (基础功能)
- [ ] 项目识别与存储
- [ ] Go/Node/Python 类型检测
- [ ] 基础上下文注入
- [ ] `/context`, `/forget` 命令

### Phase 2: 完善
- [ ] 自动学习新项目
- [ ] 代码风格学习
- [ ] 近期话题记忆
- [ ] `/projects`, `/learn` 命令

### Phase 3: 优化
- [ ] 智能命令建议
- [ ] 跨项目代码复用建议
- [ ] 项目健康度检查

---

*PRD 版本: 1.0*  
*创建日期: 2025-02-10*  
*作者: otter-dev*
