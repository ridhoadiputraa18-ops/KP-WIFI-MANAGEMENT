-- ==========================================
-- KP WIFI MANAGEMENT
-- PostgreSQL schema for Neon
-- ==========================================

-- ==========================================
-- ROLES
-- ==========================================
CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO roles (name) VALUES
    ('ADMIN'),
    ('MEMBER'),
    ('TECHNICIAN')
ON CONFLICT (name) DO NOTHING;


-- ==========================================
-- USERS
-- ==========================================
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES roles(id),
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- MEMBERS
-- ==========================================
CREATE TABLE IF NOT EXISTS members (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    member_code TEXT NOT NULL UNIQUE,
    access_start TIMESTAMPTZ,
    access_end TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- GUESTS
-- ==========================================
CREATE TABLE IF NOT EXISTS guests (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    access_start TIMESTAMPTZ,
    access_end TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- DEVICES
-- ==========================================
CREATE TABLE IF NOT EXISTS devices (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    device_type TEXT NOT NULL,
    vendor TEXT,
    model TEXT,
    ip_address TEXT,
    mac_address TEXT,
    api_endpoint TEXT,
    status TEXT NOT NULL DEFAULT 'UNKNOWN',
    last_seen TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- SESSIONS
-- ==========================================
CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    guest_id BIGINT REFERENCES guests(id),
    device_id BIGINT REFERENCES devices(id),
    client_ip TEXT,
    client_mac TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'ONLINE',

    CHECK (
        (user_id IS NOT NULL AND guest_id IS NULL)
        OR
        (user_id IS NULL AND guest_id IS NOT NULL)
    )
);


-- ==========================================
-- USAGE LOGS
-- ==========================================
CREATE TABLE IF NOT EXISTS usage_logs (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    upload_bytes BIGINT NOT NULL DEFAULT 0,
    download_bytes BIGINT NOT NULL DEFAULT 0,
    total_bytes BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- WIFI ACCESS
-- ==========================================
CREATE TABLE IF NOT EXISTS wifi_access (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    guest_id BIGINT REFERENCES guests(id),
    ssid TEXT NOT NULL,
    access_type TEXT NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE',

    CHECK (
        (user_id IS NOT NULL AND guest_id IS NULL)
        OR
        (user_id IS NULL AND guest_id IS NOT NULL)
    )
);


-- ==========================================
-- WIFI CONFIGS
-- ==========================================
CREATE TABLE IF NOT EXISTS wifi_configs (
    id BIGSERIAL PRIMARY KEY,
    ssid TEXT NOT NULL UNIQUE,
    network_type TEXT NOT NULL,
    password_encrypted TEXT,
    guest_duration_minutes INTEGER NOT NULL DEFAULT 120,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- DEVICE METRICS
-- ==========================================
CREATE TABLE IF NOT EXISTS device_metrics (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    cpu_usage DOUBLE PRECISION,
    memory_usage DOUBLE PRECISION,
    uptime_seconds BIGINT,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- AUDIT LOGS
-- ==========================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    target_type TEXT,
    target_id BIGINT,
    description TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- AUTH SESSIONS
-- Dibuat oleh sistem autentikasi
-- ==========================================
CREATE TABLE IF NOT EXISTS auth_sessions (
    token TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ==========================================
-- INDEX
-- ==========================================
CREATE INDEX IF NOT EXISTS idx_users_role
    ON users(role_id);

CREATE INDEX IF NOT EXISTS idx_sessions_status
    ON sessions(status);

CREATE INDEX IF NOT EXISTS idx_sessions_user
    ON sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_sessions_guest
    ON sessions(guest_id);

CREATE INDEX IF NOT EXISTS idx_usage_session
    ON usage_logs(session_id);

CREATE INDEX IF NOT EXISTS idx_devices_status
    ON devices(status);

CREATE INDEX IF NOT EXISTS idx_audit_created
    ON audit_logs(created_at);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user
    ON auth_sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry
    ON auth_sessions(expires_at);
