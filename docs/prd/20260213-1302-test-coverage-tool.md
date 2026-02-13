# Feature: Test & Coverage Tool

## 背景与动机

### 当前问题

开发者在日常工作中需要频繁运行测试和检查代码覆盖率。虽然 Otter 已经有 `shell` 工具可以执行命令，但存在以下痛点：

1. **多语言/多框架适配复杂**：不同语言（Go, Python, JavaScript, Rust, Java）的测试命令和覆盖率工具各不相同，需要开发者记忆大量命令
2. **输出不够友好**：原始测试输出格式不统一，难以快速理解测试结果和覆盖率信息
3. **需要手动解析**：测试失败时需要手动查找错误位置，影响效率
4. **缺少统一入口**：没有统一的工具来管理测试相关操作

### 用户价值

- **提升效率**：一键运行测试，自动检测项目配置
- **降低心智负担**：无需记忆不同语言的测试命令
- **快速定位问题**：格式化错误输出，突出关键信息
- **持续关注质量**：便捷的覆盖率检查，鼓励编写测试

### 与现有生态的关系

`test` 工具不会替代 `shell` 工具，而是作为专门针对测试场景的高级封装，提供：
- 更智能的项目检测
- 更友好的输出格式
- 更丰富的参数选项

## 功能描述

### 核心功能

1. **自动项目检测**
   - 根据项目文件自动识别语言和框架
   - Go（go test）、Python（pytest, unittest）、JavaScript/TypeScript（jest, vitest, mocha）、Rust（cargo test）等

2. **测试运行**
   - 运行全部测试或指定测试文件/模式
   - 支持并行运行（根据语言特性）
   - 支持详细输出模式

3. **覆盖率分析**
   - 生成覆盖率报告（如果语言支持）
   - 显示整体覆盖率百分比
   - 显示未覆盖的文件/代码行（可选）

4. **智能输出**
   - 统一格式的测试摘要（通过/失败/跳过/总用时）
   - 高亮显示失败的测试用例
   - 显示失败位置（文件:行号）
   - 压缩通过的测试输出，突出重点

5. **灵活的参数控制**
   - 指定项目路径
   - 指定测试文件或模式
   - 设置超时时间
   - 控制输出详细程度

### 支持的语言和框架（MVP）

优先支持以下语言（根据工具使用频率和项目相关性）：

| 语言 | 测试工具 | 覆盖率工具 |
|------|---------|-----------|
| Go | go test | go test -cover |
| Python | pytest/unittest | pytest-cov/coverage.py |
| JavaScript/TypeScript | jest/vitest | --coverage |
| Rust | cargo test | tarpaulin/cargo-tarpaulin |

未来可扩展：Java (JUnit/JaCoCo), C++ (Google Test), Ruby (RSpec) 等

## 技术方案

### 文件结构

```
internal/tool/
  ├── test.go          # 新增：Test 工具实现
  └── (现有文件保持不变)
```

### 核心实现

#### 1. Test 工具接口

```go
type Test struct{}

func (Test) Name() string { return "test" }
func (Test) Desc() string {
    return "Run tests and generate coverage reports. Auto-detects project language and framework."
}
```

#### 2. 项目检测器

```go
type ProjectType string

const (
    TypeGo   ProjectType = "go"
    TypePy   ProjectType = "python"
    TypeJS   ProjectType = "javascript"
    TypeRust ProjectType = "rust"
)

type Detector struct {
    path string
}

func (d *Detector) Detect() ProjectType {
    // 检查 go.mod -> Go
    // 检查 pyproject.toml, setup.py, pytest.ini -> Python
    // 检查 package.json, tsconfig.json -> JavaScript/TypeScript
    // 检查 Cargo.toml -> Rust
}
```

#### 3. 测试运行器接口

```go
type Runner interface {
    RunTests(ctx context.Context, opts RunOptions) (TestResult, error)
    RunCoverage(ctx context.Context, opts RunOptions) (CoverageResult, error)
}

type RunOptions struct {
    Path     string   // 项目路径
    Pattern  string   // 测试文件/函数模式
    Verbose  bool     // 详细输出
    Timeout  int      // 超时时间（秒）
}

type TestResult struct {
    Passed   int
    Failed   int
    Skipped  int
    Duration time.Duration
    Failures []TestCase
    Output   string
}

type CoverageResult struct {
    Percent  float64
    Files    []FileCoverage
    Output   string
}
```

