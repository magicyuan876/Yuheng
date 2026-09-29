#!/usr/bin/env bash
# End-to-end smoke test of a running Yuheng stack, through the same door a
# browser uses: the frontend's nginx. It expects a FRESH install (no user yet)
# and walks the first-run path: the page loads, the first account can register
# and becomes the only one, login issues a token, and the API tells a signed-in
# caller from an anonymous one.
#
#   BASE_URL=http://127.0.0.1:8086 scripts/smoke.sh
#
# BASE_URL         where the frontend answers (default http://127.0.0.1:${FRONTEND_PORT:-80})
# SMOKE_WAIT_SECS  how long to wait for the stack to come up (default 300)
# SMOKE_EMAIL / SMOKE_PASSWORD / SMOKE_USERNAME  the account to create
#
# Needs curl and python3 (to read JSON). It creates one account, so point it at
# a throwaway stack, never at a deployment you care about.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:${FRONTEND_PORT:-80}}"
BASE_URL="${BASE_URL%/}"
WAIT_SECS="${SMOKE_WAIT_SECS:-300}"
EMAIL="${SMOKE_EMAIL:-smoke@example.com}"
USERNAME="${SMOKE_USERNAME:-smoke}"
# Generated per run so no credential lives in the repository.
PASSWORD="${SMOKE_PASSWORD:-$(python3 -c 'import secrets; print(secrets.token_urlsafe(18))')}"

for tool in curl python3; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "smoke: $tool is required" >&2
		exit 2
	}
done

BODY="$(mktemp)"
trap 'rm -f "$BODY"' EXIT

fail() {
	echo "FAIL: $*" >&2
	if [ -s "$BODY" ]; then
		echo "  last response body: $(head -c 500 "$BODY")" >&2
	fi
	exit 1
}
pass() { echo "ok:   $*"; }

# request METHOD PATH [JSON] [TOKEN] -> prints the status code, body in $BODY.
request() {
	local method="$1" path="$2" data="${3:-}" token="${4:-}"
	local args=(-sS -o "$BODY" -w '%{http_code}' --max-time 30 -X "$method" "$BASE_URL$path")
	if [ -n "$data" ]; then
		args+=(-H 'Content-Type: application/json' --data "$data")
	fi
	if [ -n "$token" ]; then
		args+=(-H "Authorization: Bearer $token")
	fi
	curl "${args[@]}"
}

# json_get KEY... reads a value from $BODY by a chain of keys; prints
# "true"/"false" for booleans and nothing when the path is absent.
json_get() {
	python3 - "$BODY" "$@" <<'PY'
import json, sys
try:
    with open(sys.argv[1]) as f:
        v = json.load(f)
    for k in sys.argv[2:]:
        v = v[k]
except Exception:
    sys.exit(0)
print("true" if v is True else "false" if v is False else v)
PY
}

expect_status() {
	local want="$1" got="$2" what="$3"
	[ "$got" = "$want" ] || fail "$what: expected HTTP $want, got $got"
	pass "$what (HTTP $got)"
}

# 1. The stack answers. /api/v1/auth/config is public and only succeeds when
# nginx, the app and the database (migrations included) all work, so it is a
# better readiness signal than the front page alone.
echo "smoke: waiting up to ${WAIT_SECS}s for $BASE_URL"
deadline=$((SECONDS + WAIT_SECS))
until [ "$(request GET /api/v1/auth/config 2>/dev/null || true)" = "200" ]; do
	if [ "$SECONDS" -ge "$deadline" ]; then
		fail "the stack did not answer GET /api/v1/auth/config within ${WAIT_SECS}s"
	fi
	sleep 3
done
pass "stack is up"

expect_status 200 "$(request GET /)" "GET / serves the frontend"
grep -qi '<div id="app"\|<html' "$BODY" || fail "GET / did not return an HTML page"

# 2. A fresh install offers to create the first (administrator) account.
expect_status 200 "$(request GET /api/v1/auth/config)" "GET /api/v1/auth/config"
[ "$(json_get registration_open)" = "true" ] || fail "registration_open should be true on a fresh install"
[ "$(json_get first_user)" = "true" ] || fail "first_user should be true on a fresh install"
pass "fresh install: registration_open and first_user are true"

# 3. The first account registers ...
payload="$(python3 -c 'import json,sys; print(json.dumps({"username":sys.argv[1],"email":sys.argv[2],"password":sys.argv[3]}))' \
	"$USERNAME" "$EMAIL" "$PASSWORD")"
expect_status 201 "$(request POST /api/v1/auth/register "$payload")" "first registration"

# ... and closes the door behind it.
second="$(python3 -c 'import json,sys; print(json.dumps({"username":"second","email":"second@example.com","password":sys.argv[1]}))' "$PASSWORD")"
expect_status 403 "$(request POST /api/v1/auth/register "$second")" "second registration is refused"
request GET /api/v1/auth/config >/dev/null
[ "$(json_get registration_open)" = "false" ] || fail "registration_open should be false after the first account"
pass "registration closed after the first account"

# 4. Login returns a token.
login="$(python3 -c 'import json,sys; print(json.dumps({"email":sys.argv[1],"password":sys.argv[2]}))' "$EMAIL" "$PASSWORD")"
expect_status 200 "$(request POST /api/v1/auth/login "$login")" "login"
TOKEN="$(json_get token)"
[ -n "$TOKEN" ] || fail "login returned no token"
pass "login returned a token"

# 5. Protected routes: 401 without a token, 200 with one.
expect_status 401 "$(request GET /api/v1/auth/me)" "GET /api/v1/auth/me without a token"
expect_status 200 "$(request GET /api/v1/auth/me '' "$TOKEN")" "GET /api/v1/auth/me with the token"
[ "$(json_get data user email)" = "$EMAIL" ] || fail "/auth/me returned a different user"
expect_status 401 "$(request GET /api/v1/knowledge-bases)" "GET /api/v1/knowledge-bases without a token"
expect_status 200 "$(request GET /api/v1/knowledge-bases '' "$TOKEN")" "GET /api/v1/knowledge-bases with the token"

# 6. A wrong password does not log in.
bad="$(python3 -c 'import json,sys; print(json.dumps({"email":sys.argv[1],"password":"wrong-password"}))' "$EMAIL")"
expect_status 401 "$(request POST /api/v1/auth/login "$bad")" "login with a wrong password"

echo "smoke: all checks passed"
