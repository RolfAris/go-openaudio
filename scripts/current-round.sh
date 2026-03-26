#!/usr/bin/env bash
# current-round.sh — show the current in-progress consensus round
#
# Usage:
#   ./scripts/current-round.sh [exec_cmd]
#
# exec_cmd:  A command prefix used to execute shell commands on the node.
#            Defaults to local execution (no prefix).
#
# Examples:
#   # Run against a local node
#   ./scripts/current-round.sh
#
#   # Run against a Kubernetes pod
#   ./scripts/current-round.sh "kubectl exec -n creator-1 audiusd-0 --"
#
#   # Run against a Docker container
#   ./scripts/current-round.sh "docker exec audiusd"
#
#   # Run against a remote host via SSH
#   ./scripts/current-round.sh "ssh user@node1.example.com"
#
# The exec_cmd is prepended to: sh -c '<command>'
# It must support running arbitrary shell commands on the target node.

set -euo pipefail

EXEC_CMD=${1:-}

run() {
  if [ -n "$EXEC_CMD" ]; then
    $EXEC_CMD sh -c "$1"
  else
    sh -c "$1"
  fi
}

echo "Fetching data..." >&2

VAL_MAP=$(run 'psql -U postgres audius_creator_node -t -c "SELECT LEFT(comet_address,12), endpoint FROM core_validators ORDER BY comet_address"')

