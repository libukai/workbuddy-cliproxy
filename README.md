# workbuddy-cliproxy

这是 [lovingfish/workbuddy-cliproxy](https://github.com/lovingfish/workbuddy-cliproxy) 的持续维护 Fork。当前维护目标是跟进 CodeBuddy 模型变化、CLIProxyAPI 插件 ABI 与认证生命周期，并以真实请求验证反代可用性。

把**腾讯 CodeBuddy**（`copilot.tencent.com`）封装成 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)(CPA)插件,任何支持 OpenAI / Anthropic 协议的客户端(Claude Code、Cursor、Cline、SDK……)都能直接调用 CodeBuddy 背后的模型。

对 [Sliverkiss/cpa-plugin](https://github.com/Sliverkiss/cpa-plugin) 公开 `workbuddy.so` 的 clean-room 逆向重写,补齐了源码与 x86_64 支持;workbuddy 的原始设计归属 Sliverkiss。

## 工作原理

在 CPA 里注册为 `workbuddy` provider:负责 CodeBuddy 扫码登录、token 刷新,并把请求转发到 `copilot.tencent.com/v2/chat/completions`。登录后凭据存为 `workbuddy.json`。

## 模型

`glm-5.3` · `glm-5.3-flash` · `glm-5.2` · `glm-5.1` · `glm-5v-turbo` · `kimi-k3` · `kimi-k2.7` · `kimi-k2.6` · `minimax-m3` · `minimax-m3-pay` · `hy4-preview` · `hy3` · `hy3-preview` · `hy3-preview-agent` · `deepseek-v4-pro` · `deepseek-v4-flash`

具体可用性以 CodeBuddy 账号权限为准。

## 安装

**前置**:运行中的 CLIProxyAPI v7.2.x(带 CGO / 插件支持)、CodeBuddy 账号、Go 1.26+ 与 gcc;编译架构需与 CPA 实例一致(amd64 / arm64)。

```bash
git clone https://github.com/libukai/workbuddy-cliproxy.git
cd workbuddy-cliproxy
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -buildmode=c-shared -o workbuddy.so .
```

产物:`.so`(Linux)/ `.dylib`(macOS)/ `.dll`(Windows)。放到 CPA 的 `plugins/` 目录,在 `config.yaml` 启用:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    workbuddy: { enabled: true, priority: 100 }
```

重启 CPA,日志出现 `plugin loaded ... plugin_id=workbuddy` 即成功,`GET /v1/models` 也能看到上面的模型。然后到 CPA 面板添加 workbuddy 凭据,扫码登录 CodeBuddy。

## 使用

CPA 默认端口 `8317`,API key 见 `config.yaml` 的 `api-keys`。

| 协议 | Base URL |
|------|----------|
| OpenAI | `http://<host>:8317/v1` |
| Anthropic | `http://<host>:8317`(不带 `/v1`,走 `x-api-key`) |

```bash
# Claude Code
export ANTHROPIC_BASE_URL=http://localhost:8317
export ANTHROPIC_API_KEY=<api-key>
export ANTHROPIC_MODEL=hy3-preview-agent
claude
```

```bash
# curl / OpenAI
curl http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer <api-key>" -H "Content-Type: application/json" \
  -d '{"model":"hy3-preview-agent","messages":[{"role":"user","content":"你好"}],"stream":true}'
```

流式 / 非流式都支持;非流式请求会被内部转成流式再聚合(CodeBuddy 上游 `code 11101` 拒绝非流式)。

## Claude Code 兼容性

本 Fork 继承了上游针对两句旧版 Claude Code system 模板的精确字符串改写:

- `You are Claude Code, Anthropic's official CLI for Claude.`(身份句)
- `Main branch (you will usually use this for PRs)`(git 注入句)

workbuddy 转发前会对匹配到的旧模板做最小改写(`CLI`→`CLI tool`、`Main branch`→`Default branch`)。该逻辑依赖具体客户端版本和上游审核规则，不能作为稳定兼容保证，也可能涉及 CodeBuddy 的使用条款边界。

后续版本将把该行为改为显式配置项并默认关闭；在此之前，生产使用方应自行评估是否启用 Claude Code 兼容路径。

## 思考模式

hy3 系列(`hy3` / `hy3-preview` / `hy3-preview-agent`)自动开最大思考:workbuddy 转发前强制 `reasoning_effort=high`,覆盖客户端任何设置。CodeBuddy 只对 `high` 真正开深度思考(`medium` / `max` / `xhigh` 等档位它直接忽略),所以这已是 hy3 能用的最高档。思考内容走 SSE 的 `delta.reasoning_content`,客户端要支持渲染思考块才看得到。

## 流式

真流式(async):转发上游时边读边通过 `host.stream.emit` 把每个 chunk 实时推给 CPA,客户端逐字收到(不是等收齐了一股脑)。hy3 几千字的思考过程也是实时流出的,不是憋半天再刷出来。

## 开发与验证

```bash
gofmt -w main.go main_test.go
go test ./...
go vet ./...
CGO_ENABLED=1 go build -buildmode=c-shared -o workbuddy.dylib .
```

发布或更新模型时不能只检查 `/v1/models`;至少要用目标账号对每个新增模型发出一次最小真实请求。当前维护版会把凭据过期时间同步给 CLIProxyAPI 的刷新调度，并在插件卸载时取消正在运行的异步流。刷新与模型执行通过 `host.http.do` / `host.http.do_stream` 复用 CLIProxyAPI 的宿主传输、代理和请求记录；扫码登录仍使用独立 Cookie Jar 保持登录状态隔离。

架构优化以 CLIProxyAPI 官方 Kimi Provider 为主要参考，具体差异和迁移边界见 [`docs/OFFICIAL_PROVIDER_GAP.md`](docs/OFFICIAL_PROVIDER_GAP.md)，宿主版本验证见 [`docs/COMPATIBILITY.md`](docs/COMPATIBILITY.md)。外置插件只依赖公开 Plugin ABI，不导入 CLIProxyAPI 的 `internal/*` 包。

## License

MIT。
