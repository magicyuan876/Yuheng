# Yuheng MCP Server 安装和使用指南

## 快速开始

### 1. 安装依赖
```bash
uv sync        # 按 uv.lock 安装锁定的依赖
```

### 2. 设置环境变量
```bash
# Linux/macOS
export YUHENG_BASE_URL="http://localhost:8080/api/v1"
export YUHENG_API_KEY="your_api_key_here"

# Windows PowerShell
$env:YUHENG_BASE_URL="http://localhost:8080/api/v1"
$env:YUHENG_API_KEY="your_api_key_here"

# Windows CMD
set YUHENG_BASE_URL=http://localhost:8080/api/v1
set YUHENG_API_KEY=your_api_key_here
```

### 3. 运行服务器

```bash
uv run python main.py
```

## 作为 Python 包安装

### 开发模式安装
```bash
pip install -e .
```

安装后可以使用命令行工具：
```bash
yuheng-mcp-server
```

### 生产模式安装
```bash
pip install .
```

### 构建分发包
```bash
uv build
```

## 命令行选项

主入口点 `main.py` 支持以下选项：

```bash
python main.py --help                 # 显示帮助信息
python main.py --check-only           # 仅检查环境配置
python main.py --verbose              # 启用详细日志
python main.py --version              # 显示版本信息
```

## 环境检查

运行以下命令检查环境配置：
```bash
python main.py --check-only
```

这将显示：
- Yuheng API 基础 URL 配置
- API 密钥设置状态
- 依赖包安装状态

## 故障排除

### 1. 导入错误
如果遇到 `ImportError`，请确保：
- 已安装所有依赖：`uv sync`
- Python 版本兼容（推荐 3.10+）
- 没有文件名冲突

### 2. 连接错误
如果无法连接到 Yuheng API：
- 检查 `YUHENG_BASE_URL` 是否正确
- 确认 Yuheng 服务正在运行
- 验证网络连接

### 3. 认证错误
如果遇到认证问题：
- 检查 `YUHENG_API_KEY` 是否设置
- 确认 API 密钥有效
- 验证权限设置

## 开发模式

### 项目结构
```
Yuheng/mcp-server/
├── __init__.py              # 包初始化文件
├── main.py                  # 启动入口
├── yuheng_mcp_server.py   # MCP 服务器实现
├── upload_paths.py        # 上传路径校验与目录白名单
├── pyproject.toml         # 项目元数据（包名 yuheng-mcp）
├── uv.lock                # 锁定的依赖集（CI、镜像与许可证清单都以它为准）
├── MANIFEST.in            # 包含文件清单
├── LICENSE                # 许可证
├── README.md              # 项目说明
└── INSTALL.md             # 安装指南
```

### 添加新功能
1. 在 `YuhengClient` 类中添加新的 API 方法
2. 用 `@mcp.tool()` 装饰器注册一个新工具函数：参数用类型标注（schema 自动生成），描述写在 docstring 里，函数体调用上面新增的客户端方法
3. 更新文档和测试

### 测试
```bash
# 运行基本测试
python check_imports.py

# 测试环境配置
python main.py --check-only

# 测试服务器启动
python main.py --verbose
```

## 部署

### Docker 部署
目录下的 `Dockerfile` 只安装 `uv.lock` 锁定的依赖（`uv export --frozen` 生成带哈希的清单，`pip install --require-hashes` 安装），以 Streamable HTTP 传输启动：
```bash
docker build -t yuheng-mcp .
docker run -e MCP_SERVER_AUTH_TOKEN=... -e YUHENG_API_KEY=... -p 8000:8000 yuheng-mcp
```

### 系统服务
创建 systemd 服务文件 `/etc/systemd/system/yuheng-mcp.service`：
```ini
[Unit]
Description=Yuheng MCP Server
After=network.target

[Service]
Type=simple
User=yuheng
WorkingDirectory=/opt/yuheng-mcp
Environment=YUHENG_BASE_URL=http://localhost:8080/api/v1
Environment=YUHENG_API_KEY=your_api_key
ExecStart=/usr/local/bin/yuheng-mcp-server
Restart=always

[Install]
WantedBy=multi-user.target
```

启用服务：
```bash
sudo systemctl enable yuheng-mcp
sudo systemctl start yuheng-mcp
```

## 支持

如果遇到问题，请：
1. 查看日志输出
2. 检查环境配置
3. 参考故障排除部分
4. 提交 Issue 到项目仓库: https://github.com/magicyuan876/yuheng/issues