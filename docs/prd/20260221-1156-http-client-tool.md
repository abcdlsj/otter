# Feature: HTTP Client Tool

## 背景与动机

### 当前问题

开发者在日常工作中频繁需要与 HTTP API 交互，包括调试接口、测试 webhook、调用 REST/GraphQL 服务等。虽然 Otter 已有 `webfetch` 和 `websearch` 工具，但存在以下局限：

1. **功能单一**：只能执行 GET 请求，无法发送 POST/PUT/DELETE 等其他 HTTP 方法
2. **无请求定制**：无法设置自定义 headers、查询参数、请求体
3. **不支持认证**：无法处理 Bearer Token、Basic Auth、API Key 等常见认证方式
4. **响应处理弱**：只能获取原始内容，无法解析 JSON、查看状态码、响应头
5. **无请求历史**：每次请求都是独立的，无法保存和复用常用请求

### 实际场景

```
场景1: 调试 REST API
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 登录接口返回 401，帮我检查一下
现状: curl -X POST https://api.example.com/login \
      -H "Content-Type: application/json" \
      -d '{"email":"test@example.com","password":"wrong"}'
问题: curl 命令复杂难记，参数容易出错，输出不友好

场景2: 测试 GraphQL API
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 查询用户详情
现状: 手动构造 GraphQL query，用 curl 发送
问题: GraphQL body 格式特殊，容易出错

场景3: 调用带认证的 API
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 用 Bearer token 调用 GitHub API
现状: curl -H "Authorization: Bearer $TOKEN" ...
问题: 需要手动处理认证头，token 容易泄露到日志

场景4: Webhook 调试
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 调试一个 webhook 回调
现状: 启动本地服务器，使用 ngrok 暴露
问题: 步骤繁琐，无法快速验证请求格式

场景5: 环境切换
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
用户: 在 dev/staging/prod 环境间切换
现状: 每次手动修改 URL 前缀
问题: 重复繁琐，容易改错环境
```

### 用户价值

- **统一接口**：一个工具搞定所有 HTTP 请求类型
- **智能认证**：内置常见认证方式，安全可靠
- **友好输出**：格式化 JSON、自动高亮关键信息
- **请求复用**：保存常用请求，快速执行
- **环境管理**：多环境配置，轻松切换

### 与现有工具的关系

`http` 工具是 `webfetch` 的超集，提供更强大的功能：

| 特性 | webfetch | http |
|------|----------|------|
| GET | ✅ | ✅ |
| POST/PUT/DELETE | ❌ | ✅ |
| 自定义 Headers | ❌ | ✅ |
| 请求体 (JSON/Form) | ❌ | ✅ |
| 认证支持 | ❌ | ✅ |
| 状态码解析 | ❌ | ✅ |
| 响应头查看 | ❌ | ✅ |
| 请求历史 | ❌ | ✅ |

## 功能描述

### 核心功能

#### 1. HTTP 方法支持
- **GET**: 获取资源
- **POST**: 创建资源
- **PUT**: 完整更新
- **PATCH**: 部分更新
- **DELETE**: 删除资源
- **HEAD**: 只获取响应头
- **OPTIONS**: 获取支持的 HTTP 方法

#### 2. 请求构建
- **URL**: 支持路径参数、查询字符串
- **Headers**: 自定义请求头
- **Query Params**: 自动编码的查询参数
- **Body Types**:
  - JSON (application/json)
  - Form (application/x-www-form-urlencoded)
  - Multipart (multipart/form-data)
  - Raw Text
  - Binary

#### 3. 认证支持
- **Bearer Token**: `Authorization: Bearer <token>`
- **Basic Auth**: `Authorization: Basic <base64>`
- **API Key**: 支持 Header 或 Query 参数方式
- **OAuth 2.0**: 简化支持（client credentials flow）

#### 4. 响应处理
- **状态码**: 显示 HTTP 状态码及含义
- **响应头**: 查看响应 Headers
- **响应体**: 格式化 JSON/XML/HTML/Plain text
- **时间统计**: 请求耗时分析
- **大小统计**: 响应体大小

#### 5. 请求管理
- **保存请求**: 命名并保存常用请求
- **请求历史**: 自动记录所有请求（带时间戳）
- **环境变量**: 定义环境（dev/staging/prod）及变量
- **导入/导出**: 支持 cURL、Postman 格式导入

#### 6. 高级功能
- **重试机制**: 请求失败时自动重试
- **超时控制**: 设置请求超时时间
- **Follow Redirects**: 自动跟随重定向
- **SSL 验证**: 控制 SSL 证书验证
- **代理支持**: 通过代理发送请求

