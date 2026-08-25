"""Local SQLite persistence for contacts, conversations and message cache."""
from __future__ import annotations

import json
import sqlite3
import threading
import time
from pathlib import Path

from ..models.message import Message
from ..models.user import UserProfile

_SCHEMA = """
CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT
);
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT,
    nickname TEXT,
    avatar_url TEXT,
    signature TEXT,
    status TEXT DEFAULT 'offline'
);
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    peer_id TEXT,
    title TEXT,
    last_message TEXT,
    last_ts REAL,
    unread INTEGER DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conv_id TEXT,
    sender_id TEXT,
    sender_name TEXT,
    content TEXT,
    msg_type TEXT,
    ts REAL,
    status TEXT,
    extra TEXT
);
CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conv_id, ts);
"""


class Database:
    """Thread-safe wrapper around sqlite3 with a simple write lock."""

    def __init__(self, path: Path) -> None:
        self._path = path
        self._lock = threading.RLock()
        self._conn: sqlite3.Connection | None = None
        self._open()

    def _open(self) -> None:
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._conn = sqlite3.connect(str(self._path), check_same_thread=False)
        self._conn.row_factory = sqlite3.Row
        self._conn.executescript(_SCHEMA)
        self._conn.commit()

    def _execute(self, sql: str, params: tuple = ()) -> sqlite3.Cursor:
        with self._lock:
            cur = self._conn.execute(sql, params)
            self._conn.commit()
            return cur

    def close(self) -> None:
        with self._lock:
            if self._conn:
                self._conn.close()
                self._conn = None

    # ---- meta ----
    def get_meta(self, key: str) -> str | None:
        row = self._execute("SELECT value FROM meta WHERE key=?", (key,)).fetchone()
        return row["value"] if row else None

    def set_meta(self, key: str, value: str) -> None:
        self._execute(
            "INSERT OR REPLACE INTO meta(key,value) VALUES(?,?)", (key, value)
        )

    # ---- users ----
    def upsert_user(self, u: UserProfile) -> None:
        self._execute(
            """
            INSERT OR REPLACE INTO users(id,username,nickname,avatar_url,signature,status)
            VALUES(?,?,?,?,?,?)
            """,
            (u.id, u.username, u.nickname, u.avatar_url, u.signature, u.status),
        )

    def get_user(self, user_id: str) -> UserProfile | None:
        row = self._execute(
            "SELECT * FROM users WHERE id=?", (user_id,)
        ).fetchone()
        return UserProfile.from_dict(dict(row)) if row else None

    def list_users(self) -> list[UserProfile]:
        rows = self._execute("SELECT * FROM users").fetchall()
        return [UserProfile.from_dict(dict(r)) for r in rows]

    # ---- messages ----
    def save_message(self, m: Message) -> None:
        self._execute(
            """
            INSERT OR REPLACE INTO messages
              (id,conv_id,sender_id,sender_name,content,msg_type,ts,status,extra)
            VALUES(?,?,?,?,?,?,?,?,?)
            """,
            (
                m.id, m.conv_id, m.sender_id, m.sender_name, m.content,
                m.msg_type, m.timestamp, m.status,
                json.dumps(m.extra, ensure_ascii=False),
            ),
        )

    def messages_for(self, conv_id: str, limit: int = 200) -> list[Message]:
        rows = self._execute(
            "SELECT * FROM messages WHERE conv_id=? ORDER BY ts DESC LIMIT ?",
            (conv_id, limit),
        ).fetchall()
        out = [Message.from_dict(_row_to_msg_dict(r)) for r in rows]
        return out[::-1]

    def delete_messages(self, conv_id: str) -> None:
        self._execute("DELETE FROM messages WHERE conv_id=?", (conv_id,))


def _row_to_msg_dict(row: sqlite3.Row) -> dict:
    d = dict(row)
    if "extra" in d:
        try:
            d["extra"] = json.loads(d["extra"]) or {}
        except json.JSONDecodeError:
            d["extra"] = {}
    if "ts" in d:
        d["timestamp"] = d.pop("ts")
    return d