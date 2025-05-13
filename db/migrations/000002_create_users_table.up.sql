CREATE TABLE IF NOT EXISTS users (
  id         UUID         PRIMARY KEY,
  name       VARCHAR(500) NOT NULL,
  email      VARCHAR(500) NOT NULL,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT users_email_key UNIQUE (email)
);
