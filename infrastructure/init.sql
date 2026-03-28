-- Standard PostgreSQL Schema Initialization

-- Core users
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    role VARCHAR(50) DEFAULT 'trader',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Exchange connections
CREATE TABLE IF NOT EXISTS exchange_accounts (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    exchange VARCHAR(50) NOT NULL,
    api_key VARCHAR(255) NOT NULL,
    api_secret_encrypted VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Strategy Config
CREATE TABLE IF NOT EXISTS strategies (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    parameters JSONB NOT NULL,
    status VARCHAR(50) DEFAULT 'DRAFT',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Orders Ledger
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    exchange_id UUID REFERENCES exchange_accounts(id),
    strategy_id UUID REFERENCES strategies(id) ON DELETE SET NULL,
    symbol VARCHAR(50) NOT NULL,
    side VARCHAR(10) NOT NULL,
    order_type VARCHAR(20) NOT NULL,
    price NUMERIC,
    quantity NUMERIC NOT NULL,
    status VARCHAR(50) NOT NULL,
    filled_quantity NUMERIC DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Active Positions
CREATE TABLE IF NOT EXISTS positions (
    id UUID PRIMARY KEY,
    strategy_id UUID REFERENCES strategies(id) ON DELETE CASCADE,
    symbol VARCHAR(50) NOT NULL,
    side VARCHAR(10) NOT NULL,
    entry_price NUMERIC NOT NULL,
    quantity NUMERIC NOT NULL,
    leverage INT DEFAULT 1,
    unrealized_pnl NUMERIC DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Time-Series Market Ticks
CREATE TABLE IF NOT EXISTS market_ticks (
    time TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(50) NOT NULL,
    price NUMERIC NOT NULL,
    quantity NUMERIC NOT NULL,
    side VARCHAR(10) NOT NULL,
    trade_id BIGINT NOT NULL
);

-- Optimized Indexing for Standard PostgreSQL
CREATE INDEX IF NOT EXISTS idx_market_ticks_time ON market_ticks(time DESC);
CREATE INDEX IF NOT EXISTS idx_market_ticks_symbol ON market_ticks(symbol);
