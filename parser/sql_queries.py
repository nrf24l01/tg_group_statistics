SQL_CREATE = """
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tg_user_id BIGINT UNIQUE NOT NULL,
    username TEXT,
    nick TEXT
);

CREATE TABLE IF NOT EXISTS messages (
    uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    send_time TIMESTAMPTZ NOT NULL,
    sender_id UUID REFERENCES users(uuid) ON DELETE CASCADE,
    message_type TEXT NOT NULL,
    message_text TEXT,
    UNIQUE(chat_id, message_id)
);
 
CREATE TABLE IF NOT EXISTS groups (
    uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tg_group_id BIGINT UNIQUE NOT NULL,
    name TEXT
);
"""

SQL_UPSERT_USER = """
INSERT INTO users (tg_user_id, username, nick)
VALUES ($1, $2, $3)
ON CONFLICT (tg_user_id)
DO UPDATE SET username = EXCLUDED.username, nick = EXCLUDED.nick
RETURNING uuid;
"""

SQL_UPSERT_GROUP = """
INSERT INTO groups (tg_group_id, name)
VALUES ($1, $2)
ON CONFLICT (tg_group_id) DO UPDATE SET name = EXCLUDED.name
RETURNING uuid;
"""

SQL_INSERT_MESSAGE = """
INSERT INTO messages (chat_id, message_id, send_time, sender_id, message_type, message_text)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (chat_id, message_id) DO NOTHING;
"""