# WorkBuddy CLIProxyAPI Provider

[![Release](https://img.shields.io/github/v/release/libukai/workbuddy-cliproxy)](https://github.com/libukai/workbuddy-cliproxy/releases)
[![CI](https://github.com/libukai/workbuddy-cliproxy/actions/workflows/ci.yml/badge.svg)](https://github.com/libukai/workbuddy-cliproxy/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/libukai/workbuddy-cliproxy)](LICENSE)

把腾讯 CodeBuddy 的模型接入 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)，向本地客户端提供 OpenAI Chat Completions 和 Anthropic 兼容接口。

这是一个社区维护的外置 Provider 插件，不属于腾讯或 CLIProxyAPI 官方组件。项目 Fork 自 [lovingfish/workbuddy-cliproxy](https://github.com/lovingfish/workbuddy-cliproxy)，其 clean-room 实现来源于 [Sliverkiss/cpa-plugin](https://github.com/Sliverkiss/cpa-plugin) 公开的 `workbuddy.so`。

## 主要能力

- CodeBuddy 扫码登录、凭据持久化和到期前刷新。
- OpenAI / Anthropic 兼容调用，支持流式和非流式响应。
- 通过 CLIProxyAPI Host HTTP Bridge 复用宿主代理、请求上下文和传输记录。
- 多账号独立 Auth ID，以及同一 refresh token 的并发刷新去重。
- 思考内容、工具调用和指定函数 `tool_choice` 兼容处理。
- 嵌入式模型目录，以及可热重载的外部模型 manifest。
- macOS 和 Linux CI 构建；持续测试 CLIProxyAPI SDK `7.2.30`、`7.2.130` 和 `7.2.145`。

## 当前状态

当前维护版：[`v0.2.0`](https://github.com/libukai/workbuddy-cliproxy/releases/tag/v0.2.0)

已在 macOS arm64 上使用 CLIProxyAPI `7.2.130` 和 `7.2.145` 完成真实验证，包括：

- 插件加载和模型注册；
- 非流式与流式生成；
- 上游 HTTP 错误保真；
- 指定函数调用及 arguments 聚合；
- 配置和模型 manifest 热重载；
- 认证元数据与旧版 `workbuddy.json` 兼容。

Linux 构建会在 CI 中完成，但发布前仍应在目标架构上做真实账号验证。

## 支持的模型

| 系列 | 模型 ID |
|---|---|
| GLM | `glm-5.3`、`glm-5.3-flash`、`glm-5.2`、`glm-5.1`、`glm-5v-turbo` |
| Kimi | `kimi-k3`、`kimi-k2.7`、`kimi-k2.6` |
| MiniMax | `minimax-m3`、`minimax-m3-pay` |
| 混元 | `hy4-preview`、`hy3`、`hy3-preview`、`hy3-preview-agent` |
| DeepSeek | `deepseek-v4-pro`、`deepseek-v4-flash` |

模型是否真实可用，以 CodeBuddy 账号权限和一次真实请求为准。`GET /v1/models` 只证明模型已注册，不等于账号拥有调用权限。

默认目录来自 [`models.yaml`](models.yaml)。如需在不重新编译插件的情况下替换目录，可配置外部 `model_manifest`。

## 快速开始

### 1. 前置条件

- 带动态插件支持的 CLIProxyAPI v7.2.x；
- 可正常登录的 CodeBuddy 账号；
- 与 CLIProxyAPI 进程一致的操作系统和 CPU 架构；
- 从源码构建时需要 Go 1.26+ 和可用的 C 编译器。

### 2. 获取插件

macOS arm64 可以直接使用 [Release](https://github.com/libukai/workbuddy-cliproxy/releases) 中的 ZIP。下载后先根据 `checksums.txt` 验证 SHA-256，再将 `workbuddy.dylib` 放入 CLIProxyAPI 插件目录。

macOS 通过 launchd 加载插件时，可能需要对已校验的本地副本执行 ad-hoc 签名：

```bash
codesign --force --sign - --timestamp=none workbuddy.dylib
codesign --verify --strict --verbose=2 workbuddy.dylib
```

从源码构建：

```bash
git clone https://github.com/libukai/workbuddy-cliproxy.git
cd workbuddy-cliproxy

# macOS
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
  go build -trimpath -buildmode=c-shared -o workbuddy.dylib .

# Linux 示例
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -trimpath -buildmode=c-shared -o workbuddy.so .
```

动态库扩展名分别为：macOS `.dylib`、Linux `.so`、Windows `.dll`。

### 3. 启用插件

```yaml
plugins:
  enabled: true
  dir: "/absolute/path/to/plugins"
  configs:
    workbuddy:
      enabled: true
      priority: 100
      prompt_rewrite: false
      # model_manifest: "/absolute/path/to/workbuddy-models.yaml"
```

重启或重新加载 CLIProxyAPI 配置。日志出现以下内容，说明插件已注册：

```text
plugin loaded ... plugin_id=workbuddy
plugin registered ... plugin_name=workbuddy
```

### 4. 登录 CodeBuddy

打开 CLIProxyAPI 管理面板，添加 `workbuddy` 凭据并完成 CodeBuddy 扫码登录。

旧版 `workbuddy.json` 会继续按原 ID 加载。新登录账号使用 `workbuddy-<hash>.json`，文件名不会直接暴露 UID，并可为多个账号保留独立 Auth ID。

认证文件包含 access token 和 refresh token，应保持 `0600` 权限，不得提交到 Git 仓库或发送到日志、聊天和公开 Issue。

### 5. 验证

先确认模型已注册：

```bash
curl http://127.0.0.1:8317/v1/models \
  -H "Authorization: Bearer <api-key>"
```

再发送一次最小真实请求：

```bash
curl http://127.0.0.1:8317/v1/chat/completions \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.3-flash",
    "messages": [{"role": "user", "content": "只回复 OK"}],
    "stream": false
  }'
```

只有真实请求返回成功，才能说明插件、凭据、账号权限和上游模型共同可用。

## 客户端接入

CLIProxyAPI 默认端口为 `8317`，客户端 API Key 取自 CLIProxyAPI 配置中的 `api-keys`。

| 协议 | Base URL |
|---|---|
| OpenAI | `http://127.0.0.1:8317/v1` |
| Anthropic | `http://127.0.0.1:8317` |

Claude Code 示例：

```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:8317
export ANTHROPIC_API_KEY=<api-key>
export ANTHROPIC_MODEL=hy3-preview-agent
claude
```

## 实现说明

```text
OpenAI / Anthropic 客户端
            ↓
       CLIProxyAPI
  协议转换・路由・认证・代理
            ↓
     workbuddy 动态插件
  登录・模型・请求方言・流聚合
            ↓
    copilot.tencent.com
```

### 流式与非流式

CodeBuddy 上游拒绝原生非流式请求，因此插件统一向上游请求 SSE：

- 流式客户端通过 `host.http.do_stream` 和 `host.stream.emit` 实时接收 chunk；
- 非流式客户端由插件聚合 SSE 后返回单个 `chat.completion`；
- 上游 400、401、429 和 5xx 会尽量保留原 HTTP 语义，而不是统一变成 500。

### 工具调用

非流式响应会按 `tool_calls[].index` 合并拆分的调用 ID、函数名和 arguments。

CodeBuddy 的 `tool_choice` 只接受字符串。客户端使用 OpenAI 指定函数对象时，插件会仅保留被指定函数，并将选择方式转换为 `"required"`，以保留“必须调用该函数”的语义。

### 思考模式

`hy3` 系列会将 `reasoning_effort` 规范为 CodeBuddy 实际识别的 `high`。思考内容通过 `delta.reasoning_content` 返回，客户端需要支持对应字段才能显示。

### 配置热重载

插件实现 `plugin.register` 和 `plugin.reconfigure`：

- `prompt_rewrite`：是否启用旧版 Claude Code 模板改写，默认 `false`；
- `model_manifest`：外部 YAML/JSON 模型目录的绝对路径。

外部 manifest 只有在 CLIProxyAPI 发生配置重载时才会重新读取。无效、空目录或重复模型 ID 会被拒绝，并保留上一版有效配置。

## 风险与边界

- 本项目使用 CodeBuddy 当前客户端接口，它不是腾讯对外承诺稳定性的通用公开 API；接口、模型 ID 和账号策略可能随时变化。
- 模型出现在界面、源码或 `/v1/models` 中，都不能代替真实账号调用。
- 建议 CLIProxyAPI 仅监听 `127.0.0.1`；对局域网或公网开放前必须增加独立鉴权和网络边界。
- `prompt_rewrite` 默认关闭。该功能依赖具体 Claude Code 模板和 CodeBuddy 审核规则，也可能涉及上游使用条款；启用前应自行评估。
- 不要把测试环境、静态审查、编译成功或登录成功单独表述为“反代已经可用”。

## 开发与维护

```bash
gofmt -w *.go
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=1 go build -trimpath -buildmode=c-shared -o workbuddy.dylib .
```

长期维护时应分别检查：

1. 上游插件仓库和本 Fork 的提交差异；
2. CLIProxyAPI Release、Plugin ABI 和官方 Provider 范式；
3. CodeBuddy 模型目录和账号权限；
4. 插件加载、认证刷新、流式、工具调用及错误语义；
5. 每个新增或重命名模型的最小真实请求。

相关资料：

- [官方 Provider 差异表](docs/OFFICIAL_PROVIDER_GAP.md)
- [宿主兼容矩阵](docs/COMPATIBILITY.md)
- [维护路线图](docs/ROADMAP.md)

## License

[MIT](LICENSE)
