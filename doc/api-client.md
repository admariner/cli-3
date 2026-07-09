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
  A test (`TestNoPlanetscaleGoDependency` in `internal/planetscale/`) fails
  the build if the module shows up in `go.mod` or in any import.
- Service interfaces (e.g. `DatabasesService`) live next to their
  implementations; hand-written mocks are in `internal/mock/` and must be
  updated when an interface changes, same as before.
- The client's `User-Agent` starts with `pscale-cli/<version>`, set by the
  CLI at startup via `planetscale.WithUserAgent` (see `internal/cmd/root.go`).
  `WithUserAgent` keeps upstream's prepend semantics so the package stays
  byte-identical to what the mirror publishes (see below); the full header
  is `pscale-cli/<version> planetscale-go/unknown`.
- Keep this package self-contained: do not import other CLI packages from
  `internal/planetscale/`. The mirrored copy must build against
  planetscale-go's own minimal `go.mod`.

## The planetscale-go repository

The public planetscale-go module still exists for external users, but it
is now a read-only mirror of this package. The CLI copy is the source of
truth; do not make client changes in the planetscale-go repo.

The mirror is the `Mirror client to planetscale-go` workflow
(`.github/workflows/mirror-planetscale-go.yml`). It runs on CLI releases
or manually, copies `internal/planetscale/` into planetscale-go (minus
`doc.go` and `dependency_test.go`, which are CLI-only), builds and tests
the copy standalone, and opens a PR there with a `gorelease` API
compatibility report. It never pushes to main: a human reviews the PR and
picks the next tag, bumping the major version if the report shows
incompatible changes. Auth comes from the CLI GitHub App (the same one
release.yml uses), which must be installed on planetscale-go.
