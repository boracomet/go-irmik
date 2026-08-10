-- RBAC tables (SQLite + Postgres friendly).
-- user_id is TEXT to stay auth-agnostic (UUID, email, numeric string, …).

CREATE TABLE IF NOT EXISTS roles (
  name TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS permissions (
  name TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_name TEXT NOT NULL REFERENCES roles (name) ON DELETE CASCADE,
  permission_name TEXT NOT NULL REFERENCES permissions (name) ON DELETE CASCADE,
  PRIMARY KEY (role_name, permission_name)
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id TEXT NOT NULL,
  role_name TEXT NOT NULL REFERENCES roles (name) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_name)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles (user_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions (role_name);
