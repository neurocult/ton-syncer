# TON Syncer

The goal of this project is to synchronize **TON** blockchain with database like **PostgreSQL**. There are 2 ways of using this project:

1. Via `cmd/syncer` as a service that will connect to existing postgresql database and TON blockchain via environment variables
2. As a library by using `syncer` (and `storage/postgres` if needed) package

No matter how you gonna use it you need to configure it first.

## Configuring

Here's the configuration object:

```go
type Config struct {
	WorkerPoolSize        int           `env:"WORKER_POOL_SIZE, default=1"`          // How many actualizers and updaters to spawn
	AccountsCheckInterval time.Duration `env:"ACCOUNTS_CHECK_INTERVAL, default=10s"` // How long one actualizer wait before new account lookup
	ActualizerStartDelay  time.Duration `env:"ACTUALIZER_START_DELAY, default=1s"`   // How much time to wait before spawn next actualizer in a pool
	AccountSyncInterval   time.Duration `env:"ACCOUNT_SYNC_INTERVAL, default=10m"`   // How frequently each account must be synced
	UpdaterLock           time.Duration `env:"UPDATER_LOCK_TIMEOUT, default=10s"`    // How much time updater have to process one account
	AssetID int `env:"UPDATER_ASSET_ID, default=0"`    // AssetID that updater will use when inserting new transactions into the storage
}
```

As you can see environment variables are used. This is how it works when using as a service. When using as a library you'll need to provide values by yourself. You can still use environment variables though, but you'll need to parse them by yourself.

For other configuration needed for using as a service see `config/config.go`

## Using as a service

Using as a service adds some specific constraints to your application. It assumes that you have available postgres connection to the database with the structure similar to this:

```sql
create table if not exists transactions
(
    id           serial primary key,
    account_id   int references accounts (id) ON DELETE CASCADE not null,
    category_id  int references categories (id)      not null,
    asset_id     int references assets (id)          not null,
    merchant     varchar(64),
    amount       decimal(20, 10)                     not null check (amount <> 0),
    comment      varchar(255),
    effective_at timestamp default current_timestamp not null,
    crypto_hash  varchar(64),
    crypto_ton_lt       numeric(20, 0) check (crypto_ton_lt >= 0)
);

create table if not exists accounts
(
    id              serial primary key,
    crypto_address         varchar(64),
    crypto_start_sync_time timestamp,
    crypto_end_sync_time   timestamp,
);

create table if not exists gue_jobs
(
    job_id      text        not null primary key,
    priority    smallint    not null,
    run_at      timestamptz not null,
    job_type    text        not null,
    args        bytea       not null,
    error_count integer     not null default 0,
    last_error  text,
    queue       text        not null,
    created_at  timestamptz not null,
    updated_at  timestamptz not null
);

create index if not exists idx_gue_jobs_selector on gue_jobs (queue, run_at, priority);
```

`accounts` and `transactions` tables are used to store data and `gue` table is used to implement concurrent que-based worker algorithm.

## Using as a library

Using as a library allows you to use any database you want (event though you're allowed to use postgres and even use `storage/postgres` adapter from this repo) with any structure you like. All you need is to implement `syncer.Storage` interface and instantiate your `syncer.Syncer` object. After that you'll be able to call `Syncer.Sync()` method to launch the synchronization process. You can refer to `cmd/syncer` as an example.
