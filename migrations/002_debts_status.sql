-- 002_debts_status.sql

ALTER TABLE debts
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'open';

ALTER TABLE debts
    ADD COLUMN IF NOT EXISTS closed_at timestamptz;

-- useful indexes
CREATE INDEX IF NOT EXISTS idx_debts_creditor_status_due ON debts (creditor_id, status, due_date);
CREATE INDEX IF NOT EXISTS idx_debts_debtor_status_due   ON debts (debtor_id, status, due_date);
