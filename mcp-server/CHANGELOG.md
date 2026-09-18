# 更新日志

所有重要的项目更改都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
并且本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

本仓库建立之前的版本历史属于根目录 [NOTICE](../NOTICE) 中记载的上游项目，此处不再复述。

## [Unreleased]

### 新增
- Yuheng 首次公开导入：MCP Server 聚焦知识平台能力，共 23 个工具（知识库 / 文档 / 检索 / RAG 问答 / 分块 / Wiki / 模型管理），传输支持 stdio / SSE / HTTP；`chat` 工具自动创建会话，调用方只需提供 `query` 与 `knowledge_base_ids`。
