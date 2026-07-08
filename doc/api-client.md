# API client (`internal/planetscale`)

**The CLI no longer depends on `github.com/planetscale/planetscale-go`.**

As of July 2026, the Go API client that used to live in the planetscale-go
repository is vendored into this repo at `internal/planetscale/` and is
maintained here. The import path is:

```go
import ps "github.com/planetscale/cli/internal/planetscale"
```

## Why

Every API change used to require two PRs and a release dance: change
planetscale-go, tag a release, bump `go.mod` here, then write the CLI
feature. The CLI was the only internal consumer of planetscale-go, and the
published OpenAPI spec covers under half of the endpoints the CLI uses
(none of the vtctld/workflow/data-import surface), so generating a client
from the spec was not an option. Copying the client in removes the
cross-repo step entirely.

## What this means when adding or changing an endpoint

- Add the endpoint to api-bb, then add the method/types directly in
  `internal/planetscale/` in the same CLI PR. There is no SDK release to
  wait for and no version to bump.
- Do **not** add `github.com/planetscale/planetscale-go` back to `go.mod`,
  and do not update the planetscale-go repo as a prerequisite for CLI work.
- Service interfaces (e.g. `DatabasesService`) live next to their
  implementations; hand-written mocks are in `internal/mock/` and must be
  updated when an interface changes, same as before.
- The client's `User-Agent` is `pscale-cli/<version>`, set by the CLI at
  startup via `planetscale.WithUserAgent` (see `internal/cmd/root.go`).
  There is no separate library version anymore.

## The planetscale-go repository

The public planetscale-go SDK still exists for external users, but this
repo does not consume it and changes there do not affect the CLI. Its
future (maintenance mode, archive, etc.) is tracked separately.
