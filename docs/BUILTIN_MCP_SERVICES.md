# 内置 MCP 服务管理指南

> 本文为历史保留说明。服务端「内置 MCP 服务」功能已随 Agent 能力剥离一并移除：系统不再在数据库中内置 MCP 服务配置，原 `mcp_services` 相关表已由迁移 `000089_drop_agent_infra` 删除。以下第一～三节保留该功能的原始说明，供历史查阅；新用户请直接阅读第四节「替代方案」。

## 一、原功能概述

内置 MCP 服务是系统级别的 MCP（Model Context Protocol）服务配置，对所有空间可见，但敏感信息会被隐藏，且不可编辑或删除。内置 MCP 服务通常用于提供系统默认的外部工具和资源接入，确保所有空间都能使用统一的 MCP 服务。

## 二、原功能特性

- **所有空间可见**：内置 MCP 服务对所有空间都可见，无需单独配置
- **安全保护**：内置 MCP 服务的敏感信息（URL、认证配置、Headers、环境变量）会被隐藏，无法查看详情
- **只读保护**：内置 MCP 服务不能被编辑或删除，仅支持测试连接
- **统一管理**：由系统管理员统一维护，确保配置一致性和安全性

## 三、原添加方式

内置 MCP 服务需要通过数据库直接插入：向 `mcp_services` 表写入 `is_builtin = true` 的记录（字段含 `name` / `description` / `transport_type`(`sse` 或 `http-streamable`) / `url` / `auth_config` / `advanced_config` 等），`stdio` 传输在服务端已被禁用。

> 注意：以上操作仅适用于该功能移除之前的旧版本；功能移除后该表已不存在。

## 四、替代方案

Yuheng 重新定位为纯知识库平台，MCP 的方向反转——Yuheng 不再消费/托管第三方 MCP 服务，而是作为 MCP server 把你的知识库开放给外部 Agent：

1. **`yuheng-mcp`（Python MCP server，23 个工具，stdio / SSE / HTTP）**——官方 PyPI 包，适合给 Claude Desktop、Cursor 等宿主提供完整的知识库工具面。配置见 [`mcp-server/MCP_CONFIG.md`](../mcp-server/MCP_CONFIG.md)。
2. **`yuheng mcp serve`（CLI 内置 MCP server，8 个只读工具）**——适合给编码 Agent 提供受控的只读检索 / 问答面，见 [`cli/README.md`](../cli/README.md)。
3. 需要给所有用户统一预置 MCP 配置时，改为在**各 Agent 宿主侧**分发配置文件（例如统一下发 Claude Desktop 的 `claude_desktop_config.json`），不再由 Yuheng 服务端托管。

详见 [MCP 功能使用说明](./MCP功能使用说明.md)。
