#!/bin/bash
# Tests for postgres-auto-tune.sh.
#
# Run from the repo root:
#   bash cmd/openaudio/postgres-auto-tune_test.sh
#
# Memory injection: the shim reads POSTGRES_AUTO_TUNE_FORCE_MEM_MB to
# bypass detect_mem_mb. Tests set this directly. detect_mem_mb's
# cgroup-v2 / cgroup-v1 / /proc/meminfo branches are exercised at
# runtime (they read absolute paths under /sys and /proc that cannot
# be mocked from a userspace shell test without privileged tooling).
# Production verification of detect_mem_mb is covered by running the
# shim inside a real container against real cgroup files.

set -u

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SHIM="${REPO_ROOT}/cmd/openaudio/postgres-auto-tune.sh"
PASS=0
FAIL=0

if [ ! -x "$SHIM" ]; then
    chmod +x "$SHIM" 2>/dev/null || true
fi

assert_value() {
    local conf="$1" key="$2" want="$3" testname="$4"
    local got
    got=$(grep "^${key}[[:space:]]*=" "$conf" 2>/dev/null | sed 's/^[^=]*=[[:space:]]*//' | tr -d '[:space:]')
    if [ "$got" = "$want" ]; then
        PASS=$((PASS + 1))
        echo "  PASS: ${testname}: ${key}=${got}"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: ${testname}: ${key}: want=${want} got=${got}"
    fi
}

assert_file_present() {
    local f="$1" testname="$2"
    if [ -f "$f" ]; then
        PASS=$((PASS + 1))
        echo "  PASS: ${testname}: file present"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: ${testname}: file missing: ${f}"
    fi
}

assert_file_absent() {
    local f="$1" testname="$2"
    if [ ! -f "$f" ]; then
        PASS=$((PASS + 1))
        echo "  PASS: ${testname}: file absent"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: ${testname}: file unexpectedly present: ${f}"
    fi
}

assert_log_contains() {
    local logfile="$1" pattern="$2" testname="$3"
    if grep -qF "$pattern" "$logfile" 2>/dev/null; then
        PASS=$((PASS + 1))
        echo "  PASS: ${testname}: log contains '${pattern}'"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: ${testname}: log does not contain '${pattern}'"
        echo "    log was:"
        sed 's/^/      /' "$logfile"
    fi
}

count_lines() {
    grep -cE "$2" "$1" 2>/dev/null || echo 0
}

SBX_BASE="$(mktemp -d -t pgtune-test.XXXXXX)"
trap 'rm -rf "$SBX_BASE"' EXIT

# Tests run as the developer's uid, not root or postgres. The shim's
# production uid check (root or postgres only) is bypassed for tests.
export POSTGRES_AUTO_TUNE_SKIP_UID_CHECK=1

# Run the shim with a forced memory value into a fresh sandbox dir.
# The shim's preflight (postgres -C) is skipped here because the test
# sandbox does not have a postgres binary. We test that path separately.
run_shim_with_mem_mb() {
    local mem_mb="$1"
    local sandbox="$2"
    local clean_sandbox="${3:-true}"

    if [ "$clean_sandbox" = "true" ]; then
        rm -rf "$sandbox"
    fi
    mkdir -p "$sandbox"
    if [ ! -f "$sandbox/postgresql.conf" ]; then
        echo "# stub postgresql.conf for tests" > "$sandbox/postgresql.conf"
    fi

    POSTGRES_AUTO_TUNE_FORCE_MEM_MB="$mem_mb" \
        bash "$SHIM" "$sandbox" >"$sandbox/.run.log" 2>&1
    return $?
}

run_test_tier() {
    local mem_mb="$1" tier_name="$2"
    local sb_want="$3" wm_want="$4" mwm_want="$5" ecs_want="$6" wb_want="$7" maxwal_want="$8" minwal_want="$9"

    local sbx="${SBX_BASE}/${tier_name}-${mem_mb}"
    echo "Test tier ${tier_name} at ${mem_mb} MB"
    run_shim_with_mem_mb "$mem_mb" "$sbx"

    local conf="${sbx}/conf.d/00-audiusd-defaults.conf"
    assert_file_present "$conf" "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "shared_buffers"        "$sb_want"      "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "work_mem"              "$wm_want"      "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "maintenance_work_mem"  "$mwm_want"     "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "effective_cache_size"  "$ecs_want"     "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "wal_buffers"           "$wb_want"      "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "max_wal_size"          "$maxwal_want"  "tier-${tier_name}-${mem_mb}"
    assert_value "$conf" "min_wal_size"          "$minwal_want"  "tier-${tier_name}-${mem_mb}"
}