### 使用示例

#### 示例 1：简单 GET 请求
```json
{
  "method": "GET",
  "url": "https://api.github.com/users/octocat"
}
```

输出：
```
GET https://api.github.com/users/octocat
─────────────────────────────────────────────────────
Status: 200 OK (HTTP/1.1)
Time: 234ms
Size: 2.1 KB

{
  "login": "octocat",
  "id": 1,
  "avatar_url": "https://github.com/images/error/octocat_happy.gif",
  "type": "User",
  "name": "monalisa octocat",
  ...
}
```

#### 示例 2：POST JSON 请求
```json
{
  "method": "POST",
  "url": "https://api.example.com/users",
  "headers": {
    "Content-Type": "application/json",
    "Accept": "application/json"
  },
  "body": {
    "name": "John Doe",
    "email": "john@example.com"
  }
}
```

输出：
```
POST https://api.example.com/users
─────────────────────────────────────────────────────
Request Headers:
  Content-Type: application/json
  Accept: application/json

Request Body:
{
  "name": "John Doe",
  "email": "john@example.com"
}

─────────────────────────────────────────────────────
Status: 201 Created (HTTP/1.1)
Time: 156ms

{
  "id": 123,
  "name": "John Doe",
  "email": "john@example.com",
  "created_at": "2026-02-21T12:00:00Z"
}
```

#### 示例 3：带 Bearer Token
```json
{
  "method": "GET",
  "url": "https://api.example.com/private/data",
  "auth": {
    "type": "bearer",
    "token": "$GITHUB_TOKEN"
  }
}
```

输出：
```
GET https://api.example.com/private/data
─────────────────────────────────────────────────────
Authorization: Bearer ******** (resolved from $GITHUB_TOKEN)

Status: 200 OK
Time: 89ms
...
```

#### 示例 4：保存和复用请求
```json
{
  "save": "list-users",
  "method": "GET",
  "url": "{{base_url}}/users",
  "headers": {
    "Authorization": "Bearer {{token}}"
  }
}
```

后续调用：
```json
{
  "call": "list-users"
}
```

#### 示例 5：GraphQL 请求
```json
{
  "method": "POST",
  "url": "https://api.github.com/graphql",
  "headers": {
    "Authorization": "Bearer $GITHUB_TOKEN"
  },
  "body": {
    "query": "query { viewer { login name } }"
  }
}
```

#### 示例 6：环境切换
```json
{
  "env": "staging",
  "method": "GET",
  "url": "{{base_url}}/health"
}
```

其中 `staging` 环境定义：
```json
{
  "environments": {
    "dev": {
      "base_url": "http://localhost:3000",
      "token": "$DEV_TOKEN"
    },
    "staging": {
      "base_url": "https://staging.example.com",
      "token": "$STAGING_TOKEN"
    },
    "prod": {
      "base_url": "https://api.example.com",
      "token": "$PROD_TOKEN"
    }
  }
}
```

## 技术方案

### 文件结构

```
internal/
├── http/                        # 新增包
│   ├── client.go                # HTTP 客户端封装
│   ├── request.go               # 请求构建器
│   ├── response.go              # 响应处理
│   ├── auth.go                  # 认证处理
│   ├── formatter.go             # 输出格式化
│   ├── history.go               # 请求历史管理
│   ├── storage.go               # 保存请求存储
│   └── env.go                   # 环境变量管理
└── tool/
    └── http.go                  # HTTP 工具实现
```

### 核心实现

#### 1. HTTP Client

```go
// internal/http/client.go

type Client struct {
    client  *http.Client
    timeout time.Duration
}

func NewClient(timeout time.Duration) *Client {
    return &Client{
        client: &http.Client{
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                return http.ErrUseLastResponse // 不自动跟随重定向
            },
            Timeout: timeout,
        },
        timeout: timeout,
    }
}

func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
    // 构建 http.Request
    // 添加认证头
    // 发送请求
    // 记录耗时
    // 返回结构化响应
}
```

#### 2. Request 构建器

```go
// internal/http/request.go

type Request struct {
    Method  string
    URL     string
    Headers map[string]string
    Query   map[string]string
    Body    interface{}
    Auth    *Auth
    Timeout time.Duration
}

type Auth struct {
    Type   string // "bearer", "basic", "apikey", "oauth2"
    Token  string
    Key    string
    In     string // "header" or "query" (for apikey)
}
```

#### 3. 响应处理

