-- Down Migration
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS directions;
DROP TABLE IF EXISTS tariffs;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS user_role;