EXPECTED_ROLLUP=$(run 'psql -U postgres audius_creator_node -t -c "
SELECT cv.comet_address, COALESCE(r.blocks_proposed, 0)
FROM core_validators cv
LEFT JOIN sla_node_reports r ON r.address = cv.comet_address AND r.sla_rollup_id IS NULL
WHERE coalesce(cv.jailed, false) = false
ORDER BY cv.comet_address
"')

LAST_ROLLUP=$(run 'psql -U postgres audius_creator_node -t -c "SELECT block_end FROM sla_rollups ORDER BY id DESC LIMIT 1"')

_CS_TMP=$(mktemp)
run 'curl -s --unix-socket /tmp/cometbft.rpc.sock "http://localhost/dump_consensus_state"' > "$_CS_TMP"

python3 - "$_CS_TMP" <<PYEOF
import json, sys, re, struct, base64, datetime

cs_tmpfile = sys.argv[1]

with open(cs_tmpfile) as f:
    cs = json.load(f)

val_map_raw     = '''$VAL_MAP'''
exp_rollup_raw  = '''$EXPECTED_ROLLUP'''
last_rollup_raw = '''$LAST_ROLLUP'''

# ── protobuf helpers ──────────────────────────────────────────────────────────

def decode_varint(data, pos):
    result, shift = 0, 0
    while pos < len(data):
        b = data[pos]; pos += 1
        result |= (b & 0x7F) << shift
        if not (b & 0x80):
            return result, pos
        shift += 7
    raise ValueError("truncated varint")

def decode_fields(data):
    pos = 0
    while pos < len(data):
        try:
            tag, pos = decode_varint(data, pos)
        except:
            break
        fn, wt = tag >> 3, tag & 7
        try:
            if wt == 0:
                v, pos = decode_varint(data, pos)
            elif wt == 1:
                v = struct.unpack_from("<Q", data, pos)[0]; pos += 8
            elif wt == 2:
                n, pos = decode_varint(data, pos)
                v = bytes(data[pos:pos+n]); pos += n
            elif wt == 5:
                v = struct.unpack_from("<I", data, pos)[0]; pos += 4
            else:
                break
        except:
            break
        yield fn, wt, v

def get_field(data, fn, wt=None):
    for f, w, v in decode_fields(data):
        if f == fn and (wt is None or w == wt):
            return v
    return None

def get_fields(data, fn, wt=None):
    return [v for f, w, v in decode_fields(data) if f == fn and (wt is None or w == wt)]

def decode_timestamp(ts_bytes):
    if not ts_bytes:
        return "?", 0, 0
    secs  = get_field(ts_bytes, 1, 0) or 0
    nanos = get_field(ts_bytes, 2, 0) or 0
    s = datetime.datetime.utcfromtimestamp(secs).strftime("%Y-%m-%dT%H:%M:%S")
    return f"{s}.{nanos:09d}Z", secs, nanos

def decode_sla_rollup(rb):
    ts_b = get_field(rb, 1, 2)
    block_start = get_field(rb, 2, 0) or 0
    block_end   = get_field(rb, 3, 0) or 0
    ts_str, ts_secs, ts_nanos = decode_timestamp(ts_b)
    reports = []
    for r in get_fields(rb, 4, 2):
        addr     = (get_field(r, 1, 2) or b"").decode("utf-8", errors="replace")
        proposed = get_field(r, 2, 0) or 0
        reports.append((addr, proposed))
    return {"timestamp": ts_str, "ts_secs": ts_secs, "ts_nanos": ts_nanos,
            "block_start": block_start, "block_end": block_end, "reports": reports}

TX_TYPES = {
    1000: "TrackPlays", 1002: "SlaRollup", 1003: "ManageEntity",
    1007: "Attestation",
}

def decode_tx(tx_b64):
    try:
        raw = base64.b64decode(tx_b64)
        for fn, wt, v in decode_fields(raw):
            if fn in TX_TYPES and wt == 2:
                name = TX_TYPES[fn]
                if name == "SlaRollup":
                    return name, decode_sla_rollup(v)
                return name, {}
        return "Other", {}
    except:
        return "DecodeFailed", {}

# ── parse DB data ─────────────────────────────────────────────────────────────

def parse_psql_rows(raw):
    rows = []
    for line in raw.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        parts = [p.strip() for p in line.split("|")]
        rows.append(parts)
    return rows

val_map = {}
for row in parse_psql_rows(val_map_raw):
    if len(row) == 2:
        val_map[row[0].upper()] = row[1]

exp_reports = {}
for row in parse_psql_rows(exp_rollup_raw):
    if len(row) == 2:
        try:
            exp_reports[row[0]] = int(row[1])
        except:
            pass

last_block_end = 0
rows = parse_psql_rows(last_rollup_raw)
if rows and rows[0]:
    try:
        last_block_end = int(rows[0][0])
    except:
        pass

# ── parse consensus state ─────────────────────────────────────────────────────

rs          = cs.get("result", {}).get("round_state", {})
height      = int(rs.get("height") or 0) or None
cur_round   = rs.get("round", 0)
step_str    = rs.get("step", "unknown")
validators  = rs.get("validators", {}).get("validators", [])
votes       = rs.get("votes", [])
val_addrs   = [v["address"][:12].upper() for v in validators]

# proposer for current round
proposer    = rs.get("validators", {}).get("proposer", {})
proposer_addr = (proposer.get("address") or "")[:12].upper()

# proposal_block is from the CURRENT round — use it for rollup diff
proposal_block = rs.get("proposal_block")
block_time_str = block_time_secs = block_time_nanos = None
rollup_in_block = None
if proposal_block:
    block_time_str = (proposal_block.get("header") or {}).get("time")
    if block_time_str:
        try:
            m = re.match(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})\.?(\d*)Z?$", block_time_str.rstrip("Z"))
            if m:
                dt = datetime.datetime.strptime(m.group(1), "%Y-%m-%dT%H:%M:%S")
                block_time_secs = int(dt.replace(tzinfo=datetime.timezone.utc).timestamp())
                frac = m.group(2).ljust(9, "0")[:9]
                block_time_nanos = int(frac)
        except:
            pass
    txs = (proposal_block.get("data") or {}).get("txs") or []
    for tx_b64 in txs:
        name, details = decode_tx(tx_b64)
        if name == "SlaRollup":
            rollup_in_block = details
            break

# ── find current round's vote data ───────────────────────────────────────────

def get_bh(v):
    m = re.search(r"SIGNED_MSG_TYPE_\w+\(\w+\)\s+(\S+)", v)
    return m.group(1) if m else None

def tally(round_votes):
    pv = round_votes.get("prevotes", [])
    for_list = nil_list = abs_list = []
    for_list  = [i for i, v in enumerate(pv)
                 if "nil-Vote" not in v and v.startswith("Vote") and get_bh(v) not in (None, "000000000000")]
    nil_list  = [i for i, v in enumerate(pv)
                 if "nil-Vote" not in v and v.startswith("Vote") and get_bh(v) in (None, "000000000000")]
    abs_list  = [i for i, v in enumerate(pv) if "nil-Vote" in v]
    return for_list, nil_list, abs_list

# Find the current round's vote data
cur_data = None
for r in votes:
    rnum = r.get("round", -1)
    if rnum == cur_round:
        cur_data = r
        break

# ── formatting ─────────────────────────────────────────────────────────────────