# Mid-range tier checks
#                  mem_mb  tier_name      sb     wm   mwm    ecs   wb   maxwal minwal
run_test_tier      3000    "mid-2-4GB"   256MB   4MB  128MB  1GB   8MB  1GB    256MB
run_test_tier      6144    "mid-4-8GB"   1GB     8MB  256MB  3GB   16MB 2GB    512MB
run_test_tier      12288   "mid-8-16GB"  2GB    16MB  512MB  6GB   16MB 2GB    1GB
run_test_tier      24576   "mid-16-32GB" 4GB    32MB  1GB    12GB  16MB 4GB    1GB
run_test_tier      49152   "mid-32-64GB" 8GB    32MB  2GB    24GB  16MB 8GB    2GB
run_test_tier      131072  "mid-64GB+"   16GB   32MB  2GB    48GB  16MB 16GB   2GB

# Boundary checks: each tier's lower bound, and the value just below it.
# A fencepost regression (-le instead of -lt) would flip these.
run_test_tier      2048    "boundary-2-4GB-lo"   256MB   4MB  128MB  1GB   8MB  1GB    256MB
run_test_tier      4095    "boundary-2-4GB-hi"   256MB   4MB  128MB  1GB   8MB  1GB    256MB
run_test_tier      4096    "boundary-4-8GB-lo"   1GB     8MB  256MB  3GB   16MB 2GB    512MB
run_test_tier      8191    "boundary-4-8GB-hi"   1GB     8MB  256MB  3GB   16MB 2GB    512MB
run_test_tier      8192    "boundary-8-16GB-lo"  2GB    16MB  512MB  6GB   16MB 2GB    1GB
run_test_tier      16383   "boundary-8-16GB-hi"  2GB    16MB  512MB  6GB   16MB 2GB    1GB
run_test_tier      16384   "boundary-16-32GB-lo" 4GB    32MB  1GB    12GB  16MB 4GB    1GB
run_test_tier      32767   "boundary-16-32GB-hi" 4GB    32MB  1GB    12GB  16MB 4GB    1GB
run_test_tier      32768   "boundary-32-64GB-lo" 8GB    32MB  2GB    24GB  16MB 8GB    2GB
run_test_tier      65535   "boundary-32-64GB-hi" 8GB    32MB  2GB    24GB  16MB 8GB    2GB
run_test_tier      65536   "boundary-64GB+-lo"   16GB   32MB  2GB    48GB  16MB 16GB   2GB

# Below floor: under 2 GB should leave NO conf file.
echo "Test sub-tier-floor"
sbx="${SBX_BASE}/below-floor-2047"
run_shim_with_mem_mb 2047 "$sbx"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "below-floor-2047"
assert_log_contains "${sbx}/.run.log" "below 2 GB tier floor" "below-floor-log"

echo "Test sub-tier-floor at exactly 1024"
sbx="${SBX_BASE}/below-floor-1024"
run_shim_with_mem_mb 1024 "$sbx"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "below-floor-1024"

# Idempotency: include_dir line appears exactly once across re-runs.
echo "Test include_dir append idempotency"
sbx="${SBX_BASE}/idempotent"
run_shim_with_mem_mb 24576 "$sbx"
run_shim_with_mem_mb 24576 "$sbx" "false"
run_shim_with_mem_mb 24576 "$sbx" "false"
n=$(count_lines "${sbx}/postgresql.conf" "^include_dir = 'conf.d'")
if [ "$n" = "1" ]; then
    PASS=$((PASS + 1))
    echo "  PASS: idempotent: include_dir count = 1"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: idempotent: include_dir count = ${n} (want 1)"
fi

