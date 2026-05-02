#!/bin/bash
# postgres-auto-tune.sh
#
# Renders memory and WAL defaults sized to detected host RAM into
# $POSTGRES_DATA_DIR/conf.d/00-audiusd-defaults.conf, and ensures
# postgresql.conf includes that directory.
#
# Operators override by:
#   - setting AUDIUSD_DISABLE_AUTO_TUNE=1 (skip entirely), or
#   - placing their own conf.d/99-*.conf (later includes win), or
#   - mounting a custom postgresql.auto.conf (always wins last).
#
# The shim runs every container start (so upgrading the image picks up
# new defaults), but only writes the include line into postgresql.conf
# once. Any failure path exits 0 and leaves stock defaults in place.
#
# Conservative-by-default behavior:
#   * If postgresql.conf already has any of the tuned parameters set,
#     skip (operator has hand-tuned values; do not override).
#   * If postgresql.conf already has any include_dir directive (active,
#     commented, or pointing at a different directory), skip (operator
#     intent is unclear; do not append a duplicate).
#   * If running as non-root and not as postgres, skip (cannot chown
#     conf files into postgres ownership).
#   * If postgres rejects the rendered conf via `postgres -C` preflight,
#     remove the rendered file and skip.
#
# Test injection: set POSTGRES_AUTO_TUNE_FORCE_MEM_MB to bypass memory
# detection. Used by the test suite. Production callers must not set it.

set -u

PG_AUTO_TUNE_LOG_PREFIX="[postgres-auto-tune]"

log() { echo "${PG_AUTO_TUNE_LOG_PREFIX} $*"; }

# 1 PiB in bytes. Some kernels report this (or near it) for an unlimited
# cgroup memory.max instead of the literal "max" string.
CGROUP_BOGUS_LIMIT_BYTES=1125899906842624

# Postgres GUCs the shim sets. Used to detect operator tuning conflict.
TUNED_PARAMS="shared_buffers work_mem maintenance_work_mem effective_cache_size wal_buffers max_wal_size min_wal_size"

if [ "${AUDIUSD_DISABLE_AUTO_TUNE:-0}" = "1" ]; then
    log "AUDIUSD_DISABLE_AUTO_TUNE set; skipping."
    exit 0
fi

# Skip when not running as root or postgres. The shim writes conf files
# under a chmod-700 postgres-owned data dir; only those uids can chown
# the rendered file into postgres-readable ownership. Tests bypass via
# POSTGRES_AUTO_TUNE_SKIP_UID_CHECK=1.
if [ "${POSTGRES_AUTO_TUNE_SKIP_UID_CHECK:-0}" != "1" ]; then
    RUN_UID=$(id -u 2>/dev/null || echo "")
    PG_UID=$(id -u postgres 2>/dev/null || echo "")
    if [ "$RUN_UID" != "0" ] && [ "$RUN_UID" != "$PG_UID" ]; then
        log "running as uid $RUN_UID (not root, not postgres); skipping."
        exit 0
    fi
fi

DATA_DIR="${1:-${POSTGRES_DATA_DIR:-/data/postgres}}"
if [ ! -d "$DATA_DIR" ]; then
    log "data dir '$DATA_DIR' missing; skipping (postgres init has not run yet)."
    exit 0
fi

PG_CONF="${DATA_DIR}/postgresql.conf"

# If postgresql.conf already has any of the tuned parameters set
# uncommented, the operator has done their own tuning. Do not override.
if [ -f "$PG_CONF" ]; then
    for param in $TUNED_PARAMS; do
        if grep -qE "^[[:space:]]*${param}[[:space:]]*=" "$PG_CONF"; then
            log "operator-tuned ${param} found in postgresql.conf; skipping (operator wins)."
            exit 0
        fi
    done
fi

# If postgresql.conf already has any include_dir directive (active,
# commented, or pointing at a different directory), do not append a
# duplicate. Last-occurrence-wins on include_dir would silently steal
# the operator's directory.
if [ -f "$PG_CONF" ]; then
    if grep -qE "^[[:space:]]*#?[[:space:]]*include_dir[[:space:]]*=" "$PG_CONF"; then
        if grep -qE "^[[:space:]]*include_dir[[:space:]]*=[[:space:]]*'conf.d'" "$PG_CONF"; then
            INCLUDE_DIR_PRESENT=ours
        else
            INCLUDE_DIR_PRESENT=foreign
        fi
    else
        INCLUDE_DIR_PRESENT=none
    fi
else
    INCLUDE_DIR_PRESENT=none
fi

if [ "${INCLUDE_DIR_PRESENT:-none}" = "foreign" ]; then
    log "non-conf.d include_dir directive already in postgresql.conf; skipping (operator wins)."
    exit 0
fi

