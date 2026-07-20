#!/usr/bin/env bash
#
# End-to-end test of `pscale branch resize --parameters`, `pscale branch
# parameters list`, and `pscale branch resize status|cancel` against a local
# api-bb stack. Iterates the entire parameter catalog of a branch and checks:
#
#   - immutable parameters are rejected by the CLI preflight
#   - mutable parameters round-trip through the API: setting the current
#     value returns {"result": "no_change"} and mutates nothing
#   - out-of-range values surface the API's real validation message
#   - invalid select options surface a real error (not the internal fallback)
#   - unknown parameters and missing namespaces fail fast with guidance
#   - a real change can be created, observed via status, and canceled
#
# Usage: script/test-parameters.sh [database] [branch]
# Env:   PSCALE_BIN (default bin/pscale), PSCALE_API_URL, PSCALE_ORG

set -uo pipefail

cd "$(dirname "$0")/.."

BIN="${PSCALE_BIN:-bin/pscale}"
DB="${1:-import-9gb}"
BRANCH="${2:-main}"
API="${PSCALE_API_URL:-http://api.pscaledev.com:3000/v1}"
ORG_FLAG=()
[[ -n "${PSCALE_ORG:-}" ]] && ORG_FLAG=(--org "$PSCALE_ORG")

PASS=0
FAIL=0
SKIP=0
FAILURES=()

run() {
  "$BIN" "$@" "${ORG_FLAG[@]}" --api-url "$API" --format json 2>&1
}

# check <description> <output> <expected substring>...
# Passes when the output contains every expected substring.
check() {
  local desc="$1" out="$2"
  shift 2
  local want ok=1
  for want in "$@"; do
    if [[ "$out" != *"$want"* ]]; then
      ok=0
      FAILURES+=("$desc"$'\n'"    wanted: $want"$'\n'"    got:    ${out//$'\n'/ }")
      break
    fi
  done
  if [[ $ok -eq 1 ]]; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
  fi
}

# check_not <description> <output> <forbidden substring>
check_not() {
  local desc="$1" out="$2" bad="$3"
  if [[ "$out" == *"$bad"* ]]; then
    FAIL=$((FAIL + 1))
    FAILURES+=("$desc"$'\n'"    must not contain: $bad"$'\n'"    got: ${out//$'\n'/ }")
  else
    PASS=$((PASS + 1))
  fi
}

echo "== pscale branch parameters end-to-end test =="
echo "binary: $BIN  database: $DB  branch: $BRANCH  api: $API"
echo

echo "-- auth check"
# Note: --api-url must precede the auth subcommand; auth check has its own
# --api-url flag that otherwise shadows the global one.
if ! "$BIN" --api-url "$API" auth check --format json >/dev/null 2>&1; then
  echo "FATAL: pscale auth check failed. Run: pscale auth login --api-url http://auth.pscaledev.com:3000" >&2
  exit 1
fi

echo "-- resetting: cancel any queued change request"
run branch resize cancel "$DB" "$BRANCH" >/dev/null

echo "-- fetching parameter catalog"
CATALOG="$(run branch parameters list "$DB" "$BRANCH")"
if ! jq -e 'type == "array" and length > 0' <<<"$CATALOG" >/dev/null 2>&1; then
  echo "FATAL: parameters list did not return a non-empty array:" >&2
  echo "$CATALOG" | head -10 >&2
  exit 1
fi
TOTAL="$(jq 'length' <<<"$CATALOG")"
echo "   $TOTAL parameters"
echo

echo "-- static error cases"
out="$(run branch resize "$DB" "$BRANCH")"
check "no flags -> nothing to change" "$out" "nothing to change"

out="$(run branch resize "$DB" "$BRANCH" --parameters "max_connections=1")"
check "missing namespace -> guidance" "$out" "namespace"

out="$(run branch resize "$DB" "$BRANCH" --parameters "pgconf.max_connections")"
check "missing value -> usage error" "$out" "namespace.name=value"

out="$(run branch resize "$DB" "$BRANCH" --parameters "pgconf.definitely_not_real=1")"
check "unknown parameter -> preflight error" "$out" "unknown parameter" "pscale branch parameters list"
echo

