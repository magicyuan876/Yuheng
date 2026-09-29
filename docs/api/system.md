# 系统管理 API

[返回目录](./README.md)

| 方法   | 路径                              | 描述                   |
| ------ | --------------------------------- | ---------------------- |
| GET    | `/system/capabilities`            | 获取部署能力清单       |
| GET    | `/system/info`                    | 获取系统信息           |
| GET    | `/system/parser-engines`          | 获取解析引擎列表       |
| POST   | `/system/parser-engines/check`    | 检查解析引擎可用性     |
| POST   | `/system/docreader/reconnect`     | 重连文档解析服务       |
| GET    | `/system/storage-engine-status`   | 获取存储引擎状态       |
| POST   | `/system/storage-engine-check`    | 检查存储引擎连通性     |

## GET `/system/capabilities` - 获取部署能力清单

返回各功能模块是否已在后端注册对应路由。`supported: false` 表示 SPA 应隐藏相关入口；字段缺失或接口不可用时不应据此清空整个菜单（fail-open）。

**权限**：Viewer+（租户成员）；任意有效 API Key 可读（`apiKeyAny`）。

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/capabilities' \
--header 'Authorization: Bearer <token>' \
--header 'Content-Type: application/json'
```

**响应**:

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "capabilities": {
      "organizations": { "supported": true },
      "settings.websearch": { "supported": true },
      "settings.vectorstore": { "supported": true },
      "settings.storage": { "supported": true }
    }
  }
}
```

## GET `/system/info` - 获取系统信息

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/info' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json'
```

**响应**:

```json
{
    "data": {
        "version": "1.2.0",
        "commit_id": "a1b2c3d",
        "build_time": "2025-08-12T08:00:00Z",
        "go_version": "go1.21.5",
        "keyword_index_engine": "bleve",
        "vector_store_engine": "postgres",
        "graph_database_engine": "neo4j",
        "db_version": "20250810_001"
    },
    "success": true
}
```

## GET `/system/parser-engines` - 获取解析引擎列表

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/parser-engines' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json'
```

**响应**:

```json
{
    "data": [
        {
            "name": "docreader",
            "label": "DocReader",
            "description": "高精度文档解析引擎",
            "available": true
        },
        {
            "name": "tika",
            "label": "Apache Tika",
            "description": "通用文档解析引擎",
            "available": false
        }
    ],
    "connected": true,
    "success": true
}
```

## POST `/system/parser-engines/check` - 检查解析引擎可用性

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/parser-engines/check' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json' \
--data '{
    "addr": "http://docreader:8000"
}'
```

**响应**:

```json
{
    "data": [
        {
            "name": "docreader",
            "label": "DocReader",
            "description": "高精度文档解析引擎",
            "available": true
        }
    ],
    "success": true
}
```

## POST `/system/docreader/reconnect` - 重连文档解析服务

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/docreader/reconnect' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json' \
--data '{
    "addr": "http://docreader:8000"
}'
```

**响应**:

```json
{
    "success": true
}
```

## GET `/system/storage-engine-status` - 获取存储引擎状态

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/storage-engine-status' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json'
```

**响应**:

```json
{
    "data": {
        "engines": [
            {
                "name": "local",
                "available": true,
                "description": "本地文件系统"
            },
            {
                "name": "s3",
                "available": true,
                "description": "S3 兼容对象存储（RustFS、MinIO、AWS S3、OSS、COS、TOS、OBS 等）"
            }
        ]
    },
    "success": true
}
```

## POST `/system/storage-engine-check` - 检查存储引擎连通性

`provider` 取 `local` 或 `s3`；`s3` 配置对象的字段与 [存储后端 API](./storage-backend.md) 的 `config` 相同。

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/system/storage-engine-check' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json' \
--data '{
    "provider": "s3",
    "s3": {
        "endpoint": "http://rustfs:9000",
        "region": "us-east-1",
        "access_key_id": "rustfsadmin",
        "secret_access_key": "rustfsadmin",
        "bucket_name": "yuheng",
        "addressing_style": "path"
    }
}'
```

**响应**:

```json
{
    "data": {
        "ok": true,
        "message": "连接成功"
    },
    "success": true
}
```


