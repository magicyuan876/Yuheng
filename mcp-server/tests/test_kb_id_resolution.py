from yuheng_mcp_server import YuhengClient, _normalize_kb_entries

# Shapes mirror GET /knowledge-bases wire JSON.
# See internal/handler/knowledgebase.go (buildKBListResponse).


def test_normalize_kb_entries_passes_through_rows():
    resp = {
        "success": True,
        "data": [{"id": "kb-owned-1", "name": "My Docs"}],
    }
    assert _normalize_kb_entries(resp) == [{"id": "kb-owned-1", "name": "My Docs"}]


def test_normalize_kb_entries_skips_rows_without_id():
    resp = {"success": True, "data": [{"name": "no id"}, "not a row", {"id": "kb-1", "name": "ok"}]}
    assert _normalize_kb_entries(resp) == [{"id": "kb-1", "name": "ok"}]


def test_resolve_kb_id_matches_name_case_insensitively(monkeypatch):
    client = YuhengClient("http://example.test/api/v1", "")

    def fake_request(method, path, **kwargs):
        if path == "/knowledge-bases":
            return {
                "success": True,
                "data": [{"id": "kb-owned-1", "name": "技术文档库"}, {"id": "kb-owned-2", "name": "Same Name"}],
            }
        raise AssertionError(f"unexpected request: {method} {path}")

    monkeypatch.setattr(client, "_request", fake_request)

    assert client.resolve_kb_id("技术文档库") == "kb-owned-1"
    assert client.resolve_kb_id("same name") == "kb-owned-2"


def test_resolve_kb_id_raises_when_name_unknown(monkeypatch):
    client = YuhengClient("http://example.test/api/v1", "")
    monkeypatch.setattr(client, "_request", lambda method, path, **kwargs: {"success": True, "data": []})

    try:
        client.resolve_kb_id("missing")
    except ValueError as exc:
        assert "list_knowledge_bases" in str(exc)
    else:
        raise AssertionError("an unknown name must raise ValueError")