# Disable: AUDIUSD_DISABLE_AUTO_TUNE=1 short-circuits before any write.
echo "Test AUDIUSD_DISABLE_AUTO_TUNE=1"
sbx="${SBX_BASE}/disabled-1"
mkdir -p "$sbx"
echo "# stub" > "$sbx/postgresql.conf"
AUDIUSD_DISABLE_AUTO_TUNE=1 POSTGRES_AUTO_TUNE_FORCE_MEM_MB=24576 \
    bash "$SHIM" "$sbx" >"$sbx/.run.log" 2>&1
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "disabled-1"
assert_log_contains "${sbx}/.run.log" "AUDIUSD_DISABLE_AUTO_TUNE set" "disabled-1-log"

# Disable: any value other than "1" must NOT disable. The shim's
# canonical disable value is "1". An older draft accepted "true" too;
# we tightened that. This test pins the canonical form.
echo "Test AUDIUSD_DISABLE_AUTO_TUNE=true is NOT honored (=1 only)"
sbx="${SBX_BASE}/disabled-true"
mkdir -p "$sbx"
echo "# stub" > "$sbx/postgresql.conf"
AUDIUSD_DISABLE_AUTO_TUNE=true POSTGRES_AUTO_TUNE_FORCE_MEM_MB=24576 \
    bash "$SHIM" "$sbx" >"$sbx/.run.log" 2>&1
assert_file_present "${sbx}/conf.d/00-audiusd-defaults.conf" "disabled-true-rejected"

echo "Test AUDIUSD_DISABLE_AUTO_TUNE=0 does not disable"
sbx="${SBX_BASE}/disabled-0"
mkdir -p "$sbx"
echo "# stub" > "$sbx/postgresql.conf"
AUDIUSD_DISABLE_AUTO_TUNE=0 POSTGRES_AUTO_TUNE_FORCE_MEM_MB=24576 \
    bash "$SHIM" "$sbx" >"$sbx/.run.log" 2>&1
assert_file_present "${sbx}/conf.d/00-audiusd-defaults.conf" "disabled-0-rejected"

# Missing data dir: shim exits 0 cleanly.
echo "Test missing data dir"
if POSTGRES_AUTO_TUNE_FORCE_MEM_MB=24576 bash "$SHIM" "${SBX_BASE}/does-not-exist" >/dev/null 2>&1; then
    PASS=$((PASS + 1))
    echo "  PASS: missing-data-dir: exit 0"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: missing-data-dir: non-zero exit"
fi

# Operator override: 99-*.conf is preserved across re-runs.
echo "Test operator 99-*.conf preserved across re-runs"
sbx="${SBX_BASE}/operator-99-conf"
run_shim_with_mem_mb 24576 "$sbx"
echo "shared_buffers = 8GB # operator override" > "$sbx/conf.d/99-operator.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
if grep -q "8GB # operator override" "$sbx/conf.d/99-operator.conf" 2>/dev/null; then
    PASS=$((PASS + 1))
    echo "  PASS: operator-99-conf: preserved"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: operator-99-conf: modified or missing"
fi

# Operator-tuning detection: if postgresql.conf has any of the tuned
# params set, the shim must skip without writing conf.d.
echo "Test operator-tuned shared_buffers in postgresql.conf -> skip"
sbx="${SBX_BASE}/operator-tuned-sb"
mkdir -p "$sbx"
{
    echo "# operator postgresql.conf"
    echo "shared_buffers = 8GB"
} > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "operator-tuned-sb"
assert_log_contains "${sbx}/.run.log" "operator-tuned shared_buffers" "operator-tuned-sb-log"

echo "Test operator-tuned work_mem in postgresql.conf -> skip"
sbx="${SBX_BASE}/operator-tuned-wm"
mkdir -p "$sbx"
{
    echo "work_mem = 64MB"
} > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "operator-tuned-wm"

echo "Test commented-out tuning in postgresql.conf does NOT trigger skip"
sbx="${SBX_BASE}/commented-tuning"
mkdir -p "$sbx"
{
    echo "# shared_buffers = 8GB"
    echo "#work_mem = 64MB"
} > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_present "${sbx}/conf.d/00-audiusd-defaults.conf" "commented-tuning-not-skipped"

