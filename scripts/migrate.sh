#!/bin/bash
set -e

# Get the script directory and project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Load .env file if it exists (for development mode)
if [ -f "$PROJECT_ROOT/.env" ]; then
    echo "Loading .env file from $PROJECT_ROOT/.env"
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Database connection details (can be overridden by environment variables)
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-Yuheng}

# Percent-encode one URL component (RFC 3986 unreserved characters pass
# through). Plain bash on purpose: this script runs inside the app image, which
# carries no Python. It used to shell out to python3 with the password pasted
# into the Python source, so a quote in the password broke the command, and
# without python3 it fell back to the raw password, which breaks on any of
# @ : / ? #. LC_ALL=C makes it walk bytes, so UTF-8 is encoded byte by byte;
# the mask undoes the sign extension older bash (macOS 3.2) applies above 0x7F.
urlencode() {
    local LC_ALL=C s="$1" out="" c n hex i
    for ((i = 0; i < ${#s}; i++)); do
        c="${s:i:1}"
        case "$c" in
            [a-zA-Z0-9.~_-]) out+="$c" ;;
            *)
                printf -v n '%d' "'$c"
                printf -v hex '%%%02X' $((n & 255))
                out+="$hex"
                ;;
        esac
    done
    printf '%s' "$out"
}

# Use versioned migrations directory
MIGRATIONS_DIR="${MIGRATIONS_DIR:-migrations/versioned}"

# Check if migrate tool is installed
if ! command -v migrate &> /dev/null; then
    echo "Error: migrate tool is not installed"
    echo "Install it with: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
    exit 1
fi

# Construct the database URL
# If DB_URL is already set in .env, use it but ensure sslmode=disable is set
# Otherwise, construct it from individual components
if [ -n "$DB_URL" ]; then
    # If DB_URL already exists, ensure sslmode=disable is set (unless sslmode is already specified)
    if [[ "$DB_URL" != *"sslmode="* ]]; then
        # Add sslmode=disable if not present
        if [[ "$DB_URL" == *"?"* ]]; then
            DB_URL="${DB_URL}&sslmode=disable"
        else
            DB_URL="${DB_URL}?sslmode=disable"
        fi
    elif [[ "$DB_URL" == *"sslmode=require"* ]] || [[ "$DB_URL" == *"sslmode=prefer"* ]]; then
        # Replace sslmode=require/prefer with sslmode=disable for local dev
        DB_URL="${DB_URL//sslmode=require/sslmode=disable}"
        DB_URL="${DB_URL//sslmode=prefer/sslmode=disable}"
    fi
else
    DB_URL="postgres://$(urlencode "$DB_USER"):$(urlencode "$DB_PASSWORD")@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
fi

# Execute migration based on command
case "$1" in
    up)
        # DB_URL and DB_PASSWORD are not echoed: both carry the password, and
        # this runs in container logs and CI output.
        echo "Running migrations up..."
        echo "DB_USER: ${DB_USER}"
        echo "DB_HOST: ${DB_HOST}"
        echo "DB_PORT: ${DB_PORT}"
        echo "DB_NAME: ${DB_NAME}"
        echo "MIGRATIONS_DIR: ${MIGRATIONS_DIR}"
        migrate -path ${MIGRATIONS_DIR} -database ${DB_URL} up
        ;;
    down)
        echo "Running migrations down..."
        migrate -path ${MIGRATIONS_DIR} -database ${DB_URL} down
        ;;
    create)
        if [ -z "$2" ]; then
            echo "Error: Migration name is required"
            echo "Usage: $0 create <migration_name>"
            exit 1
        fi
        echo "Creating migration files for $2..."
        migrate create -ext sql -dir ${MIGRATIONS_DIR} -seq $2
        echo "Created:"
        echo "  - ${MIGRATIONS_DIR}/$(ls -t ${MIGRATIONS_DIR} | head -1)"
        echo "  - ${MIGRATIONS_DIR}/$(ls -t ${MIGRATIONS_DIR} | head -2 | tail -1)"
        ;;
    version)
        echo "Checking current migration version..."
        migrate -path ${MIGRATIONS_DIR} -database ${DB_URL} version
        ;;
    force)
        if [ -z "$2" ]; then
            echo "Error: Version number is required"
            echo "Usage: $0 force <version>"
            echo "Note: Use -1 to reset to no version (allows re-running all migrations)"
            exit 1
        fi
        VERSION="$2"
        echo "Forcing migration version to $VERSION..."
        # Use env to pass the command, avoiding shell flag parsing issues with negative numbers
        env migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" force -- "$VERSION"
        ;;
    goto)
        if [ -z "$2" ]; then
            echo "Error: Version number is required"
            echo "Usage: $0 goto <version>"
            exit 1
        fi
        echo "Migrating to version $2..."
        migrate -path ${MIGRATIONS_DIR} -database ${DB_URL} goto $2
        ;;
    *)
        echo "Usage: $0 {up|down|create <migration_name>|version|force <version>|goto <version>}"
        exit 1
        ;;
esac

echo "Migration command completed successfully" 