```go
// internal/http/response.go

type Response struct {
    StatusCode int
    Status     string
    Proto      string
    Headers    map[string]string
    Body       []byte
    Duration   time.Duration
    Size       int64
}

func (r *Response) FormatJSON() (string, error) {
    // 格式化 JSON 响应
    // 缩进、颜色高亮
}

func (r *Response) IsSuccess() bool {
    return r.StatusCode >= 200 && r.StatusCode < 300
}
```

#### 4. 认证处理

```go
// internal/http/auth.go

func (r *Request) ApplyAuth() error {
    if r.Auth == nil {
        return nil
    }

    switch r.Auth.Type {
    case "bearer":
        r.Headers["Authorization"] = "Bearer " + r.Auth.Token
    case "basic":
        encoded := base64.StdEncoding.EncodeToString(
            []byte(r.Auth.Username+":"+r.Auth.Password))
        r.Headers["Authorization"] = "Basic " + encoded
    case "apikey":
        if r.Auth.In == "query" {
            r.Query[r.Auth.Key] = r.Auth.Token
        } else {
            r.Headers[r.Auth.Key] = r.Auth.Token
        }
    }
    return nil
}
```

#### 5. HTTP Tool

```go
// internal/tool/http.go

type HTTP struct {
    client  *http.Client
    history *History
    storage *Storage
    env     *Env
}

func (HTTP) Name() string { return "http" }
func (HTTP) Desc() string {
    return "Send HTTP requests (REST/GraphQL). Supports GET, POST, PUT, DELETE with authentication."
}

func (HTTP) Args() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "method": map[string]any{
                "type": "string",
                "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
                "description": "HTTP method",
            },
            "url": map[string]any{
                "type": "string",
                "description": "Request URL (supports {{variable}} interpolation)",
            },
            "headers": map[string]any{
                "type": "object",
                "description": "Request headers",
            },
            "query": map[string]any{
                "type": "object",
                "description": "Query parameters",
            },
            "body": map[string]any{
                "type": "object",
                "description": "Request body (object will be JSON encoded)",
            },
            "auth": map[string]any{
                "type": "object",
                "description": "Authentication config",
                "properties": map[string]any{
                    "type": map[string]any{
                        "type": "string",
                        "enum": []string{"bearer", "basic", "apikey"},
                    },
                    "token": map[string]any{"type": "string"},
                    "username": map[string]any{"type": "string"},
                    "password": map[string]any{"type": "string"},
                    "key": map[string]any{"type": "string"},
                    "in": map[string]any{
                        "type": "string",
                        "enum": []string{"header", "query"},
                    },
                },
            },
            "timeout": map[string]any{
                "type": "number",
                "description": "Request timeout in seconds",
            },
            "save": map[string]any{
                "type": "string",
                "description": "Save request with this name",
            },
            "call": map[string]any{
                "type": "string",
                "description": "Call a saved request by name",
            },
            "env": map[string]any{
                "type": "string",
                "description": "Environment to use (dev/staging/prod)",
            },
            "history": map[string]any{
                "type": "boolean",
                "description": "Show request history",
            },
        },
        "required": []string{"url"},
    }
}

func (h *HTTP) Run(ctx context.Context, raw json.RawMessage) (string, error) {
    var args struct {
        Method  string          `json:"method"`
        URL     string          `json:"url"`
        Headers map[string]any  `json:"headers"`
        Query   map[string]any  `json:"query"`
        Body    any             `json:"body"`
        Auth    *Auth           `json:"auth"`
        Timeout int             `json:"timeout"`
        Save    string          `json:"save"`
        Call    string          `json:"call"`
        Env     string          `json:"env"`
        History bool            `json:"history"`
    }
    if err := json.Unmarshal(raw, &args); err != nil {
        return "", err
    }

    // 处理保存/调用请求
    // 处理环境变量
    // 执行请求
    // 格式化输出
}
```

#### 6. 请求历史

```go
// internal/http/history.go

type History struct {
    file *os.File
    mu   sync.Mutex
}

type HistoryEntry struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Method    string    `json:"method"`
    URL       string    `json:"url"`
    Status    int       `json:"status"`
    Duration  int64     `json:"duration_ms"`
    Error     string    `json:"error,omitempty"`
}

func (h *History) Add(entry HistoryEntry) error
func (h *History) List(limit int) ([]HistoryEntry, error)
func (h *History) Get(id string) (*HistoryEntry, error)
```

#### 7. 请求存储