# Foreign include_dir detection: any include_dir that is not 'conf.d'
# (active or commented) makes the shim skip without appending.
echo "Test foreign include_dir (different directory) -> skip"
sbx="${SBX_BASE}/foreign-include"
mkdir -p "$sbx"
{
    echo "# operator postgresql.conf"
    echo "include_dir = 'my-conf.d'"
} > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "foreign-include-skip"
assert_log_contains "${sbx}/.run.log" "non-conf.d include_dir" "foreign-include-log"

echo "Test double-quoted include_dir = \"conf.d\" -> skip (different syntax, treat as foreign)"
sbx="${SBX_BASE}/dquoted-include"
mkdir -p "$sbx"
echo 'include_dir = "conf.d"' > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "dquoted-include-skip"

echo "Test commented include_dir -> skip (operator deliberately disabled)"
sbx="${SBX_BASE}/commented-include"
mkdir -p "$sbx"
echo "# include_dir = 'conf.d'" > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_absent "${sbx}/conf.d/00-audiusd-defaults.conf" "commented-include-skip"

echo "Test our own include_dir = 'conf.d' is recognized -> append skipped, conf still rendered on next run"
sbx="${SBX_BASE}/our-include-already-present"
mkdir -p "$sbx"
{
    echo "# stub"
    echo "include_dir = 'conf.d'"
} > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
assert_file_present "${sbx}/conf.d/00-audiusd-defaults.conf" "our-include-already-present"
n=$(count_lines "${sbx}/postgresql.conf" "^include_dir = 'conf.d'")
if [ "$n" = "1" ]; then
    PASS=$((PASS + 1))
    echo "  PASS: our-include-no-duplicate: include_dir count = 1"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: our-include-no-duplicate: include_dir count = ${n} (want 1)"
fi

# Atomic include_dir append: postgresql.conf must be well-formed after
# the shim runs (no torn writes, no partial directives). We verify by
# reading the file and confirming it ends with a complete include_dir
# directive (or has one somewhere, with no truncated lines).
echo "Test postgresql.conf is well-formed after include_dir append"
sbx="${SBX_BASE}/well-formed"
mkdir -p "$sbx"
{
    echo "# original conf line 1"
    echo "# original conf line 2"
} > "$sbx/postgresql.conf"
run_shim_with_mem_mb 24576 "$sbx" "false"
if grep -q "^include_dir = 'conf.d'$" "$sbx/postgresql.conf"; then
    PASS=$((PASS + 1))
    echo "  PASS: well-formed: include_dir line is complete"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: well-formed: include_dir line missing or malformed"
fi
if grep -q "^# original conf line 1$" "$sbx/postgresql.conf"; then
    PASS=$((PASS + 1))
    echo "  PASS: well-formed: original content preserved"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: well-formed: original content lost"
fi

# Orphan tmp cleanup: a stale temp file from a prior interrupted run
# must be cleaned up on next start.
echo "Test orphan tmp file cleanup"
sbx="${SBX_BASE}/orphan-cleanup"
mkdir -p "$sbx/conf.d"
echo "# stub" > "$sbx/postgresql.conf"
echo "stale" > "$sbx/conf.d/.00-audiusd-defaults.conf.STALE1"
echo "stale" > "$sbx/conf.d/00-audiusd-defaults.conf.tmp.99999"
run_shim_with_mem_mb 24576 "$sbx" "false"
if [ ! -f "$sbx/conf.d/.00-audiusd-defaults.conf.STALE1" ] && [ ! -f "$sbx/conf.d/00-audiusd-defaults.conf.tmp.99999" ]; then
    PASS=$((PASS + 1))
    echo "  PASS: orphan-cleanup: stale temp files removed"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: orphan-cleanup: stale temp files still present"
fi

# Confirm the shim's logged tier matches expectation for a known input.
echo "Test logged tier line"
sbx="${SBX_BASE}/log-line"
run_shim_with_mem_mb 24576 "$sbx"
assert_log_contains "${sbx}/.run.log" "applied tier 16-32GB" "log-line-tier"
assert_log_contains "${sbx}/.run.log" "shared_buffers=4GB" "log-line-sb"

echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"
[ "$FAIL" = "0" ] || exit 1
