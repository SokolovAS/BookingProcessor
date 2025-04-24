
SELECT run_command_on_workers('DROP TABLE IF EXISTS users CASCADE');
DROP TABLE IF EXISTS users CASCADE;

SELECT run_command_on_workers('DROP TABLE IF EXISTS hotels CASCADE');
DROP TABLE IF EXISTS hotels CASCADE;

BEGIN;

-- 1) Create users table
CREATE TABLE users (
                       id          BIGSERIAL,
                       name        TEXT        NOT NULL,
                       email       TEXT        NOT NULL,
                       created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 2) Configure & distribute users
SET citus.shard_replication_factor = 1;
SET citus.shard_count = 8;
SELECT create_distributed_table('users', 'id', shard_count := 8);

-- 3) Add PK & UNIQUE on users
ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (id, email);

-- 4) Create hotels table
CREATE TABLE hotels (
                        id          BIGSERIAL,
                        user_id     BIGINT      NOT NULL,
                        data        TEXT,
                        created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 5) Configure & distribute hotels
SET citus.shard_replication_factor = 1;
SET citus.shard_count = 8;
SELECT create_distributed_table(
               'hotels',
               'user_id',
               colocate_with := 'users'
       );

-- 6) Add PK & FK on hotels
ALTER TABLE hotels ADD CONSTRAINT hotels_pkey PRIMARY KEY (user_id, id);
ALTER TABLE hotels
    ADD CONSTRAINT hotels_user_fk
        FOREIGN KEY (user_id)
            REFERENCES users(id);

COMMIT;
