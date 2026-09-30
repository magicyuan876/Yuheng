#!/usr/bin/env python3
"""Regression tests for the MCP network transports and the REST client."""

import asyncio
import os
import subprocess
import sys
import threading
import unittest
from pathlib import Path

MCP_SERVER_DIR = Path(__file__).resolve().parent


class TransportRegressionTest(unittest.TestCase):
    def test_http_transport_is_stateless(self):
        import yuheng_mcp_server as srv
        from mcp.server import MCPServer

        probe = MCPServer("probe")
        probe.streamable_http_app(
            host="127.0.0.1", stateless_http=srv.STREAMABLE_HTTP_STATELESS
        )
        self.assertTrue(probe.session_manager.stateless)

    def test_yuheng_client_session_is_thread_local(self):
        from yuheng_mcp_server import YuhengClient

        client = YuhengClient("http://localhost:8080/api/v1", "test-key")
        barrier = threading.Barrier(2)
        sessions: dict[str, object] = {}

        def worker(name: str) -> None:
            barrier.wait()
            sessions[name] = client.session

        threads = [
            threading.Thread(target=worker, args=(name,))
            for name in ("a", "b")
        ]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()

        self.assertEqual(len(sessions), 2)
        self.assertIsNot(sessions["a"], sessions["b"])


class StdioToolsListTest(unittest.TestCase):
    def test_tools_list_returns_23_tools(self):
        async def _run() -> int:
            from mcp import ClientSession, StdioServerParameters
            from mcp.client.stdio import stdio_client

            params = StdioServerParameters(
                command=sys.executable,
                args=[str(MCP_SERVER_DIR / "yuheng_mcp_server.py")],
                env={
                    **os.environ,
                    "YUHENG_API_KEY": "test-key",
                },
            )
            async with stdio_client(params) as (read, write):
                async with ClientSession(read, write) as session:
                    await session.initialize()
                    tools = await session.list_tools()
                    return len(tools.tools)

        count = asyncio.run(_run())
        self.assertEqual(count, 23)


class HttpStatelessSmokeTest(unittest.TestCase):
    def test_initialize_does_not_require_mcp_session_id(self):
        import time

        port = 19876
        env = {
            **os.environ,
            "MCP_SERVER_AUTH_TOKEN": "test-token",
            "YUHENG_API_KEY": "test-key",
        }
        proc = subprocess.Popen(
            [
                sys.executable,
                "yuheng_mcp_server.py",
                "--transport",
                "http",
                "--host",
                "127.0.0.1",
                "--port",
                str(port),
            ],
            cwd=MCP_SERVER_DIR,
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        try:
            deadline = time.time() + 10
            while time.time() < deadline:
                probe = subprocess.run(
                    [
                        "curl",
                        "-s",
                        "-D",
                        "-",
                        "-o",
                        "/dev/null",
                        "-X",
                        "POST",
                        f"http://127.0.0.1:{port}/mcp",
                        "-H",
                        "Authorization: Bearer test-token",
                        "-H",
                        "Content-Type: application/json",
                        "-H",
                        "Accept: application/json, text/event-stream",
                        "-d",
                        (
                            '{"jsonrpc":"2.0","id":1,"method":"initialize",'
                            '"params":{"protocolVersion":"2025-03-26",'
                            '"capabilities":{},"clientInfo":{"name":"t","version":"1"}}}'
                        ),
                    ],
                    capture_output=True,
                    text=True,
                )
                if probe.returncode == 0 and "HTTP/" in probe.stdout:
                    break
                time.sleep(0.2)
            else:
                self.fail("HTTP server did not become ready in time")

            headers = probe.stdout.lower()
            self.assertIn("200 ok", headers)
            self.assertNotIn("mcp-session-id:", headers)
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()


if __name__ == "__main__":
    unittest.main()
