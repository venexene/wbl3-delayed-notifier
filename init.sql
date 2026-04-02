CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    target TEXT,
    message TEXT,
    send_at TIMESTAMP,
    status TEXT,
    retry INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);