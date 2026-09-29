FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
RUN pip install --no-cache-dir -e .

ENV MCP_HOST=0.0.0.0
ENV MCP_PORT=8000
ENV YUHENG_BASE_URL=http://app:8080/api/v1
# Set MCP_SERVER_AUTH_TOKEN at runtime; HTTP transport refuses to start without it.

# Run unprivileged: the server only reads its own code and, for uploads, the
# directories named by MCP_ALLOWED_UPLOAD_DIRS (mount those readable by uid 10001).
RUN useradd -r -m -u 10001 mcp
USER mcp

EXPOSE 8000

CMD ["yuheng-mcp-server", "--transport", "http", "--host", "0.0.0.0", "--port", "8000"]
