# Duck2API 接口文档

Duck2API 提供与 OpenAI API 完全兼容的接口，底层使用 DuckDuckGo AI Chat 服务。

## 基础信息

- Base URL: `http://localhost:8080`
- 认证方式: `Authorization: Bearer <key>`（如设置了 `Authorization` 环境变量）
- 响应格式: `application/json`

---

## 目录

1. [Chat Completions](#1-chat-completions)
2. [Responses API](#2-responses-api)
3. [图像生成](#3-图像生成)
4. [图像编辑](#4-图像编辑)
5. [文件管理](#5-文件管理)
6. [语音转文字](#6-语音转文字)
7. [模型列表](#7-模型列表)
8. [健康检查](#8-健康检查)

---

## 1. Chat Completions

### `POST /v1/chat/completions`

与 DuckDuckGo AI 进行对话，支持推理模式和网络搜索。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | 是 | 模型名称，见 [支持的模型](#支持的模型) |
| `messages` | array | 是 | 消息数组 |
| `stream` | boolean | 否 | 是否流式输出，默认 `false` |
| `reasoning_effort` | string | 否 | 推理深度：`none`/`low`/`medium`/`high` |
| `web_search` | boolean | 否 | 启用网络搜索，默认 `false` |
| `file_ids` | string[] | 否 | 文件 ID 列表（使用已上传的文件作为上下文） |

### Messages 结构

```json
{
  "role": "user",
  "content": "你好"
}
```

支持多模态内容（图片）：

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述这张图片"},
    {"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}
  ]
}
```

### 请求示例

**基础对话:**
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

**推理模式:**
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-mini",
    "messages": [{"role": "user", "content": "证明勾股定理"}],
    "reasoning_effort": "high"
  }'
```

**联网搜索:**
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-nano",
    "messages": [{"role": "user", "content": "今天有什么新闻"}],
    "web_search": true
  }'
```

**带文件上下文:**
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-nano",
    "messages": [{"role": "user", "content": "总结这个文件"}],
    "file_ids": ["file-1234567890"]
  }'
```

### 响应示例

**非流式:**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 0,
  "model": "gpt-4o-mini",
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  },
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！有什么可以帮你的？"
      },
      "finish_reason": null
    }
  ]
}
```

**流式:**
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"好"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

---

## 2. Responses API

### `POST /v1/responses`

OpenAI Responses API 兼容接口。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | 是 | 模型名称 |
| `input` | string/array | 是 | 输入内容 |
| `instructions` | string | 否 | 系统指令 |
| `stream` | boolean | 否 | 流式输出 |

### 请求示例

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5-mini",
    "input": "你好",
    "instructions": "你是一个有帮助的助手"
  }'
```

---

## 3. 图像生成

### `POST /v1/images/generations`

文本生成图像，返回 base64 编码的图片数据。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `prompt` | string | 是 | 图像描述 |
| `model` | string | 否 | 模型，默认 `gpt-5.4-nano` |
| `n` | integer | 否 | 生成数量，默认 `1` |
| `response_format` | string | 否 | `b64_json`（默认） |
| `reasoning_effort` | string | 否 | 推理深度：`none`/`low`/`medium`/`high`，默认 `none` |

### 请求示例

```bash
curl http://localhost:8080/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "一只可爱的猫咪坐在窗台上，阳光照射",
    "model": "gpt-5.4-mini",
    "reasoning_effort": "low"
  }'
```

### 响应示例

```json
{
  "created": 1719398400,
  "data": [
    {
      "b64_json": "iVBORw0KGgo...",
      "revised_prompt": "..."
    }
  ]
}
```

---

## 4. 图像编辑

### `POST /v1/images/edits`

基于已有图像进行编辑（图生图/改图）。

### 请求方式

**JSON 方式:**
```bash
curl http://localhost:8080/v1/images/edits \
  -H "Content-Type: application/json" \
  -d '{
    "image": "<base64编码图片>",
    "prompt": "把背景改成海滩",
    "model": "gpt-5.4-nano"
  }'
```

**文件上传方式:**
```bash
curl http://localhost:8080/v1/images/edits \
  -F "image=@photo.png" \
  -F "prompt=把背景改成海滩" \
  -F "model=gpt-5.4-nano"
```

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `image` | string/file | 是 | base64 图片或文件上传 |
| `prompt` | string | 是 | 编辑指令 |
| `model` | string | 否 | 模型，默认 `gpt-5.4-nano` |
| `reasoning_effort` | string | 否 | 推理深度：`none`/`low`/`medium`/`high` |

---

## 5. 文件管理

### 上传文件

#### `POST /v1/files`

```bash
curl http://localhost:8080/v1/files \
  -F "file=@document.pdf" \
  -F "purpose=assistants"
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | 上传的文件 |
| `purpose` | string | 否 | 用途，默认 `assistants` |

**响应:**
```json
{
  "id": "file-1719398400000000000",
  "object": "file",
  "bytes": 12345,
  "created_at": 1719398400,
  "filename": "document.pdf",
  "purpose": "assistants"
}
```

### 列出文件

#### `GET /v1/files`

```bash
curl http://localhost:8080/v1/files
```

**响应:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "file-xxx",
      "object": "file",
      "bytes": 12345,
      "created_at": 1719398400,
      "filename": "document.pdf",
      "purpose": "assistants"
    }
  ]
}
```

### 获取文件信息

#### `GET /v1/files/:file_id`

```bash
curl http://localhost:8080/v1/files/file-xxx
```

### 删除文件

#### `DELETE /v1/files/:file_id`

```bash
curl -X DELETE http://localhost:8080/v1/files/file-xxx
```

**响应:**
```json
{
  "id": "file-xxx",
  "object": "file",
  "deleted": true
}
```

### 下载文件内容

#### `GET /v1/files/:file_id/content`

```bash
curl http://localhost:8080/v1/files/file-xxx/content -o output.pdf
```

### 文件问答流程

1. 上传文件 → 获取 `file_id`
2. 在 Chat Completions 请求中传入 `file_ids`
3. 文件内容会自动作为上下文注入到对话中

---

## 6. 语音转文字

### `POST /v1/audio/transcriptions`

Whisper 兼容的语音转文字接口，底层调用 Duck.ai 的听写服务。

### 请求方式

```bash
curl http://localhost:8080/v1/audio/transcriptions \
  -F "file=@audio.webm" \
  -F "model=whisper-1"
```

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | 音频文件 |
| `model` | string | 否 | 模型（兼容参数，实际使用 Duck.ai 听写） |

### 支持的音频格式

| 格式 | MIME Type |
|------|-----------|
| WebM | `audio/webm` |
| OGG | `audio/ogg` |
| MP3 | `audio/mpeg` |
| WAV | `audio/wav` |
| M4A | `audio/mp4` |
| FLAC | `audio/flac` |
| OPUS | `audio/opus` |
| AAC | `audio/aac` |

### 响应示例

```json
{
  "text": "这是一段语音转文字的结果"
}
```

---

## 7. 模型列表

### `GET /v1/models`

```bash
curl http://localhost:8080/v1/models
```

### 响应示例

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-5.4-nano",
      "object": "model",
      "created": 1685474247,
      "owned_by": "duckduckgo"
    }
  ]
}
```

---

## 8. 健康检查

### `GET /`

```bash
curl http://localhost:8080/
```

响应: `{"message": "Hello, world!"}`

### `GET /ping`

```bash
curl http://localhost:8080/ping
```

响应: `{"message": "pong"}`

---

## 支持的模型

### 通用模型

| 模型 ID | 说明 |
|---------|------|
| `gpt-5-mini` | OpenAI GPT-5 Mini |
| `gpt-4o-mini` | OpenAI GPT-4o Mini |
| `gpt-3.5-turbo-0125` | 映射到 gpt-4o-mini |
| `claude-3-haiku-20240307` | Anthropic Claude 3 Haiku |
| `claude-haiku-4-5` | Anthropic Claude Haiku 4.5 |
| `llama-3.3-70b` | Meta Llama 3.3 70B |
| `llama-4-scout` | Meta Llama 4 Scout |
| `mistral-small` | Mistral Small |
| `meta-llama/Llama-4-Scout-17B-16E-Instruct` | Llama 4 (原始 ID) |
| `mistralai/Mistral-Small-24B-Instruct-2501` | Mistral Small (原始 ID) |

### 推理模型

| 模型 ID | 说明 |
|---------|------|
| `gpt-5.4-nano` | OpenAI GPT-5.4 Nano（推理） |
| `gpt-5.4-mini` | OpenAI GPT-5.4 Mini（推理） |
| `gpt-5.2-thinking` | GPT-5.2 深度思考 |
| `gpt-5.1-thinking` | GPT-5.1 深度思考 |
| `claude-opus-4-6-thinking` | Claude Opus 推理模型 |

---

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `Authorization` | API 认证 Key | 无（不设置则无需认证） |
| `PROXY_URL` | 代理地址 | 无 |
| `PREFIX` | URL 前缀 | 无 |
| `TLS_CERT` | TLS 证书路径 | 无 |
| `TLS_KEY` | TLS 密钥路径 | 无 |

### 代理配置

支持三种代理方式（按优先级）：

1. `PROXY_URL` 环境变量
2. `proxies.txt` 文件（每行一个代理地址，轮询使用）
3. `http_proxy` 环境变量

`proxies.txt` 示例：
```
http://proxy1.example.com:8080
http://proxy2.example.com:8080
socks5://proxy3.example.com:1080
```

---

## 错误码

| HTTP 状态码 | 说明 |
|-------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 认证失败 |
| 404 | 资源不存在 |
| 418 | Duck.ai 请求限制（自动重试） |
| 429 | 请求频率限制（自动重试） |
| 500 | 服务器内部错误 |

---

## 技术细节

### 图像生成原理

- 使用 Duck.ai 的 `/duckchat/v1/chat` 端点
- 通过 `metadata.toolChoice.GenerateImage: true` 启用图像生成
- SSE 响应中包含 `parts` 数组，`type: "generated-image"` 的 part 包含 base64 图片数据
- 图片格式为 PNG，以 `data:image/png;base64,...` 形式返回

### 推理模式原理

- Duck.ai 的推理模型（如 gpt-5.4-mini）支持 `reasoningEffort` 参数
- 可选值：`none`（快速模式）、`low`、`medium`、`high`
- 高推理深度会生成更详细的思考过程

### 语音转文字原理

- 调用 Duck.ai 的 `/duckchat/v1/dictation` 端点
- 音频以原始二进制发送，`Content-Type` 为音频格式
- 使用相同的 VQD 认证机制