#### 4. 具体语言实现

```go
// internal/tool/test_go.go
type GoRunner struct{}

func (g GoRunner) RunTests(ctx context.Context, opts RunOptions) (TestResult, error) {
    // go test -v ./...
    // 解析输出，提取测试结果
}

func (g GoRunner) RunCoverage(ctx context.Context, opts RunOptions) (CoverageResult, error) {
    // go test -cover ./...
    // 解析覆盖率输出
}

// internal/tool/test_python.go
type PythonRunner struct {
    // 类似实现
}

// internal/tool/test_js.go
type JSRunner struct {
    // 类似实现
}

// internal/tool/test_rust.go
type RustRunner struct {
    // 类似实现
}
```

#### 5. 输出格式化

```go
func FormatTestResult(result TestResult, verbose bool) string {
    var sb strings.Builder

    // 摘要行
    fmt.Fprintf(&sb, "✓ Passed: %d | ✗ Failed: %d | ○ Skipped: %d\n",
        result.Passed, result.Failed, result.Skipped)
    fmt.Fprintf(&sb, "Duration: %v\n", result.Duration)

    // 失败详情
    if len(result.Failures) > 0 {
        sb.WriteString("\nFailed Tests:\n")
        for _, tc := range result.Failures {
            fmt.Fprintf(&sb, "  ✗ %s (%s:%d)\n", tc.Name, tc.File, tc.Line)
            fmt.Fprintf(&sb, "    %s\n", tc.Error)
        }
    }

    // 详细输出（如果需要）
    if verbose {
        sb.WriteString("\n" + result.Output)
    }

    return sb.String()
}

func FormatCoverageResult(result CoverageResult) string {
    var sb strings.Builder

    // 总体覆盖率
    symbol := "✓"
    if result.Percent < 50 {
        symbol = "✗"
    } else if result.Percent < 80 {
        symbol = "○"
    }

    fmt.Fprintf(&sb, "%s Total Coverage: %.1f%%\n", symbol, result.Percent)

    // 文件级别覆盖率
    if len(result.Files) > 0 {
        sb.WriteString("\nFile Coverage:\n")
        for _, fc := range result.Files {
            symbol = "✓"
            if fc.Percent < 50 {
                symbol = "✗"
            } else if fc.Percent < 80 {
                symbol = "○"
            }
            fmt.Fprintf(&sb, "  %s %s: %.1f%%\n", symbol, fc.Path, fc.Percent)
        }
    }

    return sb.String()
}
```

### 输出解析策略

不同语言的测试输出格式各异，需要解析为统一的内部结构：

#### Go
```
--- FAIL: TestAdd (0.00s)
    test_test.go:10: expected 5, got 3
```

#### Python (pytest)
```
FAILED test_add.py::test_add - assert 3 == 5
```

#### JavaScript (jest)
```
● test_add › should add correctly
  Expected: 5
  Received: 3
```

解析逻辑：
1. 使用正则表达式匹配失败模式
2. 提取测试名称、文件、行号、错误信息
3. 填充到 `TestCase` 结构体

### 错误处理

1. **项目类型无法检测**
   - 返回友好的错误信息，提示检查项目配置
   - 建议使用 `shell` 工具手动运行测试

2. **测试命令执行失败**
   - 区分"测试失败"和"命令错误"
   - 测试失败：返回测试结果，标记失败
   - 命令错误：返回详细错误信息

3. **超时处理**
   - 使用 `context.WithTimeout` 限制执行时间
   - 超时后终止测试进程
   - 返回超时提示

### 性能考虑

1. **并行运行**：对于 Go 和 Rust，利用语言内置的并行测试能力
2. **输出截断**：限制输出长度，避免大量日志影响 UI
3. **缓存检测结果**：同一会话内缓存项目类型，避免重复检测

## 接口设计

### 命令行参数

```go
func (Test) Args() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "path": map[string]any{
                "type":        "string",
                "description": "Project path (default: current directory)",
            },
            "pattern": map[string]any{
                "type":        "string",
                "description": "Test file or function pattern (e.g., 'test_add', 'test_add.go')",
            },
            "coverage": map[string]any{
                "type":        "boolean",
                "description": "Generate coverage report (default: false)",
            },
            "verbose": map[string]any{
                "type":        "boolean",
                "description": "Show detailed output (default: false)",
            },
            "timeout": map[string]any{
                "type":        "number",
                "description": "Timeout in seconds (default: 60, max: 300)",
            },
        },
        "required": []string{},
    }
}
```

