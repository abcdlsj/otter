# Otter

终端 AI Agent，支持多模型、文件操作和命令执行。

## 功能

- 交互式 TUI 界面
- 多 LLM Provider 支持（Anthropic、OpenAI、Kimi 等）
- 文件读写操作
- Shell 命令执行
- 会话历史保存

## 安装

```bash
git clone https://github.com/abcdlsj/otter
cd otter
go build -o otter
```

## 配置

创建 `~/.agent/config.toml`：

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

## 快捷键

| 按键 | 功能 |
|------|------|
| `Enter` | 发送消息 |
| `Tab` | 切换输入/历史模式 |
| `Ctrl+C` | 退出 |

## License

MIT
