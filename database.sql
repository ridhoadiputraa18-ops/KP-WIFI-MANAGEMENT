PRAGMA foreign_keys = ON;

-- ==========================================
-- ROLES
-- ==========================================
CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

INSERT OR IGNORE INTO roles (name) VALUES
('ADMIN'),
('MEMBER'),
('TECHNICIAN');

-- ==========================================
-- USERS
-- Semua akun yang dapat login ke aplikasi
-- ==========================================
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role_id INTEGER NOT NULL,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (role_id) REFERENCES roles(id)
);

-- ==========================================
-- MEMBERS
-- Data khusus member
-- ==========================================
CREATE TABLE IF NOT EXISTS members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    member_code TEXT NOT NULL UNIQUE,
    access_start DATETIME,
    access_end DATETIME,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ==========================================
-- WIFI ACCESS
-- Aturan akses Wi-Fi setiap member/guest
-- ==========================================
CREATE TABLE IF NOT EXISTS wifi_access (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    guest_id INTEGER,
    ssid TEXT NOT NULL,
    access_type TEXT NOT NULL,
    start_at DATETIME NOT NULL,
    end_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CHECK (
        (user_id IS NOT NULL AND guest_id IS NULL)
        OR
        (user_id IS NULL AND guest_id IS NOT NULL)
    )
);

-- ==========================================
-- GUESTS
-- Guest tidak perlu membuat akun member
-- ==========================================
CREATE TABLE IF NOT EXISTS guests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    access_start DATETIME,
    access_end DATETIME,
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ==========================================
-- SESSIONS
-- Sesi koneksi Wi-Fi
-- ==========================================
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    guest_id INTEGER,
    device_id INTEGER,
    client_ip TEXT,
    client_mac TEXT,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME,
    status TEXT NOT NULL DEFAULT 'ONLINE',

    CHECK (
        (user_id IS NOT NULL AND guest_id IS NULL)
        OR
        (user_id IS NULL AND guest_id IS NOT NULL)
    )
);

-- ==========================================
-- USAGE LOGS
-- Data penggunaan jaringan
-- ==========================================
CREATE TABLE IF NOT EXISTS usage_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    upload_bytes INTEGER NOT NULL DEFAULT 0,
    download_bytes INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- ==========================================
-- DEVICES
-- Router / AP / Switch
-- ==========================================
CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    device_type TEXT NOT NULL,
    vendor TEXT,
    model TEXT,
    ip_address TEXT,
    mac_address TEXT,
    api_endpoint TEXT,
    status TEXT NOT NULL DEFAULT 'UNKNOWN',
    last_seen DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ==========================================
-- DEVICE METRICS
-- Monitoring perangkat jaringan
-- ==========================================
CREATE TABLE IF NOT EXISTS device_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    cpu_usage REAL,
    memory_usage REAL,
    uptime_seconds INTEGER,
    recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- ==========================================
-- AUDIT LOGS
-- Aktivitas Admin / Teknisi / sistem
-- ==========================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action TEXT NOT NULL,
    target_type TEXT,
    target_id INTEGER,
    description TEXT,
    ip_address TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- ==========================================
-- WIFI CONFIGURATION
-- Konfigurasi jaringan Guest/Member
-- ==========================================
CREATE TABLE IF NOT EXISTS wifi_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ssid TEXT NOT NULL UNIQUE,
    network_type TEXT NOT NULL,
    password_encrypted TEXT,
    guest_duration_minutes INTEGER DEFAULT 120,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ==========================================
-- INDEX
-- Mempercepat pencarian data
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
