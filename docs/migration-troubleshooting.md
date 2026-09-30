# Database migration troubleshooting

This guide is linked from the system info page when Yuheng's startup database
migration fails. It covers the most common causes, how to diagnose them, and
how to recover without losing data.

If none of these match your situation, jump to
[Reporting an issue](#reporting-an-issue) at the bottom.

---

## What "migration failed" means

Yuheng applies its `golang-migrate` migrations on every startup (the SQL files
are embedded in the binary, so this does not depend on the working directory).
When a migration fails, **the server stops with an error and does not start**
(`MIGRATION_FAIL_FAST`, default `true`). Starting against a half-migrated
schema would serve requests that hit columns and tables that do not exist yet.
The container exits (or restarts in a loop under `restart: unless-stopped`);
read the reason with:

```bash
docker compose logs app | grep -i migrat
```

What the failure leaves behind:

- Migrations are **not** wrapped in a transaction by the runner. A migration
  that fails on statement 3 of 5 has already applied statements 1 and 2, and
  the database is marked **dirty** at that version (see
  [Dirty migration state](#2-dirty-migration-state)).
- Extension migrations (schema registered by an extension through
  `database.RegisterMigrationSource`) run after the core ones, each with its
  own version table; the error names the source.
- `/ready` returns 503 whenever the recorded migration state is failed or
  dirty, and `/health` keeps saying only that the process is alive.

If you migrate the schema yourself (`scripts/migrate.sh up` from CI, a
separate job, a DBA) and want the server to start regardless, set
`MIGRATION_FAIL_FAST=false` (or `AUTO_MIGRATE=false` to skip migrations
entirely). With `MIGRATION_FAIL_FAST=false` a failure is logged as a warning,
the server starts, the system info page shows the partial DB version with a red
"Migration failed" tag and the captured error, and `/ready` stays at 503.

The error message is the same one logged at startup. Recent container logs are
the authoritative source; copy them before doing anything destructive.

**Take a backup before repairing anything by hand**
(see `website-docs/01-getting-started/05-backup-and-upgrade.md`).

---

## Common causes

### 1. Missing PostgreSQL extension

Many migrations require extensions (`pg_trgm`, `vector`, `pg_search`) created
by `CREATE EXTENSION IF NOT EXISTS`. **`IF NOT EXISTS` does not validate that
the extension is actually installed** — it only checks the catalog. If the
extension's shared library is missing or the role lacks `CREATE` privilege,
the statement may succeed in the migration that nominally creates it but a
later migration that uses the extension (e.g. building a `gin_trgm_ops` index)
will fail.

**Symptoms in the error**:

```
ERROR: operator class "gin_trgm_ops" does not exist for access method "gin"
ERROR: type "vector" does not exist
ERROR: function ... does not exist
```

**Fix**:

```sql
-- Connect as a superuser (typically `postgres`):
\c your_yuheng_database
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS vector;       -- required while RETRIEVE_DRIVER includes postgres
CREATE EXTENSION IF NOT EXISTS pg_search;    -- likewise; only available on ParadeDB or a self-installed build

-- Verify they are actually loaded:
SELECT extname, extversion FROM pg_extension WHERE extname IN ('pg_trgm','vector','pg_search');
```

Both `vector` and `pg_search` are also checked at startup: the server refuses to
start when either is missing. Managed PostgreSQL services usually cannot install
`pg_search`; use the ParadeDB image from `docker-compose.yml` or install
pgvector and pg_search yourself.

Then restart Yuheng. The next startup will pick up where the failing
migration left off (if it left the database dirty, repair that first: see
section 2).

If `CREATE EXTENSION` itself errors with **"could not open extension control
file"** or **"permission denied"**, the extension is not installed on your
PostgreSQL server — install the corresponding OS package (e.g.
`postgresql-contrib` for `pg_trgm`) or switch to an image that ships it
preinstalled, then retry.

### 2. Dirty migration state

If a migration crashed or failed partway through (OOM, container kill, a
statement error) `golang-migrate` marks the schema "dirty" at the failing
version, and the next start refuses to continue:

```
database is in a dirty state at version N ...
```

Yuheng does **not** guess how to continue by default: some of the migration's
statements were applied and some were not, and only you can tell which. To
repair it by hand:

1. Read the error of the failing migration (in the log of the run that failed)
   and open `migrations/versioned/<N>_*.up.sql`. Which statements were applied?
2. Undo or finish those statements yourself with `psql`, or restore the backup
   you took before upgrading.
3. Set the version table to the last migration that fully applied, which is
   the **previous version that exists** in `migrations/versioned/`, then
   re-run. That is not always `N - 1`: the numbering has a gap (000090–000119
   are unused because the online-documents module reserved 000120–000139), so
   for a dirty 000120 the previous version is **89**. golang-migrate cannot
   continue from a version that has no file, so `force 119` would leave you
   stuck. The startup error prints the right number for you.

   ```bash
   make migrate-version            # confirm what is recorded
   make migrate-force version=<previous version>
   make migrate-up                 # optional: apply pending migrations now
   ```

   In the app container: `docker exec Yuheng-app ./scripts/migrate.sh force <previous version>`
   (the image carries `scripts/`, `migrations/` and the `migrate` CLI; the
   script reads `DB_*` from the container environment).
4. Restart Yuheng.

**`AUTO_RECOVER_DIRTY=true`** (default `false`) makes the server do step 3 by
itself and retry the migration. Only enable it when you know the interrupted
migration can be run twice without harm: every statement guarded
(`IF NOT EXISTS`, `IF EXISTS`, `ON CONFLICT DO NOTHING`) or a pure no-op on a
second pass. An unguarded `ALTER TABLE ... ADD COLUMN` fails again and leaves
the database dirty once more; a non-idempotent data rewrite silently applies
twice. Many migrations in this repository are guarded, but not all of them, and
that is why it is off by default.

Extension sources have their own version table; pass its name to the migrate
CLI (`x-migrations-table=<table>` in the database URL) when repairing one.

### 3. Insufficient privileges on the database role

Some migrations create extensions or alter shared catalogs, which require
either superuser or `CREATEROLE` / `CREATEDB`. Errors look like:

```
ERROR: permission denied to create extension "pg_trgm"
ERROR: must be owner of database ...
```

**Fix**: grant the role used by `DB_USER` the necessary privileges, or
pre-create the extensions / objects as a superuser ahead of time, then
restart. The migration's `CREATE EXTENSION IF NOT EXISTS` will then no-op.

### 4. Out-of-disk during `CREATE INDEX`

GIN / pgvector indexes can require significant temporary space. Errors:

```
ERROR: could not extend file ...: No space left on device
ERROR: cannot create temporary tables in transaction
```

**Fix**: free disk on the volume backing `PGDATA`, then restart. The
migration will retry the index build.

### 5. Schema drift from manual edits

If you previously edited tables / columns by hand and a later migration
expects the original shape, it will fail with mismatched-type errors. The
safest recovery is to align the live schema with the previous successful
migration's `*.up.sql` and then re-run pending migrations.

---

## Generic diagnostic checklist

1. **Read the full error**: the cached message in the UI is truncated only by
   your browser scroll — it is the complete `golang-migrate` error. The
   container log shows the same content with stack context.
2. **Identify the failing migration**: the version number in the error (or
   `make migrate-version`) points to a file under `migrations/versioned/`.
   Open `migrations/versioned/<version>_*.up.sql` and look for the statement
   matching the error type (extension, index, function, foreign key, …).
3. **Run the failing statement manually** against the DB using `psql`. The
   error will be far more specific than the migration wrapper's.
4. **Fix the underlying cause** (install extension, fix privileges, free
   disk, …), then either:
   - Restart Yuheng (the migration is retried on start-up; if the database is
     dirty, repair it first as described above); **or**
   - Run `make migrate-up` from a checkout to apply migrations outside the
     server process.
5. **Verify**: `curl localhost:8080/ready` returns 200, the system info page
   shows the DB version without the "Migration failed" tag, and the previously broken feature (Wiki, KG,
   …) should start producing output.

---

## Reporting an issue

If you've worked through the checklist and the migration still fails, please
open an issue at:

<https://github.com/magicyuan876/yuheng/issues/new?template=bug_report.yml>

Include:

- Yuheng version + commit ID (from the system info page).
- The full error from the system info page (or container logs).
- PostgreSQL version (`SELECT version();`) and how it was deployed (vanilla,
  ParadeDB, Aurora, Aliyun RDS, …).
- The output of:
  ```sql
  SELECT extname, extversion FROM pg_extension;
  ```
- Any non-default values of `RETRIEVE_DRIVER`, `AUTO_MIGRATE`,
  `MIGRATION_FAIL_FAST` and `AUTO_RECOVER_DIRTY`.

The "Report issue" link on the system info page pre-fills a body with the
captured error for you — clicking it is the fastest path.
