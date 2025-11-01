SQL_UPSERT_USER = """
INSERT INTO users (tg_user_id, username, nick)
VALUES ($1, $2, $3)
ON CONFLICT (tg_user_id)
DO UPDATE SET username = EXCLUDED.username, nick = EXCLUDED.nick
RETURNING id;
"""

SQL_UPSERT_GROUP = """
INSERT INTO groups (tg_group_id, name)
VALUES ($1, $2)
ON CONFLICT (tg_group_id) DO UPDATE SET name = EXCLUDED.name
RETURNING id;
"""

SQL_INSERT_MESSAGE = """
INSERT INTO messages (chat_id, message_id, send_time, sender_id, message_type, message_text)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (chat_id, message_id) DO NOTHING;
"""