echo "-- sweeping catalog ($TOTAL parameters)"
i=0
while IFS= read -r param; do
  i=$((i + 1))
  ns="$(jq -r '.namespace' <<<"$param")"
  name="$(jq -r '.name' <<<"$param")"
  key="$ns.$name"
  immutable="$(jq -r '.immutable' <<<"$param")"
  value="$(jq -r '.value // empty' <<<"$param")"
  printf '   [%3d/%3d] %s\r' "$i" "$TOTAL" "$key"

  if [[ "$immutable" == "true" ]]; then
    out="$(run branch resize "$DB" "$BRANCH" --parameters "$key=$value")"
    check "immutable $key -> preflight rejects" "$out" "cannot be changed"
    continue
  fi

  if [[ -z "$value" ]]; then
    SKIP=$((SKIP + 1))
    continue
  fi

  # Round-trip: setting the current value must be accepted by the API and
  # detected as a no-op. Proves parsing, preflight, and API validation all
  # work for this parameter without mutating anything.
  out="$(run branch resize "$DB" "$BRANCH" --parameters "$key=$value")"
  check "no-op set $key=$value -> no_change" "$out" '"result": "no_change"'

  # Out-of-range: numeric max -> exceed it, expect the API's real message.
  max_num="$(jq -r 'if (.max | type) == "number" then .max else empty end' <<<"$param")"
  if [[ -n "$max_num" ]]; then
    over="$(jq -r '.max * 2 + 1 | floor' <<<"$param")"
    out="$(run branch resize "$DB" "$BRANCH" --parameters "$key=$over")"
    check "over-max set $key=$over -> validation message" "$out" '"status": "error"' "less than or equal"
    check_not "over-max set $key -> no internal error" "$out" "unrecognized error response"
  fi

  # Invalid option: select-typed parameters must reject a bogus option with a
  # real message, not the internal fallback.
  has_options="$(jq -r '(.options // []) | length' <<<"$param")"
  if [[ "$has_options" -gt 0 ]]; then
    out="$(run branch resize "$DB" "$BRANCH" --parameters "$key=bogus_option_xyz")"
    check "invalid option $key -> validation message" "$out" '"status": "error"'
    check_not "invalid option $key -> no internal error" "$out" "unrecognized error response"
    check_not "invalid option $key -> not accepted" "$out" '"result"'
  fi
done < <(jq -c '.[]' <<<"$CATALOG")
printf '%-60s\n' '   done'
echo

echo "-- change / status / cancel lifecycle"
# Pick a mutable integer parameter with numeric bounds and derive a valid
# value different from the current one.
lifecycle="$(jq -c '[.[] | select(.immutable == false and .parameter_type == "integer" and (.min | type) == "number" and (.max | type) == "number")][0]' <<<"$CATALOG")"
if [[ "$lifecycle" == "null" || -z "$lifecycle" ]]; then
  echo "FATAL: no mutable integer parameter with numeric bounds found" >&2
  exit 1
fi
ns="$(jq -r '.namespace' <<<"$lifecycle")"
name="$(jq -r '.name' <<<"$lifecycle")"
key="$ns.$name"
cur="$(jq -r '.value' <<<"$lifecycle")"
newval="$(jq -r 'if (.value | tonumber) > .min then .min else .min + (.step // 1) end | floor' <<<"$lifecycle")"
echo "   using $key: $cur -> $newval"

out="$(run branch resize "$DB" "$BRANCH" --parameters "$key=$newval")"
check "create change $key=$newval -> change request" "$out" '"id"' '"state"'
change_id="$(jq -r '.id // empty' <<<"$out" 2>/dev/null)"

out="$(run branch resize status "$DB" "$BRANCH")"
check "status -> shows change $change_id" "$out" "$change_id"
check "status -> includes parameters" "$out" "$name"

out="$(run branch resize cancel "$DB" "$BRANCH")"
check "cancel -> canceled" "$out" '"result": "canceled"'

out="$(run branch resize status "$DB" "$BRANCH")"
check "status after cancel -> canceled" "$out" '"state": "canceled"'

out="$(run branch parameters list "$DB" "$BRANCH")"
after="$(jq -r --arg ns "$ns" --arg n "$name" '.[] | select(.namespace == $ns and .name == $n) | .value' <<<"$out" 2>/dev/null)"
if [[ "$after" == "$cur" ]]; then
  PASS=$((PASS + 1))
else
  FAIL=$((FAIL + 1))
  FAILURES+=("value restored after cancel: wanted $cur, got $after")
fi
echo

echo "== results: $PASS passed, $FAIL failed, $SKIP skipped =="
if [[ $FAIL -gt 0 ]]; then
  echo
  echo "failures:"
  for f in "${FAILURES[@]}"; do
    echo "  - $f"
  done
  exit 1
fi
