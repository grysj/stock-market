BEGIN;

-- Wallets are created on first buy/sell operation for a wallet_id.
CREATE TABLE IF NOT EXISTS wallets (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Bank inventory. Row existence means the stock exists in the system.
CREATE TABLE IF NOT EXISTS bank_stocks (
    stock_name TEXT PRIMARY KEY,
    quantity BIGINT NOT NULL CHECK (quantity >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Stocks currently held by each wallet.
CREATE TABLE IF NOT EXISTS wallet_stocks (
    wallet_id TEXT NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    stock_name TEXT NOT NULL REFERENCES bank_stocks(stock_name) ON DELETE RESTRICT,
    quantity BIGINT NOT NULL CHECK (quantity >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (wallet_id, stock_name)
);

-- Successful wallet operations only, ordered by occurrence.
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('buy', 'sell')),
    wallet_id TEXT NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    stock_name TEXT NOT NULL REFERENCES bank_stocks(stock_name) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common reads.
CREATE INDEX IF NOT EXISTS idx_wallet_stocks_stock_name
    ON wallet_stocks (stock_name);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at_id
    ON audit_log (created_at, id);

CREATE INDEX IF NOT EXISTS idx_audit_log_wallet_created_at_id
    ON audit_log (wallet_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_audit_log_stock_created_at_id
    ON audit_log (stock_name, created_at, id);

COMMIT;
