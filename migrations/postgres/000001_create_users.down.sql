DROP INDEX IF EXISTS idx_users_user_id_unique;
DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