### 使用示例

#### 示例 1：运行所有测试
```json
{
  "action": "run"
}
```

输出：
```
Running tests (Go)...
✓ Passed: 12 | ✗ Failed: 1 | ○ Skipped: 0
Duration: 1.23s

Failed Tests:
  ✗ TestAdd (add_test.go:10)
    expected 5, got 3
```

#### 示例 2：运行特定测试
```json
{
  "pattern": "TestAdd"
}
```

#### 示例 3：生成覆盖率报告
```json
{
  "coverage": true
}
```

输出：
```
Running tests with coverage...
✓ Passed: 12 | ✗ Failed: 0 | ○ Skipped: 0
Duration: 1.45s

✓ Total Coverage: 85.3%

File Coverage:
  ✓ add.go: 100.0%
  ✓ subtract.go: 100.0%
  ○ divide.go: 66.7%
  ✗ multiply.go: 0.0%
```

#### 示例 4：指定路径和超时
```json
{
  "path": "./pkg/utils",
  "timeout": 120
}
```

## 验收标准

### 功能验收

- [ ] **基本功能**
  - [x] 自动检测 Go 项目并运行测试
  - [x] 自动检测 Python 项目并运行测试（pytest/unittest）
  - [x] 自动检测 JavaScript/TypeScript 项目并运行测试（jest/vitest）
  - [x] 自动检测 Rust 项目并运行测试（cargo test）

- [ ] **覆盖率支持**
  - [x] Go 项目生成覆盖率报告
  - [x] Python 项目生成覆盖率报告（pytest-cov/coverage.py）
  - [x] JavaScript/TypeScript 项目生成覆盖率报告
  - [x] Rust 项目生成覆盖率报告（cargo-tarpaulin，可选）

- [ ] **输出格式化**
  - [x] 显示测试摘要（通过/失败/跳过/用时）
  - [x] 高亮显示失败的测试用例
  - [x] 显示失败位置（文件:行号）
  - [x] 显示覆盖率百分比
  - [x] 按文件显示覆盖率（可选）

- [ ] **参数控制**
  - [x] 支持指定项目路径
  - [x] 支持指定测试文件/函数模式
  - [x] 支持覆盖率开关
  - [x] 支持详细输出开关
  - [x] 支持超时设置

### 代码质量验收

- [ ] 遵循 CLAUDE.md 定义的代码风格
  - [x] 简洁的命名
  - [x] 小函数（一屏可读）
  - [x] 错误立即处理
  - [x] 优雅的错误处理

- [ ] 性能要求
  - [x] 项目检测时间 < 100ms
  - [x] 输出解析时间合理
  - [x] 超时机制正常工作

- [ ] 安全性
  - [x] 遵守 config.toml 中的权限设置
  - [x] 不执行未授权的测试命令
  - [x] 超时限制生效

### 文档验收

- [ ] [ ] 工具描述清晰准确
- [ ] [ ] 参数文档完整
- [ ] [ ] 使用示例丰富

## 参考

### 相关项目

- **Go Test**：https://pkg.go.dev/cmd/go#hdr-Testing_flags
- **pytest**：https://docs.pytest.org/
- **Jest**：https://jestjs.io/
- **Cargo Test**：https://doc.rust-lang.org/cargo/commands/cargo-test.html

### 覆盖率工具

- **go test -cover**：https://pkg.go.dev/cmd/go#hdr-Testing_flags
- **pytest-cov**：https://pytest-cov.readthedocs.io/
- **Jest Coverage**：https://jestjs.io/docs/configuration#collectcoverage-boolean
- **cargo-tarpaulin**：https://github.com/xd009642/tarpaulin

### 设计参考

- **GitHub Actions**：测试报告格式化方式
- **Codecov**：覆盖率报告展示方式
- **SonarQube**：测试结果聚合方式

### 内部参考

- Otter `tool/tool.go`：工具接口定义
- Otter `tool/git.go`：命令执行和输出解析示例
- Otter `tool/shell.go`：超时处理和错误处理示例

---

**PRD 版本**: 1.0
**创建日期**: 2026-02-13
**预计工期**: 2-3 天
**优先级**: 中（增强开发体验）
