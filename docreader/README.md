# DocReader Service

DocReader 是 Yuheng 项目中负责文档解析和处理的 gRPC 服务。它支持多种文档格式的读取、OCR 识别、多模态处理等功能。

## Docker Compose 环境变量配置

在 `docker-compose.yml` 文件中，docreader 服务配置了以下环境变量：

```yaml
docreader:
  image: magicyuan876/yuheng-docreader:${YUHENG_VERSION:-latest}
  environment:
    - MINERU_ENDPOINT=${MINERU_ENDPOINT:-}
    - MAX_FILE_SIZE_MB=${MAX_FILE_SIZE_MB:-}
```

### 环境变量说明

#### 1. MINERU_ENDPOINT

- **说明**: MinerU 服务的访问地址（可选）
- **默认值**: 空（不使用 MinerU）
- **用途**: MinerU 是一个高级文档解析服务，支持更复杂的文档结构识别和处理。配置此变量后，DocReader 可以调用 MinerU 进行文档解析
- **配置示例**:
  ```bash
  # .env 文件
  MINERU_ENDPOINT=http://mineru-service:8080
  ```

#### 2. MAX_FILE_SIZE_MB

- **说明**: 允许上传的最大文件大小（单位：MB）
- **默认值**: `50` MB
- **用途**: 限制 gRPC 服务接收的文件大小，防止过大的文件导致服务崩溃或性能问题
- **配置示例**:
  ```bash
  # .env 文件
  MAX_FILE_SIZE_MB=100  # 允许最大 100MB 的文件
  ```

## 其他可配置的环境变量

除了 docker-compose.yml 中已配置的变量外，DocReader 还支持以下环境变量（可根据需要添加）：

### gRPC 配置

- `DOCREADER_GRPC_MAX_WORKERS`: gRPC 服务的最大工作线程数（默认：4）
- `DOCREADER_GRPC_PORT`: gRPC 服务监听端口（默认：50051）

### 解析器资源控制

- `DOCREADER_MARKITDOWN_MAX_WORKERS`: MarkItDown 解析的最大并发数（默认：1，设为 0 可关闭限流）
- `DOCREADER_PDF_RENDER_MAX_WORKERS`: 扫描 PDF 渲染为图片的最大并发数（默认：1，设为 0 可关闭限流）
- `DOCREADER_PDF_RENDER_DPI`: 扫描 PDF 渲染 DPI（默认：200）
- `DOCREADER_PDF_JPEG_QUALITY`: 扫描 PDF 输出 JPEG 质量（默认：85，范围会自动限制在 1-95）

### OCR / VLM

DocReader 自身不再内置 OCR 与 VLM 后端。扫描 PDF 会被渲染为 JPEG 图片后交由 Go App 侧调用 OCR/VLM 服务处理，相关配置请参考主项目文档。

### 存储

DocReader 不直接访问对象存储：解析出的图片以内联字节返回，由 Go App 按其存储配置（`local` 或 S3 兼容）持久化。
因此这里没有存储相关的环境变量。

### 代理配置

如果需要通过代理访问外部服务：

- `EXTERNAL_HTTP_PROXY`: HTTP 代理地址
- `EXTERNAL_HTTPS_PROXY`: HTTPS 代理地址

### 图像处理配置

扫描 PDF 会被渲染为 JPEG 图片后交给 Go App 侧 OCR 处理。如果在导入多个大 PDF 时出现资源占用过高，
可以优先调低 `DOCREADER_PDF_RENDER_MAX_WORKERS` 或 `DOCREADER_MARKITDOWN_MAX_WORKERS`。

## 配置示例

### 基础配置

```yaml
docreader:
  environment:
    - MAX_FILE_SIZE_MB=50
```

### 高级配置（启用 MinerU）

```yaml
docreader:
  environment:
    - MINERU_ENDPOINT=http://mineru:8080
    - MAX_FILE_SIZE_MB=100
```

## 常见问题

### 1. DocReader 服务无法启动？

检查容器日志中是否存在依赖缺失或权限相关错误，DocReader 本身不访问对象存储，存储相关配置（`STORAGE_TYPE`、`S3_*`）属于 app 服务。

### 2. 图片无法显示？

图片由 app 服务保存到对象存储，请检查 app 的 `STORAGE_TYPE` 与 `S3_*` 配置（参见 `.env.example`），并确认存储服务从 app 容器内可达。

### 3. 文件上传失败？

检查 `MAX_FILE_SIZE_MB` 配置，确保限制足够大。同时需要确保前端和后端服务的文件大小限制保持一致。

## 服务健康检查

DocReader 服务配置了健康检查：

```yaml
healthcheck:
  test: ["CMD", "grpc_health_probe", "-addr=localhost:50051"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s
```

可以通过以下命令检查服务状态：

```bash
docker ps | grep docreader
docker logs Yuheng-docreader
```

## 更多信息

- 服务端口：50051（gRPC）
- 容器名称：Yuheng-docreader
- 网络：Yuheng-network
- 重启策略：unless-stopped
