# Pre-downloaded DuckDB extensions (optional)

The app image installs the DuckDB extensions its data-analysis tool needs
(`spatial`, `excel`) while it is built. They come from extensions.duckdb.org,
which is slow or unreachable from some networks (the spatial extension is
about 26 MB), and the image build then fails.

Files placed here are used instead of downloading:

```bash
scripts/fetch_duckdb_extensions.sh        # downloads them on the host, for the Docker host's architecture
docker compose build app
```

The layout is DuckDB's own: `<duckdb version>/<platform>/<name>.duckdb_extension`.
Only this README is tracked; the extension files are ignored by git.
