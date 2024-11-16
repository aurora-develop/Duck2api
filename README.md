# duck2api

# Web端 

访问http://你的服务器ip:8080/web

![web使用](https://fastly.jsdelivr.net/gh/xiaozhou26/tuph@main/images/%E5%B1%8F%E5%B9%95%E6%88%AA%E5%9B%BE%202024-04-07%20111706.png)

## Deploy


### 编译部署

```bash
git clone https://github.com/aurora-develop/duck2api
cd duck2api
go build -o duck2api
chmod +x ./duck2api
./duck2api
```

### Docker部署
## Docker部署
您需要安装Docker和Docker Compose。

```bash
docker run -d \
  --name duck2api \
  -p 8080:8080 \
  ghcr.io/aurora-develop/duck2api:latest
```

## Docker Compose部署
创建一个新的目录，例如duck2api，并进入该目录：
```bash
mkdir duck2api
cd duck2api
```
在此目录中下载库中的docker-compose.yml文件：

```bash
docker-compose up -d
```

## Usage

```bash
curl --location 'http://你的服务器ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--data '{
     "model": "gpt-4o-mini",
     "messages": [{"role": "user", "content": "Say this is a test!"}],
     "stream": true
   }'
```

## 支持的模型

- ~~gpt-3.5-turbo~~  duckduckGO官方已移除3.5模型的支持  
- claude-3-haiku
- llama-3.1-70b
- mixtral-8x7b
- gpt-4o-mini
## 高级设置

默认情况不需要设置，除非你有需求

### 环境变量
```

Authorization=your_authorization  用户认证 key。
TLS_CERT=path_to_your_tls_cert 存储TLS（传输层安全协议）证书的路径。
TLS_KEY=path_to_your_tls_key 存储TLS（传输层安全协议）证书的路径。
PROXY_URL=your_proxy_url 添加代理池来。
```

## 鸣谢

感谢各位大佬的pr支持，感谢。


## 参考项目


https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
