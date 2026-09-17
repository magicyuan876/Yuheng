#!/usr/bin/env python3
"""
Yuheng MCP Server Package

A Model Context Protocol server that provides access to the Yuheng knowledge management API.
"""

__version__ = "1.1.1"
__author__ = "Yuheng Team"
__description__ = "Yuheng MCP Server - Model Context Protocol server for Yuheng API"

from yuheng_mcp_server import YuhengClient, run

__all__ = ["YuhengClient", "run"]
