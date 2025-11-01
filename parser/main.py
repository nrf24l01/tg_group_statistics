import asyncio
import uuid
from datetime import timezone
import asyncpg
import argparse
from pathlib import Path
from telethon import TelegramClient, events
from config import *
from sql_queries import *


client = TelegramClient(SESSION_NAME, API_ID, API_HASH)

user_cache: dict[int, uuid.UUID] = {}
db_pool: asyncpg.Pool | None = None


async def get_user_uuid(conn: asyncpg.Connection, sender) -> uuid.UUID | None:
    """Возвращает uuid пользователя, создавая запись при необходимости, с кэшированием"""
    if sender is None:
        return None
    tg_user_id = getattr(sender, "id", None)
    if tg_user_id is None:
        return None

    if tg_user_id in user_cache:
        return user_cache[tg_user_id]

    username = getattr(sender, "username", None)

    nick = None
    if getattr(sender, "first_name", None): # If user
        last = getattr(sender, "last_name", None)
        nick = f"{sender.first_name} {last}".strip() if last else sender.first_name
    else: # If channel/chat
        nick = getattr(sender, "title", None)

    row = await conn.fetchrow(SQL_UPSERT_USER, tg_user_id, username, nick)
    user_uuid = row["id"]
    user_cache[tg_user_id] = user_uuid
    return user_uuid


async def save_message(conn: asyncpg.Connection, msg, sender_uuid: uuid.UUID) -> None:
    """Сохраняет сообщение, без повторов"""
    chat_id = msg.chat_id
    msg_id = msg.id
    send_time = msg.date.astimezone(timezone.utc)
    text = msg.text or ""

    if msg.gif:
        mtype = "gif"
    elif msg.sticker:
        mtype = "sticker"
    elif text:
        mtype = "text"
    else:
        mtype = "other"

    await conn.execute(SQL_INSERT_MESSAGE, chat_id, msg_id, send_time, sender_uuid, mtype, text)


