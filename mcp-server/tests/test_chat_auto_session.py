"""Tests for the chat tool's automatic session creation.

The backend removed agent endpoints; sessions remain as plain conversation
containers. ``YuhengClient.chat`` must create a fresh session via
``POST /sessions`` before streaming to ``/knowledge-chat/{session_id}``.
"""

from yuheng_mcp_server import YuhengClient

SESSION_ID = "sess-auto-1"


def _make_client(monkeypatch):
    """Wire a YuhengClient whose REST and SSE layers are fully mocked.

    Returns (client, rest_calls, sse_calls) where rest_calls records every
    _request invocation and sse_calls records every _consume_sse_stream call.
    """
    client = YuhengClient("http://example.test/api/v1", "")
    rest_calls = []
    sse_calls = []

    def fake_request(method, path, **kwargs):
        rest_calls.append((method, path, kwargs))
        if method == "POST" and path == "/sessions":
            return {
                "success": True,
                "data": {"id": SESSION_ID, "title": "MCP chat"},
            }
        raise AssertionError(f"unexpected request: {method} {path}")

    def fake_sse(url, body):
        sse_calls.append((url, body))
        return {"answer": "mock answer", "references": [], "_debug_events": []}

    monkeypatch.setattr(client, "_request", fake_request)
    monkeypatch.setattr(client, "_consume_sse_stream", fake_sse)
    return client, rest_calls, sse_calls


def test_chat_creates_session_then_streams_knowledge_chat(monkeypatch):
    client, rest_calls, sse_calls = _make_client(monkeypatch)

    result = client.chat("什么是 RAG?", knowledge_base_ids=["kb-1"])

    # Exactly one session created, and the knowledge-chat stream used its id.
    assert [(m, p) for m, p, _ in rest_calls] == [("POST", "/sessions")]
    assert len(sse_calls) == 1
    url, body = sse_calls[0]
    assert url == f"http://example.test/api/v1/knowledge-chat/{SESSION_ID}"
    assert body["query"] == "什么是 RAG?"
    assert body["knowledge_base_ids"] == ["kb-1"]
    assert body["channel"] == "api"

    # The session id is surfaced so callers can correlate the answer.
    assert result["session_id"] == SESSION_ID
    assert result["answer"] == "mock answer"


def test_chat_session_body_carries_no_kb_or_agent_fields(monkeypatch):
    """Create-session body must be a bare container: no knowledge_base_id,
    no session_strategy, no agent fields (backend agent capability removed)."""
    client, rest_calls, _ = _make_client(monkeypatch)

    client.chat("hello", knowledge_base_ids=["kb-1"])

    body = rest_calls[0][2]["json"]
    assert set(body.keys()) == {"title"}


def test_chat_omits_kb_ids_when_not_provided(monkeypatch):
    client, _, sse_calls = _make_client(monkeypatch)

    client.chat("hello")

    _, body = sse_calls[0]
    assert "knowledge_base_ids" not in body


def test_chat_propagates_web_search_enabled(monkeypatch):
    client, _, sse_calls = _make_client(monkeypatch)

    client.chat("hello", knowledge_base_ids=["kb-1"], web_search_enabled=True)

    _, body = sse_calls[0]
    assert body["web_search_enabled"] is True


def test_chat_fails_when_session_response_lacks_id(monkeypatch):
    import pytest

    client = YuhengClient("http://example.test/api/v1", "")
    monkeypatch.setattr(
        client,
        "_request",
        lambda *args, **kwargs: {"success": True, "data": {}},
    )

    with pytest.raises(Exception, match="no session id"):
        client.chat("hello", knowledge_base_ids=["kb-1"])
