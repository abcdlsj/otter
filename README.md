# Otter

终端 AI Agent，支持多模型、文件操作和命令执行。

## 功能

- 交互式 TUI 界面
- 多 LLM Provider 支持（Anthropic、OpenAI、Kimi 等）
- 文件读写操作
- Shell 命令执行
- 会话历史保存
- 代码搜索（grep）
- 多模式 Agent（build/plan/explore）

## 安装

```bash
git clone https://github.com/abcdlsj/otter
cd otter
go build -o otter
```

## 配置

创建 `~/.config/otter/config.toml`：

```toml
stream = true
max_steps = 100

[[providers]]
name = "anthropic"
base_url = "https://api.anthropic.com"
api_key = "sk-your-api-key"
default = true

[[providers.models]]
name = "claude-sonnet-4-20250514"
default = true
```

## 使用

```bash
./otter
```

### Agent 模式

Otter 内置多种优化后的 Agent 模式，通过系统 prompt 自动调整行为：

- **build** (默认): 全功能编码助手，支持文件修改
- **plan**: 只读模式，用于探索和分析代码库
- **explore**: 快速搜索和定位代码

## 快捷键

| 按键 | 功能 |
|------|------|
| `Enter` | 发送消息 |
| `Tab` | 切换输入/历史模式 |
| `Ctrl+C` | 退出 |

## License

MIT
