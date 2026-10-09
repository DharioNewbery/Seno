-- 0002_sso.sql — Consumo único do token de SSO (spec §3: jti não reutilizável
-- dentro da vida do st + revogação ao entrar).

BEGIN;

CREATE TABLE sso_consumo (
  token_hash  TEXT PRIMARY KEY,        -- SHA-256 (hex) do token `st`
  expira_em   TIMESTAMPTZ NOT NULL
);

CREATE INDEX sso_consumo_expira_idx ON sso_consumo (expira_em);

COMMIT;