async def process_history(pool: asyncpg.Pool) -> int:
    """Первичная загрузка истории для всех групп в GROUPS.

    Возвращает общее количество новых сохранённых сообщений.
    """
    total_counter = 0
    async with pool.acquire() as conn:
        progress_every = 100

        for group in GROUPS:
                if group is None:
                    continue

                group_name = str(group)
                print(f"Processing history for configured group: {group}")

                # Try to get group entity
                entity = None
                try:
                    entity = await client.get_input_entity(group)
                except ValueError:
                    if isinstance(group, int):
                        try:
                            alt = int(f"-100{group}")
                            entity = await client.get_input_entity(alt)
                            print(f"Resolved group by trying '-100' prefix: using {alt} as the entity.")
                        except ValueError:
                            print(f"Failed to resolve numeric group id {group} even after trying -100 prefix.")
                            print("Ensure the session account is a member of the group or provide the group's @username instead.")
                            continue
                    else:
                        print(f"Failed to resolve group '{group}'. Skipping.")
                        continue

                # Determine chat_id by fetching a single recent message (best-effort).
                chat_id = None
                sample = None
                try:
                    sample = await client.get_messages(entity, limit=1)
                    if sample:
                        sample_msg = sample[0]
                        chat_id = getattr(sample_msg, "chat_id", None)
                except Exception:
                    chat_id = None

                if chat_id is None:
                    try:
                        resolved_ent = await client.get_entity(entity)
                        ent_id = getattr(resolved_ent, "id", None)
                        if ent_id is not None:
                            try:
                                chat_id = int(f"-100{ent_id}")
                            except Exception:
                                chat_id = ent_id
                            # Attempt to get a human-friendly name
                            group_name = getattr(resolved_ent, "title", None) or getattr(resolved_ent, "username", None) or str(group)
                    except Exception:
                        chat_id = None
                        group_name = str(group)
                else:
                    # If we have sample, try to infer name from message peer
                    try:
                        resolved_ent = await client.get_entity(entity)
                        group_name = getattr(resolved_ent, "title", None) or getattr(resolved_ent, "username", None) or str(group)
                    except Exception:
                        group_name = str(group)

                # Upsert group into DB if we have a chat_id
                if chat_id is not None:
                    try:
                        await conn.fetchrow(SQL_UPSERT_GROUP, chat_id, group_name)
                    except Exception as e:
                        print(f"Warning: failed to upsert group {chat_id}: {e}")

                # Check last saved message id for this chat_id
                saved_last_id = None
                if chat_id is not None:
                    try:
                        row = await conn.fetchrow("SELECT max(message_id) AS last_id FROM messages WHERE chat_id=$1", chat_id)
                        if row:
                            saved_last_id = row.get("last_id")
                            print(f"Last saved message id for chat_id {chat_id} is {saved_last_id}")
                    except Exception as e:
                        print(f"Failed to query DB for last message id: {e}")

                # Quick early-exit per-group
                try:
                    if saved_last_id is not None and sample:
                        latest = getattr(sample[0], "id", None)
                        if latest is not None and latest <= saved_last_id:
                            print(f"No new messages since last_id {saved_last_id} for chat_id {chat_id}; skipping.")
                            continue
                except Exception:
                    pass

                counter = 0
                progress_every = 100

                # Use explicit parameters for iter_messages to avoid type-stub confusion
                try:
                    if saved_last_id:
                        # min_id returns messages with id > min_id, so this skips already-saved messages.
                        async for msg in client.iter_messages(entity, reverse=True, min_id=saved_last_id):
                            
                            # skip already saved messages if DB indicates so (best-effort)
                            if getattr(msg, "id", None) is not None and msg.id <= saved_last_id:
                                continue

                            sender = await msg.get_sender()
                            sender_uuid = await get_user_uuid(conn, sender)
                            if sender_uuid:
                                await save_message(conn, msg, sender_uuid)

                            counter += 1
                            total_counter += 1

                            # Print progress periodically (every `progress_every` messages)
                            if counter % progress_every == 0:
                                last_dt = msg.date.isoformat() if getattr(msg, "date", None) else "unknown"
                                print(f"Processed {counter} messages for {group} (last id={msg.id}, date={last_dt})")
                    else:
                        async for msg in client.iter_messages(entity, reverse=True):
                            # skip already saved messages if DB indicates so (best-effort)
                            if saved_last_id and getattr(msg, "id", None) is not None and msg.id <= saved_last_id:
                                continue

                            sender = await msg.get_sender()
                            sender_uuid = await get_user_uuid(conn, sender)
                            if sender_uuid:
                                await save_message(conn, msg, sender_uuid)

                            counter += 1
                            total_counter += 1

                            # Print progress periodically (every `progress_every` messages)
                            if counter % progress_every == 0:
                                last_dt = msg.date.isoformat() if getattr(msg, "date", None) else "unknown"
                                print(f"Processed {counter} messages for {group} (last id={msg.id}, date={last_dt})")
                finally:
                    print(f"Finished processing history for {group}. New messages processed: {counter}")

    print(f"Finished full history sync. Total new messages processed across groups: {total_counter}")
    return total_counter


async def new_message_handler(event):
    """Обработка новых сообщений в реальном времени"""
    if db_pool is None:
        print("DB pool not initialized yet; skipping new message handling")
        return
    async with db_pool.acquire() as conn:
        sender = await event.message.get_sender()
        sender_uuid = await get_user_uuid(conn, sender)
        if sender_uuid:
            await save_message(conn, event.message, sender_uuid)


async def main():
    global db_pool
    db_pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=5)

    print("Синхронизация истории...")
    await client.start()
    new_messages = await process_history(db_pool)

    # If no new messages were processed, exit instead of waiting for new messages.
    if isinstance(new_messages, int) and new_messages == 0:
        print("История синхронизирована. Новых сообщений не найдено — завершаемся.")
        return

    print("История синхронизирована. Ожидание новых сообщений...")
    # Register real-time handler(s) for configured groups
    if GROUPS:
        try:
            client.add_event_handler(new_message_handler, events.NewMessage(chats=tuple(GROUPS)))
            print(f"Registered real-time handler for groups: {GROUPS}")
        except Exception as e:
            print(f"Warning: failed to register event handlers for groups {GROUPS}: {e}")

    await client.run_until_disconnected()


if __name__ == "__main__":
    ap = argparse.ArgumentParser(description="TG group history sync")
    # Run main
    asyncio.run(main())
