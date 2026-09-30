FROM python:3.11-slim AS lock

# uv.lock is the dependency set CI tests and the third-party notices list, so
# the image installs exactly that: uv turns the lock into a hash-pinned
# requirements file, and pip below refuses anything that does not match it.
RUN pip install --no-cache-dir uv==0.11.9
WORKDIR /src
COPY pyproject.toml uv.lock ./
RUN uv export --frozen --no-emit-project --format requirements-txt -o requirements.lock

FROM python:3.11-slim

WORKDIR /app
COPY --from=lock /src/requirements.lock .
RUN pip install --no-cache-dir --require-hashes -r requirements.lock

# The server runs from source as `python main.py`; installing the project
# itself would only add a console-script shim and a build backend resolved
# outside the lock.
COPY . .

ENV MCP_HOST=0.0.0.0
ENV MCP_PORT=8000
ENV YUHENG_BASE_URL=http://app:8080/api/v1
# Set MCP_SERVER_AUTH_TOKEN at runtime; HTTP transport refuses to start without it.

# Run unprivileged: the server only reads its own code and, for uploads, the
# directories named by MCP_ALLOWED_UPLOAD_DIRS (mount those readable by uid 10001).
RUN useradd -r -m -u 10001 mcp
USER mcp

EXPOSE 8000

CMD ["python", "main.py", "--transport", "http", "--host", "0.0.0.0", "--port", "8000"]
