CREATE TABLE IF NOT EXISTS users (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  email     TEXT    UNIQUE NOT NULL,
  password  TEXT    NOT NULL,          -- hashed
  ssh_key   TEXT    NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
