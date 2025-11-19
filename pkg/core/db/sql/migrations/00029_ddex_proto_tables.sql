-- +migrate Up
create table
  if not exists core_ddex_ern_messages (
    id bigserial primary key,
    address text not null,
    thread_address text not null,
    owner text not null,
    message_id text not null,
    message_thread_id text not null,
    message bytea not null,
    tx_hash text not null,
    block_height bigint not null
  );

create index if not exists idx_core_ddex_ern_messages_address on core_ddex_ern_messages (address);

create index if not exists idx_core_ddex_ern_messages_thread_address on core_ddex_ern_messages (thread_address);

create index if not exists idx_core_ddex_ern_messages_owner on core_ddex_ern_messages (owner);

create index if not exists idx_core_ddex_ern_messages_message_id on core_ddex_ern_messages (message_id);

create index if not exists idx_core_ddex_ern_messages_message_thread_id on core_ddex_ern_messages (message_thread_id);

create index if not exists idx_core_ddex_ern_messages_tx_hash on core_ddex_ern_messages (tx_hash);

create index if not exists idx_core_ddex_ern_messages_block_height on core_ddex_ern_messages (block_height);

create index if not exists idx_core_ddex_ern_messages_owner_message on core_ddex_ern_messages (owner, message_id);

create index if not exists idx_core_ddex_ern_messages_owner_message_thread on core_ddex_ern_messages (owner, message_thread_id);

create table
  if not exists core_ddex_ern_events (
    id bigserial primary key,
    ern_id bigint not null references core_ddex_ern_messages (id),
    event bytea not null
  );

create index if not exists idx_core_ddex_ern_events_ern_id on core_ddex_ern_events (ern_id);

-- +migrate Down
drop index if exists idx_core_ddex_ern_events_ern_id;

drop table if exists core_ddex_ern_events;

drop index if exists idx_core_ddex_ern_messages_block_height;

drop index if exists idx_core_ddex_ern_messages_tx_hash;

drop index if exists idx_core_ddex_ern_messages_message_thread_id;

drop index if exists idx_core_ddex_ern_messages_message_id;

drop index if exists idx_core_ddex_ern_messages_owner_message_thread;

drop index if exists idx_core_ddex_ern_messages_owner_message;

drop index if exists idx_core_ddex_ern_messages_owner;

drop index if exists idx_core_ddex_ern_messages_thread_address;

drop index if exists idx_core_ddex_ern_messages_address;

drop table if exists core_ddex_ern_messages;
