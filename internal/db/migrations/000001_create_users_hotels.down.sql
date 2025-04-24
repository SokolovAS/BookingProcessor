BEGIN;

-- Undo hotels first
SELECT drop_distributed_table('hotels');
DROP TABLE IF EXISTS hotels CASCADE;

-- Then users
SELECT drop_distributed_table('users');
DROP TABLE IF EXISTS users CASCADE;

COMMIT;
