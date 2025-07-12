-- users table
CREATE TABLE IF NOT EXISTS users (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  name           TEXT    UNIQUE NOT NULL,
  hashed_password TEXT   NOT NULL
);

-- agents table
CREATE TABLE IF NOT EXISTS agents (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  name           TEXT    UNIQUE NOT NULL,
  last_seen      DATETIME NOT NULL,
  capacity       INTEGER NOT NULL,
  base_ssh_port  INTEGER NOT NULL DEFAULT 2222
);

-- rentals table
CREATE TABLE IF NOT EXISTS rentals (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  vm_name       TEXT    UNIQUE NOT NULL,
  user_id       INTEGER NOT NULL,
  agent_id      INTEGER NOT NULL,
  ssh_key    TEXT    NOT NULL,
  ip_address    TEXT,
  expires_at    DATETIME NOT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id),
  FOREIGN KEY(agent_id) REFERENCES agents(id)
);
CREATE INDEX IF NOT EXISTS idx_rentals_expires_at
  ON rentals(expires_at);

