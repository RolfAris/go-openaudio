# genesis-replay

Bootstraps a new Core chain with full historical Audius state by replaying
synthetic `ManageEntity` and `TrackPlay` transactions sourced from a
discovery-provider PostgreSQL database.

## How it works

The discovery provider indexes entity data from on-chain `ManageEntity`
transactions. Because all historical data lives in the DP's postgres DB rather
than on-chain, a new chain starts empty — `genesis-replay` bridges the gap by
re-submitting every entity as a genesis migration transaction signed by a
dedicated keypair. The DP treats transactions from `genesis_migration_address`
as trusted and indexes them without wallet ownership checks.

## Commands

### `keygen`

Generates a new Ethereum keypair for use as the genesis migration identity.

```
genesis-replay keygen
```

Prints the address and private key. Set the address as `genesis_migration_address`
in the chain's genesis JSON (`pkg/core/config/genesis/prod-v2.json`), then pass
the private key to `genesis-replay run`.

---

### `run`

Reads every current, non-deleted entity from the source DB and submits it to
the bootstrap chain as a `ManageEntity` transaction.

```
genesis-replay run \
  --src-dsn     <postgres_dsn>   \
  --chain-url   <bootstrap_url>  \
  --private-key <hex_privkey>    \
  [--network    prod|stage|dev]  \
  [--concurrency 500]            \
  [--batch-size  1000]           \
  [--skip-users] [--skip-tracks] [--skip-playlists] [--skip-social] [--skip-plays]
```

All flags can also be set via environment variables:

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--src-dsn` | `GENESIS_SRC_DSN` | — | Source PostgreSQL DSN |
| `--chain-url` | `GENESIS_CHAIN_URL` | `http://localhost:50051` | Bootstrap chain URL |
| `--private-key` | `GENESIS_MIGRATION_PRIVATE_KEY` | — | Genesis migration private key (hex) |
| `--network` | `NETWORK` | `prod` | EIP-712 signing domain (`prod`, `stage`, `dev`) |
| `--concurrency` | `GENESIS_CONCURRENCY` | `500` | Concurrent transaction submissions |
| `--batch-size` | `GENESIS_BATCH_SIZE` | `1000` | Rows fetched per DB query |
| `--skip-users` | `GENESIS_SKIP_USERS` | false | Skip user replay |
| `--skip-tracks` | `GENESIS_SKIP_TRACKS` | false | Skip track replay |
| `--skip-playlists` | `GENESIS_SKIP_PLAYLISTS` | false | Skip playlist replay |
| `--skip-social` | `GENESIS_SKIP_SOCIAL` | false | Skip follows/saves/reposts replay |
| `--skip-plays` | `GENESIS_SKIP_PLAYS` | false | Skip play replay |

Entities are replayed in dependency order: users → tracks → playlists → social → plays.

For maximum throughput, point `--chain-url` directly at the node's gRPC port (`http://host:50051`)
rather than the HTTPS ingress. This bypasses nginx and TLS, roughly tripling the submission rate.

---

### `verify`

Streams two discovery-provider databases in sorted order and performs a
merge comparison to detect missing, extra, or changed rows.

```
genesis-replay verify \
  --src <postgres_dsn>   \
  --dst <postgres_dsn>   \
  [--max-samples 10]     \
  [--skip-plays]
```

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--src` | `GENESIS_SRC_DSN` | — | Source (reference) database |
| `--dst` | `GENESIS_DEST_DSN` | — | Destination database |
| `--max-samples` | `GENESIS_MAX_SAMPLES` | `10` | Mismatch rows to print per entity type |
| `--skip-plays` | `GENESIS_SKIP_PLAYS` | false | Skip play-count verification |

Produces a summary table:

```
entity     src       dst       missing  extra  different  status
------     ---       ---       -------  -----  ---------  ------
users      1234567   1234567   0        0      0          OK
tracks     4567890   4567890   0        0      0          OK
plays      890123    890123    0        0      0          OK
```

**Exit codes**: `0` = all checks pass, `1` = mismatches found, `2` = fatal error.

Plays are compared as per-track aggregate counts (`GROUP BY play_item_id`) so the
command is safe to run against a full production dataset with billions of play rows.
Memory usage is O(1) — only two rows are held in memory at any time.

---

## Discovery provider patches

The DP must be rebuilt from source with the following env var set to relax
validation rules that are incompatible with historical data:

```
AUDIUS_GENESIS_MIGRATION_MODE=true
```

This disables, for `CREATE` actions only:
- Entity ID minimum offset checks (historical IDs are below the DP's offset thresholds)
- User wallet uniqueness checks (one signing key is used for all entities)
- Handle/name bad word filtering (historical handles may trip false positives)
- Bio/name character limit enforcement
- Signer ownership validation

From the `apps` repo root:

```bash
docker build -t audius-discovery-provider:latest \
  -f packages/discovery-provider/Dockerfile.prod \
  packages/discovery-provider
```

The docker-compose stack sets `AUDIUS_GENESIS_MIGRATION_MODE=true` automatically.

---

## Local development

The docker-compose stack in this directory provides everything needed to run the
integration test locally: a postgres instance pre-seeded with both the source
data and the discovery-provider schema, a local Core node, and a
discovery-provider service.

```bash
# Start the stack using Docker-managed volumes (default)
make up

# Start the stack with data on an external drive
EXT_DATA_DIR="/Volumes/T7 Shield" make up

# Run the integration test (stack must be running)
make test-integration

# Tear down (leaves external data intact if EXT_DATA_DIR was set)
make down

# Remove external data directories
EXT_DATA_DIR="/Volumes/T7 Shield" make clean
```

By default, `EXT_DATA_DIR` falls back to `.docker-data/` in the current directory.
Data directories created under `EXT_DATA_DIR`:

| Directory | Contents |
|-----------|----------|
| `test-validator/` | Core node chain data |
| `test-pg-data/` | PostgreSQL data |

### Stack requirements

- `openaudio/go-openaudio:dev` — build with `make docker-dev` from the repo root
- `audius-discovery-provider:latest` — build from `apps` repo with genesis mode patch (see above)

### Ports

| Service | Host port |
|---------|-----------|
| PostgreSQL | `5434` |
| Core node (gRPC) | `50051` |
| Ingress (HTTPS) | `443` |

### Monitoring DP errors

```bash
docker logs genesis-replay-discovery-provider-1 2>&1 \
  | grep -v "index_spl_token\|index_rewards_manager\|solders\|ParseError\|Traceback\|File \"/audius\|transactions_history\|AttributeError" \
  | grep -i "error\|critical" \
  | sed 's/.*"msg": "\([^"]*\)".*/\1/' \
  | sort | uniq -c | sort -rn \
  | head -20
```

---

## Known limitations

- `is_verified` (ETH-verified artist badge) is controlled by on-chain Ethereum
  verification and cannot be set via `ManageEntity`. Replayed users will have
  `is_verified = false` regardless of source data.