CYAN   = "\033[96m"
GREEN  = "\033[92m"
YELLOW = "\033[93m"
RED    = "\033[91m"
BOLD   = "\033[1m"
DIM    = "\033[2m"
RESET  = "\033[0m"

DEAD_ADDRS = {
    "12ABEAFF9086","1375316FB255","13BCF9B4C1DF","32B088725B4B","34DF66133AB8",
    "3714F1A5753D","3BD77CDE0AA1","4A1E3FD9A5A1","5CBA61B158B3","99C6AD576AEB",
    "9CB838815C74","A4747BF6BB60","A5B56BBFA35E","D87A6B4F65D6","E3AA5F4A8814",
}

def hdr(s):
    print(f"\n{BOLD}{CYAN}{'─'*64}{RESET}")
    print(f"{BOLD}{CYAN}  {s}{RESET}")
    print(f"{BOLD}{CYAN}{'─'*64}{RESET}")

def ep(short):
    short = short.upper()
    url = val_map.get(short, "")
    return url.replace("https://","").replace("http://","") or short

def node_line(i, prefix="  "):
    if i >= len(validators):
        return f"{prefix}[{i:2d}] ?"
    a = validators[i]["address"][:12].upper()
    dead_tag = f" {DIM}[offline]{RESET}" if a in DEAD_ADDRS else ""
    return f"{prefix}[{i:2d}] {a}  {ep(a)}{dead_tag}"

# ── 1. overview ────────────────────────────────────────────────────────────────

hdr(f"CURRENT ROUND (in-progress)")
print(f"  height       : {height}")
print(f"  round        : {cur_round}")
print(f"  step         : {step_str}")
print(f"  proposer     : {proposer_addr}  {ep(proposer_addr)}")
print(f"  has proposal : {'YES' if proposal_block else 'NO'}")

if cur_data is None:
    print(f"\n  {YELLOW}No vote data found for current round {cur_round}{RESET}")
    sys.exit(0)

# ── 2. vote breakdown ──────────────────────────────────────────────────────────

hdr(f"VOTES  (round {cur_round} — in progress)")
for_l, nil_l, abs_l = tally(cur_data)
total  = len(validators)
quorum = total * 2 // 3 + 1
gap    = quorum - len(for_l)
gap_color = GREEN if gap <= 0 else (YELLOW if gap <= 2 else RED)

print(f"  validators: {total}   quorum: {quorum}   "
      f"gap: {gap_color}{'+' if gap<=0 else ''}{-gap if gap<=0 else gap} "
      f"{'(QUORUM REACHED)' if gap<=0 else 'votes short'}{RESET}\n")

if for_l:
    print(f"  {GREEN}Prevoted FOR block ({len(for_l)}):{RESET}")
    for i in for_l: print(node_line(i))

if nil_l:
    print(f"\n  {RED}Prevoted NIL — rejecting block ({len(nil_l)}):{RESET}")
    for i in nil_l: print(node_line(i))

if abs_l:
    print(f"\n  {YELLOW}Absent — no prevote received ({len(abs_l)}):{RESET}")
    for i in abs_l: print(node_line(i))

# precommits
pcs = cur_data.get("precommits", [])
for_pc = [i for i, v in enumerate(pcs)
          if "nil-Vote" not in v and v.startswith("Vote") and get_bh(v) not in (None, "000000000000")]
if for_pc:
    print(f"\n  {GREEN}Precommitted FOR ({len(for_pc)}):{RESET}")
    for i in for_pc: print(node_line(i))
else:
    nil_pc = [i for i, v in enumerate(pcs)
              if "nil-Vote" not in v and v.startswith("Vote")]
    if nil_pc:
        print(f"\n  {RED}Precommitted NIL ({len(nil_pc)}){RESET}  (no block quorum in prevote)")
        for i in nil_pc: print(node_line(i))
    abs_pc = [i for i, v in enumerate(pcs) if "nil-Vote" in v]
    if abs_pc:
        print(f"\n  {YELLOW}No precommit received ({len(abs_pc)}):{RESET}")
        for i in abs_pc: print(node_line(i))

# ── 3. recent round summary ────────────────────────────────────────────────────

