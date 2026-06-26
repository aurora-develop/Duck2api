# Duck2API

DuckDuckGo AI Chat 转 OpenAI 兼容 API 代理。支持 Chat Completions、图像生成/编辑、文件上传问答、语音转文字、文字转语音、推理模式、网络搜索等完整功能。
## 接口文档

curl 示例请查看：[API.md](API.md)

## 部署

### 编译部署

```bash
git clone https://github.com/aurora-develop/duck2api
cd duck2api
go build -o duck2api
chmod +x ./duck2api
./duck2api
```

### Docker 部署

```bash
docker run -d \
  --name duck2api \
  -p 8080:8080 \
  ghcr.io/aurora-develop/duck2api:latest
```

### Docker Compose 部署

```bash
mkdir duck2api && cd duck2api
wget https://raw.githubusercontent.com/aurora-develop/duck2api/main/docker-compose.yml
docker-compose up -d
```

## 功能概览

| 功能 | 端点 | 说明 |
|------|------|------|
| Chat Completions | `POST /v1/chat/completions` | 流式/非流式对话 |
| Responses API | `POST /v1/responses` | OpenAI Responses API |
| 图像生成 | `POST /v1/images/generations` | 文生图 |
| 图像编辑 | `POST /v1/images/edits` | 图生图/改图 |
| 文件上传 | `POST /v1/files` | 上传文件用于问答 |
| 文件管理 | `GET/DELETE /v1/files/:id` | 查询/删除文件 |
| 语音转文字 | `POST /v1/audio/transcriptions` | Whisper 兼容接口 |
| 文字转语音 | `POST /v1/audio/speech` | TTS 接口，支持 MP3/WAV/OGG |
| 模型列表 | `GET /v1/models` | 列出可用模型 |

## 快速开始

### 基础对话

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }'
```

### 推理模式 (Reasoning)

使用 `reasoning_effort` 参数控制推理深度：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-mini",
    "messages": [{"role": "user", "content": "证明勾股定理"}],
    "reasoning_effort": "high"
  }'
```

支持的值：`none`（快速）、`low`、`medium`、`high`

### 网络搜索

设置 `web_search: true` 启用联网搜索：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-nano",
    "messages": [{"role": "user", "content": "今天的科技新闻"}],
    "web_search": true
  }'
```

### 图像生成

```bash
curl http://localhost:8080/v1/images/generations \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "一只可爱的猫咪坐在窗台上",
    "model": "gpt-5.4-nano"
  }'
```

### 图像编辑

```bash
# JSON 方式 (base64)
curl http://localhost:8080/v1/images/edits \
  -H "Content-Type: application/json" \
  -d '{
    "image": "<base64编码的图片>",
    "prompt": "把猫改成蓝色"
  }'

# 文件上传方式
curl http://localhost:8080/v1/images/edits \
  -F "image=@cat.png" \
  -F "prompt=把猫改成蓝色"
```

### 文件上传与问答

```bash
# 1. 上传文件
curl http://localhost:8080/v1/files \
  -F "file=@document.pdf" \
  -F "purpose=assistants"

# 返回: {"id": "file-xxx", "object": "file", ...}

# 2. 使用文件进行问答（将文件内容作为上下文）
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-nano",
    "messages": [{"role": "user", "content": "请总结这个文档"}],
    "file_ids": ["file-xxx"]
  }'
```

### 语音转文字

```bash
curl http://localhost:8080/v1/audio/transcriptions \
  -F "file=@audio.webm" \
  -F "model=whisper-1"
```

支持格式：webm、ogg、mp3、wav、m4a、flac、opus、aac

### 文字转语音

```bash
curl http://localhost:8080/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"tts-1","input":"你好世界","voice":"alloy","response_format":"mp3"}' \
  --output speech.mp3
```

支持格式：mp3、wav、ogg、flac、aac（底层使用 Duck.ai 的 WebRTC + OpenAI Realtime API）

## 支持的模型

| 模型 | 类型 | 说明 |
|------|------|------|
| `gpt-5.4-nano` | 推理 | OpenAI 最新轻量推理模型 |
| `gpt-5.4-mini` | 推理 | OpenAI 推理模型 |
| `gpt-5.2-thinking` | 推理 | 深度思考模型 |
| `gpt-5.1-thinking` | 推理 | 深度思考模型 |
| `gpt-5-mini` | 通用 | OpenAI 通用模型 |
| `gpt-4o-mini` | 通用 | OpenAI 通用模型 |
| `gpt-3.5-turbo-0125` | 通用 | 映射到 gpt-4o-mini |
| `claude-3-haiku-20240307` | 通用 | Anthropic Claude |
| `claude-haiku-4-5` | 通用 | Anthropic Claude |
| `claude-opus-4-6-thinking` | 推理 | Claude 推理模型 |
| `llama-3.3-70b` | 通用 | Meta Llama |
| `llama-4-scout` | 通用 | Meta Llama 4 |
| `mistral-small` | 通用 | Mistral AI |

## 高级设置

### 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `Authorization` | API 认证 Key | `Bearer your_key` |
| `PROXY_URL` | 代理地址 | `http://proxy:8080` |
| `PREFIX` | URL 前缀 | `/api` |
| `TLS_CERT` | TLS 证书路径 | `/path/to/cert.pem` |
| `TLS_KEY` | TLS 密钥路径 | `/path/to/key.pem` |

### 代理池

支持 `proxies.txt` 文件配置多个代理（每行一个）：

```
http://proxy1:8080
http://proxy2:8080
socks5://proxy3:1080
```

## 鸣谢

感谢各位大佬的 PR 支持。

## 参考项目

- https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
