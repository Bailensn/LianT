"""Chat orchestration: send/receive messages, maintain conversations."""
from __future__ import annotations

import asyncio
from typing import Callable

from ..core.config import config
from ..core.database import Database
from ..core.network import APIClient
from ..core.websocket import RealtimeClient
from ..models.message import Message, make_local_message

MessageHandler = Callable[[Message], None]


class ChatService:
    def __init__(self, db: Database, api: APIClient, rt: RealtimeClient,
                 self_user_id: str) -> None:
        self._db = db
        self._api = api
        self._rt = rt
        self._self_id = self_user_id
        self._handlers: list[MessageHandler] = []
        self._rt.on("message", self._on_rt_message)

    def on_new_message(self, handler: MessageHandler) -> None:
        self._handlers.append(handler)

    def _on_rt_message(self, data: dict) -> None:
        m = Message.from_dict(data)
        self._db.save_message(m)
        for h in self._handlers:
            try:
                h(m)
            except Exception:
                pass

    async def send(self, conv_id: str, content: str,
                   msg_type: str = "text") -> Message:
        local = make_local_message(conv_id, self._self_id, "me", content, msg_type)
        self._db.save_message(local)
        for h in self._handlers:
            try:
                h(local)
            except Exception:
                pass

        try:
            sent = await self._api.request(
                "POST",
                f"/conversations/{conv_id}/messages",
                {"content": content, "msg_type": msg_type},
            )
            acked = Message.from_dict(sent)
            acked.status = "sent"
            self._db.save_message(acked)
            for h in self._handlers:
                try:
                    h(acked)
                except Exception:
                    pass
            return acked
        except Exception:
            local.status = "failed"
            self._db.save_message(local)
            raise

    async def reload_history(self, conv_id: str, limit: int = 100) -> list[Message]:
        try:
            rows = await self._api.request(
                "GET", f"/conversations/{conv_id}/messages?limit={limit}"
            )
            msgs = [Message.from_dict(r) for r in (rows or [])]
        except Exception:
            msgs = self._db.messages_for(conv_id, limit)
        for m in msgs:
            self._db.save_message(m)
        return msgs