```go
// internal/http/storage.go

type Storage struct {
    dir string
}

type SavedRequest struct {
    Name     string            `json:"name"`
    Method   string            `json:"method"`
    URL      string            `json:"url"`
    Headers  map[string]string `json:"headers,omitempty"`
    Query    map[string]string `json:"query,omitempty"`
    Body     any               `json:"body,omitempty"`
    Auth     *Auth             `json:"auth,omitempty"`
    Created  time.Time        `json:"created"`
}

func (s *Storage) Save(name string, req SavedRequest) error
func (s *Storage) Get(name string) (*SavedRequest, error)
func (s *Storage) List() ([]SavedRequest, error)
func (s *Storage) Delete(name string) error
```

#### 8. 环境变量

```go
// internal/http/env.go

type Env struct {
    dir string
}

type Environment struct {
    Name   string            `json:"name"`
    Vars   map[string]string `json:"vars"`
}

func (e *Env) Get(envName string) (*Environment, error)
func (e *Env) Interpolate(s string, env *Environment) string
```

### 配置管理

```toml
# ~/.config/otter/config.toml

[http]
timeout = 30
max_history = 100
save_dir = "~/.config/otter/http"

# 环境配置
[http.environments.dev]
base_url = "http://localhost:3000"
token = "$DEV_TOKEN"

[http.environments.staging]
base_url = "https://staging.example.com"
token = "$STAGING_TOKEN"

[http.environments.prod]
base_url = "https://api.example.com"
token = "$PROD_TOKEN"
```

## 接口设计

### 工具参数

```json
{
  "method": "POST",
  "url": "https://api.example.com/users",
  "headers": {
    "Content-Type": "application/json",
    "X-Custom-Header": "value"
  },
  "query": {
    "page": "1",
    "limit": "10"
  },
  "body": {
    "name": "John",
    "email": "john@example.com"
  },
  "auth": {
    "type": "bearer",
    "token": "$API_TOKEN"
  },
  "timeout": 30
}
```

### 保存请求

```json
{
  "save": "create-user",
  "method": "POST",
  "url": "{{base_url}}/users",
  "headers": {
    "Authorization": "Bearer {{token}}"
  },
  "body": {
    "name": "{{name}}",
    "email": "{{email
    "email": "{{email}}"
  }
}
```

### 调用保存的请求

```json
{
  "call": "create-user",
  "body": {
    "name": "Jane",
    "email": "jane@example.com"
  }
}
```

## 验收标准

### 功能验收

#### P0（必须实现）

- [ ] **HTTP 方法支持**
  - [ ] GET 请求
  - [ ] POST 请求
  - [ ] PUT 请求
  - [ ] DELETE 请求
  - [ ] PATCH 请求

- [ ] **请求构建**
  - [ ] 自定义 Headers
  - [ ] Query 参数
  - [ ] JSON 请求体
  - [ ] Form 请求体

- [ ] **响应处理**
  - [ ] 状态码显示
  - [ ] 响应头查看
  - [ ] JSON 格式化输出
  - [ ] 请求耗时统计

- [ ] **认证支持**
  - [ ] Bearer Token
  - [ ] Basic Auth
  - [ ] API Key (header/query)

#### P1（强烈建议）

- [ ] **请求管理**
  - [ ] 保存请求
  - [ ] 调用保存的请求
  - [ ] 请求历史记录
  - [ ] 环境变量支持

- [ ] **高级功能**
  - [ ] 超时控制
  - [ ] 重试机制
  - [ ] cURL 导入

- [ ] **输出增强**
  - [ ] XML 格式化
  - [ ] CSV 导出
  - [ ] 响应大小统计

#### P2（可选）

- [ ] OAuth 2.0 支持
- [ ] WebSocket 支持
- [ ] 请求模板
- [ ] Postman 导入/导出

### 代码质量验收

- [ ] 遵循 CLAUDE.md 代码风格
- [ ] 错误处理完善
- [ ] 单元测试覆盖

### 文档验收

- [ ] 工具描述清晰准确
- [ ] 参数文档完整
- [ ] 使用示例丰富

## 参考

### Go HTTP 库

- **net/http**: 标准库 - https://pkg.go.dev/net/http
- **resty**: HTTP 客户端库 - https://github.com/go-resty/resty
- **req**: 简洁的 HTTP 客户端 - https://github.com/imroc/req

### 相关项目

- **Postman**: API 客户端标杆
- **Insomnia**: API 客户端
- **HTTPie**: 命令行 HTTP 客户端 - https://httpie.io
- **curl**: 命令行工具

### 文档

- HTTP/1.1 规范: https://tools.ietf.org/html/rfc7230
- RESTful API 最佳实践: https://restfulapi.net/

---

**PRD 版本**: 1.0
**创建日期**: 2026-02-21
**预计工期**: 3-4 天
**优先级**: 高（使用频率高，实用性强）
