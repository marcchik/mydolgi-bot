BEGIN;

CREATE TABLE IF NOT EXISTS users (
                                     id            BIGSERIAL PRIMARY KEY,
                                     telegram_id   BIGINT UNIQUE NOT NULL,
                                     username      TEXT,
                                     first_name    TEXT,
                                     last_name     TEXT,
                                     created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE TABLE IF NOT EXISTS contacts (
                                        id              BIGSERIAL PRIMARY KEY,
                                        owner_user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(owner_user_id, contact_user_id)
    );

CREATE TABLE IF NOT EXISTS contact_aliases (
                                               id          BIGSERIAL PRIMARY KEY,
                                               owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alias       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(owner_user_id, contact_user_id, alias)
    );

CREATE TABLE IF NOT EXISTS debts (
                                     id           BIGSERIAL PRIMARY KEY,
                                     creditor_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    debtor_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    currency     TEXT NOT NULL,
    due_date     DATE NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active', -- active/paid/overdue
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE INDEX IF NOT EXISTS idx_debts_due_date_status ON debts(due_date, status);
CREATE INDEX IF NOT EXISTS idx_alias_search ON contact_aliases(owner_user_id, alias);

COMMIT;