# Detect available memory in MB. Prefer cgroup limit (containerised env
# may be constrained below host RAM), fall back to /proc/meminfo, fall
# back to skip. Trim non-digit characters before numeric checks so a
# trailing CR or whitespace does not silently fall through to host RAM.
detect_mem_mb() {
    local cg_v2="/sys/fs/cgroup/memory.max"
    local cg_v1="/sys/fs/cgroup/memory/memory.limit_in_bytes"
    local val=""
    local raw=""

    if [ -r "$cg_v2" ]; then
        raw=$(cat "$cg_v2" 2>/dev/null)
        # Trim whitespace/CR for the literal "max" comparison.
        val="${raw//[[:space:]]/}"
        if [ "$val" != "max" ] && [ -n "$val" ]; then
            # Strip non-digits for the numeric compare. If the file held
            # garbage, this leaves an empty string which fails the check.
            val="${val//[!0-9]/}"
            if [ -n "$val" ] && [ "$val" -gt 0 ] 2>/dev/null && [ "$val" -lt "$CGROUP_BOGUS_LIMIT_BYTES" ]; then
                echo $((val / 1024 / 1024))
                return 0
            fi
        fi
    fi

    if [ -r "$cg_v1" ]; then
        raw=$(cat "$cg_v1" 2>/dev/null)
        val="${raw//[!0-9]/}"
        if [ -n "$val" ] && [ "$val" -gt 0 ] 2>/dev/null && [ "$val" -lt "$CGROUP_BOGUS_LIMIT_BYTES" ]; then
            echo $((val / 1024 / 1024))
            return 0
        fi
    fi

    if [ -r /proc/meminfo ]; then
        val=$(awk '/^MemTotal:/ {print int($2/1024); exit}' /proc/meminfo 2>/dev/null)
        if [ -n "$val" ] && [ "$val" -gt 0 ] 2>/dev/null; then
            echo "$val"
            return 0
        fi
    fi

    return 1
}

if [ -n "${POSTGRES_AUTO_TUNE_FORCE_MEM_MB:-}" ]; then
    MEM_MB="$POSTGRES_AUTO_TUNE_FORCE_MEM_MB"
else
    MEM_MB=$(detect_mem_mb) || {
        log "could not detect host memory; leaving stock defaults."
        exit 0
    }
fi

if [ "$MEM_MB" -lt 2048 ] 2>/dev/null; then
    log "host memory ${MEM_MB} MB below 2 GB tier floor; leaving stock defaults."
    exit 0
fi

# Tier selection. shared_buffers near 25% of RAM (capped to leave room
# for the audiusd Go process and OS cache). effective_cache_size at
# about 50% (more conservative than pgtune's 75%; postgres is not the
# only tenant in this container). wal_buffers capped at 16 MB per the
# Postgres docs (larger values are not useful). work_mem modest because
# audiusd's observed concurrency is well under max_connections.
if [ "$MEM_MB" -lt 4096 ]; then
    TIER="2-4GB"
    SHARED_BUFFERS="256MB"; WORK_MEM="4MB"; MAINTENANCE_WORK_MEM="128MB"
    EFFECTIVE_CACHE_SIZE="1GB"; WAL_BUFFERS="8MB"
    MAX_WAL_SIZE="1GB"; MIN_WAL_SIZE="256MB"
elif [ "$MEM_MB" -lt 8192 ]; then
    TIER="4-8GB"
    SHARED_BUFFERS="1GB"; WORK_MEM="8MB"; MAINTENANCE_WORK_MEM="256MB"
    EFFECTIVE_CACHE_SIZE="3GB"; WAL_BUFFERS="16MB"
    MAX_WAL_SIZE="2GB"; MIN_WAL_SIZE="512MB"
elif [ "$MEM_MB" -lt 16384 ]; then
    TIER="8-16GB"
    SHARED_BUFFERS="2GB"; WORK_MEM="16MB"; MAINTENANCE_WORK_MEM="512MB"
    EFFECTIVE_CACHE_SIZE="6GB"; WAL_BUFFERS="16MB"
    MAX_WAL_SIZE="2GB"; MIN_WAL_SIZE="1GB"
elif [ "$MEM_MB" -lt 32768 ]; then
    TIER="16-32GB"
    SHARED_BUFFERS="4GB"; WORK_MEM="32MB"; MAINTENANCE_WORK_MEM="1GB"
    EFFECTIVE_CACHE_SIZE="12GB"; WAL_BUFFERS="16MB"
    MAX_WAL_SIZE="4GB"; MIN_WAL_SIZE="1GB"
elif [ "$MEM_MB" -lt 65536 ]; then
    TIER="32-64GB"
    SHARED_BUFFERS="8GB"; WORK_MEM="32MB"; MAINTENANCE_WORK_MEM="2GB"
    EFFECTIVE_CACHE_SIZE="24GB"; WAL_BUFFERS="16MB"
    MAX_WAL_SIZE="8GB"; MIN_WAL_SIZE="2GB"
else
    TIER="64GB+"
    SHARED_BUFFERS="16GB"; WORK_MEM="32MB"; MAINTENANCE_WORK_MEM="2GB"
    EFFECTIVE_CACHE_SIZE="48GB"; WAL_BUFFERS="16MB"
    MAX_WAL_SIZE="16GB"; MIN_WAL_SIZE="2GB"
