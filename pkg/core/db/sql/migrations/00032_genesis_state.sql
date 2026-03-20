-- +migrate Up
create table if not exists core_genesis_state (
    chain_id             text primary key,
    migration_address    text not null,
    migration_end_height bigint,
    snapshot_timestamp   timestamptz,
    entity_counts        jsonb not null default '{}',
    completed_at         timestamptz,
    created_at           timestamptz not null default now()
);

-- +migrate Down
drop table if exists core_genesis_state;
