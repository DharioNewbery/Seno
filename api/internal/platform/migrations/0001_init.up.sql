-- 0001_init.sql — Fundação do Seno: pessoas, usuários, cargos, sessões,
-- tokens de uso único, log e fila de jobs (ARQUITETURA §4.3, §5, §6, §9.3).

BEGIN;

CREATE TABLE persons (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    first_name  TEXT        NOT NULL CHECK (length(btrim(first_name)) > 0),
    last_name   TEXT        NOT NULL CHECK (length(btrim(last_name)) > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    person_id      BIGINT      NOT NULL REFERENCES persons (id) ON DELETE RESTRICT,
    email          TEXT        NOT NULL UNIQUE,
    password_hash  TEXT,            -- NULL = usuário pendente (convite)
    status         TEXT        NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('active', 'pending', 'disabled')),
    disabled_at    TIMESTAMPTZ,
    created_by     BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    updated_at     TIMESTAMPTZ,
    CHECK (status <> 'disabled' OR disabled_at IS NOT NULL)
);

CREATE INDEX users_email_lower_idx ON users (lower(email));
CREATE INDEX users_status_idx ON users (status);
CREATE INDEX users_person_idx ON users (person_id);

CREATE TABLE user_roles (
    user_id    BIGINT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       TEXT       NOT NULL CHECK (role IN ('student', 'professor', 'admin', 'super')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

CREATE INDEX user_roles_role_idx ON user_roles (role);

CREATE TABLE sessions (
    id                 BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id            BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash         TEXT        NOT NULL UNIQUE, -- SHA-256 (base64url) do token
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,            -- sliding window
    max_expires_at     TIMESTAMPTZ NOT NULL,            -- teto de vida total
    last_seen_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_ip         INET,
    user_agent         TEXT
);

CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE one_time_tokens (
    id             BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind           TEXT        NOT NULL CHECK (kind IN ('invite', 'password_reset')),
    token_hash     TEXT        NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    used_at        TIMESTAMPTZ,
    created_by     BIGINT      REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX one_time_tokens_user_kind_idx ON one_time_tokens (user_id, kind);

CREATE TABLE log_entries (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_id   BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    actor      TEXT        NOT NULL DEFAULT '', -- agente: e-mail/identificador
    kind       TEXT        NOT NULL,            -- login, logout, create, update...
    detail     TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX log_entries_occurred_at_idx ON log_entries (occurred_at DESC);
CREATE INDEX log_entries_kind_idx ON log_entries (kind);

CREATE TABLE jobs (
    id              BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind            TEXT        NOT NULL,
    payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status          TEXT        NOT NULL DEFAULT 'scheduled'
                    CHECK (status IN ('scheduled', 'running', 'done', 'failed')),
    priority        SMALLINT    NOT NULL DEFAULT 0,
    attempts        SMALLINT    NOT NULL DEFAULT 0,
    run_after       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX jobs_dequeue_idx
    ON jobs (priority DESC, id)
    WHERE status = 'scheduled';

CREATE FUNCTION touch_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_touch_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMIT;
