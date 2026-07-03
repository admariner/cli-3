# PlanetScale CLI — agent guide

For **any** automated agent or script using `pscale`. Always pass **`--format json`**. Substitute placeholders from the user's request or from prior command output (`org list`, `database list`, `branch list`).

If you only have the installed `pscale` binary, start here:

```bash
pscale agent-guide --format json
pscale auth check --format json
```

Use direct CLI automation for shell commands and scripts. Use the hosted PlanetScale MCP server for MCP clients.

| Placeholder | Meaning |
|-------------|---------|
| `<org>` | Organization name |
| `<database>` | Database name |
| `<branch>` | Branch name (pick one with `"ready": true` from `branch list`) |

## Flag placement

- **`--org`** is a flag on resource subcommands (`database`, `branch`, `sql`, `api`, …). It is **not** on root `pscale` — `pscale --org …` fails.
- **`--format json`** is a global flag. It can go on `pscale` or on the subcommand.
- Commands with positional args (`sql`, `branch list`, …): put **positionals first**, then flags.

```bash
# Correct
pscale auth check --format json
pscale org list --format json
pscale database list --org <org> --format json
pscale branch list <database> --org <org> --format json
pscale sql <database> <branch> --org <org> --format json --query "SELECT 1"

# Also valid — global --format
pscale --format json database list --org <org>

# Wrong — unknown flag: --org
pscale --org <org> database list --format json
```

## Workflow

1. **Guide** — discover machine-readable conventions when you do not have this file:

   ```bash
   pscale agent-guide --format json
   ```

2. **Auth** — check before anything else:

   ```bash
   pscale auth check --format json
   ```

   `"status": "ok"` and `"authenticated": true` with no blocking `issues` means proceed. `"status": "action_required"` exits non-zero — log in, pick an org, or fix credentials (see `issues` and `next_steps`).

3. **Login** (when not authenticated):

   ```bash
   pscale auth login --format json
   ```

   Pending JSON is written to **stderr** while waiting; **stdout** has a single final JSON object when login completes (`status: ok` or `action_required` if org setup fails after credentials are saved). Fields include `verification_url`, `user_code`, and `browser_opened`. Open `verification_url` manually if the browser does not open. Do not retry login in a loop without browser access.

4. **Organization** — use `"organization"` from `auth check`, ask the user, or list orgs:

   ```bash
   pscale org list --format json
   ```

   Pass `--org <org>` on resource commands (`database`, `branch`, `sql`, `api`, …). Not on `org list`.

5. **Discover resources** before SQL:

   ```bash
   pscale database list --org <org> --format json
   pscale branch list <database> --org <org> --format json
   ```

6. **Query** (read-only default):

   ```bash
   pscale sql <database> <branch> --org <org> --format json --query "SELECT 1"
   ```

## Flags

| Flag | Purpose |
|------|---------|
| `--format json` | JSON on stdout |
| `--org <org>` | Organization (on resource subcommands only) |
| `--api-url` | Non-production API base URL — pass on every command when not using production |

## Authentication

`pscale auth login` stores credentials in the OS keychain; agents on the same machine reuse them.

Headless / CI: pass `--service-token-id` and `--service-token` on the subcommand that needs auth.

## SQL

Non-interactive queries. Default **`--role` is `reader`** (unlike `pscale shell`, which defaults to admin). Use `pscale shell` for interactive sessions.

```bash
# Read (default)
pscale sql <database> <branch> --org <org> --format json --query "SELECT 1"

# Read from replica
pscale sql <database> <branch> --org <org> --format json --replica --query "SELECT 1"

# PostgreSQL — optional --dbname (default postgres)
pscale sql <database> <branch> --org <org> --format json --query "SELECT 1"

# MySQL multi-keyspace — optional --keyspace (default @primary)
pscale sql <database> <branch> --org <org> --format json --keyspace <keyspace> --query "SELECT 1"
```

| Flag | Purpose |
|------|---------|
| `--role` | `reader` (default), `writer`, `readwriter`, `admin` — same names as `pscale shell` |
| `--replica` | Route reads to replicas |
| `--dbname` | PostgreSQL database name (default `postgres`) |
| `--keyspace` | MySQL keyspace (default `@primary`) |
| `--force` | Allow destructive SQL after explicit user approval |

**`--role` by engine** (same as `pscale shell`):

| `--role` | MySQL (Vitess) | PostgreSQL |
|----------|----------------|------------|
| `reader` | Branch password, reader role | Ephemeral role inheriting `pg_read_all_data` |
| `writer` | Branch password, writer role | Role inheriting `pg_write_all_data` |
| `readwriter` | Branch password, readwriter role | Role inheriting read + write |
| `admin` | Branch password, admin role | Role inheriting `postgres` |

### Destructive SQL

`DELETE`, `DROP`, and `TRUNCATE` anywhere in a query are blocked by default (word match, not substring — `deleted_at` is fine). Returns `"status": "action_required"` with `"query_kind": "destructive"`.

1. Ask the user to approve the query.
2. Re-run with `--force` only after they approve:

```bash
pscale sql <database> <branch> --org <org> --format json --force --query "DELETE FROM ..."
```

Never use `--force` without explicit user approval.

### SQL JSON

Success: `status`, `database`, `branch`, `kind` (`mysql` or `postgresql`), `role`, `row_count`, `columns`, `rows`; `replica` when `--replica` was used.

MySQL may return synthetic column names (e.g. `:vtg1 /* INT64 */`). PostgreSQL may use names like `?column?`.

Error: one JSON object on stdout with `status: "error"`, `error`, and `next_steps`.

Destructive SQL without `--force`: `status: "action_required"`, `query_kind: "destructive"`, `issues`, and `next_steps` (includes `--force` retry command).

## API passthrough

```bash
pscale api --org <org> organizations/<org>/databases
```

## MCP

For MCP clients, use the hosted PlanetScale MCP server:

```text
https://mcp.pscale.dev/mcp/planetscale
```

See the current MCP docs: https://planetscale.com/docs/connect/mcp

Do not use the deprecated local `pscale mcp server` path unless you explicitly need backward compatibility with an old setup.