hdr("RECENT ROUND HISTORY  (last 25 rounds)")
print(f"  {'Round':>6}  {'FOR':>4}  {'NIL':>4}  {'abs':>4}  note")
print(f"  {'-'*40}")
for r in votes:
    rnum = r.get("round", -1)
    if rnum < cur_round - 24:
        continue
    fl, nl, al = tally(r)
    gap2 = quorum - len(fl)
    if rnum == cur_round:
        note = f"{CYAN}IN PROGRESS{RESET}"
    elif gap2 <= 0:
        note = f"{GREEN}QUORUM{RESET}"
    elif len(fl) > 0:
        note = f"gap={gap2}"
    else:
        note = f"{DIM}no proposal{RESET}"
    marker = "←" if rnum == cur_round else " "
    print(f"  {rnum:>6}{marker} {len(fl):>4}  {len(nl):>4}  {len(al):>4}  {note}")

# ── 4. rollup diff (uses current round's proposal) ────────────────────────────

if rollup_in_block:
    hdr(f"ROLLUP DIFF  (from current round {cur_round} proposal)")

    SLA_ROLLUP_INTERVAL = 2048
    p_start = rollup_in_block["block_start"]
    p_end   = rollup_in_block["block_end"]
    e_start = last_block_end + 1
    should_propose = (height - last_block_end) >= SLA_ROLLUP_INTERVAL if height else False

    print(f"  shouldProposeNewRollup: {height} - {last_block_end} = "
          f"{height - last_block_end if height else '?'}  >= {SLA_ROLLUP_INTERVAL}?  "
          f"{'YES' if should_propose else RED + 'NO' + RESET}")

    print(f"\n  Proposed : block_start={p_start}  block_end={p_end}  ts={rollup_in_block['timestamp']}")
    print(f"  Expected : block_start={e_start}  block_end={height-1 if height else '?'}  ts={block_time_str}")

    any_mismatch = False

    if not should_propose:
        print(f"  {RED}✗ REJECT: shouldProposeNewRollup=false{RESET}")
        any_mismatch = True

    if block_time_secs is not None:
        if rollup_in_block["ts_secs"] != block_time_secs or rollup_in_block["ts_nanos"] != block_time_nanos:
            print(f"  {RED}✗ REJECT: timestamp mismatch{RESET}")
            print(f"      rollup tx : secs={rollup_in_block['ts_secs']}  nanos={rollup_in_block['ts_nanos']}")
            print(f"      block hdr : secs={block_time_secs}  nanos={block_time_nanos}")
            any_mismatch = True

    if p_start != e_start:
        print(f"  {RED}✗ REJECT: block_start mismatch: proposed {p_start} vs expected {e_start}{RESET}")
        any_mismatch = True

    if height and p_end != height - 1:
        print(f"  {RED}✗ REJECT: block_end mismatch: proposed {p_end} vs expected {height-1}{RESET}")
        any_mismatch = True

    proposed_map = {a: n for a, n in rollup_in_block["reports"]}
    expected_map = exp_reports

    if len(proposed_map) != len(expected_map):
        print(f"  {RED}✗ REJECT: report count mismatch: proposed {len(proposed_map)} vs expected {len(expected_map)}{RESET}")
        any_mismatch = True

    all_addrs = sorted(set(list(proposed_map) + list(expected_map)))
    diffs = [(a, proposed_map.get(a, 0), expected_map.get(a, 0))
             for a in all_addrs if proposed_map.get(a, 0) != expected_map.get(a, 0)]

    if diffs:
        print(f"\n  {RED}✗ REJECT: {len(diffs)} block-count mismatch(es):{RESET}")
        print(f"\n  {'Address':<44}  {'Proposed':>9}  {'Expected':>9}  {'Δ':>5}  Endpoint")
        print(f"  {'-'*44}  {'-'*9}  {'-'*9}  {'-'*5}  {'-'*30}")
        for addr, prop_n, exp_n in diffs:
            short  = addr[:12].upper()
            ep_url = val_map.get(short, "").replace("https://","").replace("http://","")[:35]
            diff   = prop_n - exp_n
            print(f"  {addr:<44}  {prop_n:>9}  {exp_n:>9}  {diff:>+5}  {ep_url}")
        any_mismatch = True

    if not any_mismatch:
        print(f"\n  {GREEN}✓ ACCEPT: all {len(all_addrs)} checks pass — rollup valid per this node{RESET}")
else:
    hdr("ROLLUP DIFF")
    print(f"  {YELLOW}No SlaRollup in current proposal block{RESET}")

print()
PYEOF

rm -f "$_CS_TMP"