fi

CONF_D="${DATA_DIR}/conf.d"
TUNE_FILE="${CONF_D}/00-audiusd-defaults.conf"

# Use mktemp for the staging file (unique, less symlink-attack surface
# than $$). Falls back to a $$-suffixed name if mktemp is unavailable.
TMP_FILE=$(mktemp "${CONF_D}/.00-audiusd-defaults.conf.XXXXXX" 2>/dev/null || echo "${TUNE_FILE}.tmp.$$")

mkdir -p "$CONF_D" || {
    log "could not create $CONF_D; leaving stock defaults."
    exit 0
}
chown postgres:postgres "$CONF_D" 2>/dev/null || true

# Clean up any orphan temp files left by a prior interrupted run.
rm -f "${CONF_D}"/.00-audiusd-defaults.conf.* "${TUNE_FILE}".tmp.* 2>/dev/null || true

# Re-create TMP_FILE after cleanup (in case our mktemp output was
# matched by the cleanup glob).
TMP_FILE=$(mktemp "${CONF_D}/.00-audiusd-defaults.conf.XXXXXX" 2>/dev/null || echo "${TUNE_FILE}.tmp.$$")

# Render to a temp file then atomically rename. Avoids partial-write
# states if the container is killed mid-write.
{
    cat <<EOF
# Auto-generated by postgres-auto-tune.sh on container start.
# Do not edit; values are re-rendered every start.
# Detected host memory: ${MEM_MB} MB (tier ${TIER}).
#
# Precedence (later wins):
#   1. postgresql.conf top-of-file values
#   2. THIS FILE (conf.d/00-audiusd-defaults.conf)
#   3. conf.d/99-*.conf (operator override slot)
#   4. postgresql.auto.conf (ALTER SYSTEM, processed last by Postgres)
#
# To disable entirely: set AUDIUSD_DISABLE_AUTO_TUNE=1 in the container env.
# To override per setting: drop conf.d/99-yours.conf or use ALTER SYSTEM.

shared_buffers = ${SHARED_BUFFERS}
work_mem = ${WORK_MEM}
maintenance_work_mem = ${MAINTENANCE_WORK_MEM}
effective_cache_size = ${EFFECTIVE_CACHE_SIZE}
wal_buffers = ${WAL_BUFFERS}
max_wal_size = ${MAX_WAL_SIZE}
min_wal_size = ${MIN_WAL_SIZE}
EOF
} > "$TMP_FILE" || {
    log "could not write tune file; leaving stock defaults."
    rm -f "$TMP_FILE"
    exit 0
}

mv "$TMP_FILE" "$TUNE_FILE" || {
    log "could not move tune file into place; leaving stock defaults."
    rm -f "$TMP_FILE"
    exit 0
}
chown postgres:postgres "$TUNE_FILE" 2>/dev/null || true

# Ensure postgresql.conf includes conf.d. We only reach this branch
# when INCLUDE_DIR_PRESENT was 'none' (foreign was rejected above).
# Atomic write: build new postgresql.conf in a temp file and rename.
if [ "${INCLUDE_DIR_PRESENT}" = "none" ] && [ -f "$PG_CONF" ]; then
    PG_CONF_TMP=$(mktemp "${DATA_DIR}/.postgresql.conf.XXXXXX" 2>/dev/null || echo "${PG_CONF}.tmp.$$")
    {
        cat "$PG_CONF"
        echo ""
        echo "# Added by audiusd postgres-auto-tune; reads conf.d/*.conf in name order."
        echo "include_dir = 'conf.d'"
    } > "$PG_CONF_TMP" || {
        log "could not stage postgresql.conf update; tune file ready, include_dir not wired."
        rm -f "$PG_CONF_TMP" "$TUNE_FILE"
        exit 0
    }
    chown postgres:postgres "$PG_CONF_TMP" 2>/dev/null || true
    if ! mv "$PG_CONF_TMP" "$PG_CONF"; then
        log "could not install updated postgresql.conf; tune file ready, include_dir not wired."
        rm -f "$PG_CONF_TMP" "$TUNE_FILE"
        exit 0
    fi
fi

# Preflight: ask postgres to parse the conf and read back a tuned value.
# If postgres rejects the conf, remove the tune file rather than letting
# the next pg_ctl start fail.
if command -v su >/dev/null 2>&1 && [ -x /usr/lib/postgresql/15/bin/postgres ]; then
    if ! su - postgres -c "/usr/lib/postgresql/15/bin/postgres -D '${DATA_DIR}' -C shared_buffers" >/dev/null 2>&1; then
        log "postgres rejected the rendered conf; removing tune file and leaving stock defaults."
        rm -f "$TUNE_FILE"
        exit 0
    fi
fi

log "applied tier ${TIER} (host ${MEM_MB} MB): shared_buffers=${SHARED_BUFFERS} work_mem=${WORK_MEM} effective_cache_size=${EFFECTIVE_CACHE_SIZE} max_wal_size=${MAX_WAL_SIZE}"
exit 0
