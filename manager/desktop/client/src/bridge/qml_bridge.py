"""Bridge exposing Python services to QML via QObject slots/signals."""
from __future__ import annotations

import asyncio
from typing import Any

from PySide6.QtCore import QObject, Signal, Slot

from ..core.config import config
from ..core.database import Database
from ..core.network import APIClient
from ..core.websocket import RealtimeClient
from ..services.chat_service import ChatService
from ..services.sync_service import SyncService


class QmlBridge(QObject):
    """Registered as `bridge` context property in main.py."""

    # signals consumed by QML
    messageReceived = Signal(str, str)   # (convId, json payload)
    contactsUpdated = Signal(str)        # json list string
    connectionStateChanged = Signal(bool)

    def __init__(self, loop: asyncio.AbstractEventLoop) -> None:
        super().__init__()
        self._loop = loop

        import sqlite3
        self._db = Database(config.get("path.db"))
        self._api = APIClient(config.get("server.http"))
        self._rt: RealtimeClient | None = None
        self._chat: ChatService | None = None
        self._sync: SyncService | None = None

        self._me_id = ""
        self._current_conv = ""

    # ---- lifecycle ----
    @Slot(str, str, result=bool)
    def login(self, username: str, password: str) -> bool:
        try:
            resp = asyncio.run_coroutine_threadsafe(
                self._api.login(username, password), self._loop
            ).result(timeout=15)
        except Exception:
            self.connectionStateChanged.emit(False)
            return False

        self._me_id = str(resp.get("user_id") or resp.get("id") or "")
        token = str(resp.get("token") or "")
        self._api = APIClient(config.get("server.http"), token=token)
        self._rt = RealtimeClient(config.get("server.ws"), token)
        self._chat = ChatService(self._db, self._api, self._rt, self._me_id)
        self._sync = SyncService(self._db, self._api)
        self._chat.on_new_message(self._on_message)

        def _start():
            async def go():
                await self._rt.connect()
                await self._sync.sync_contacts()
                self.connectionStateChanged.emit(True)
            asyncio.ensure_future(go(), loop=self._loop)

        asyncio.run_coroutine_threadsafe(_start(), self._loop)
        return True

    def _on_message(self, msg: Any) -> None:
        import json as _json
        self.messageReceived.emit(msg.conv_id, _json.dumps(msg.to_dict()))

    # ---- qml-facing ----
    @Slot(str, str, result=bool)
    def sendMessage(self, conv_id: str, content: str) -> bool:
        if not self._chat:
            return False
        asyncio.run_coroutine_threadsafe(
            self._chat.send(conv_id, content), self._loop
        )
        return True

    @Slot(str)
    def openConversation(self, conv_id: str) -> None:
        self._current_conv = conv_id
        if self._chat:
            asyncio.run_coroutine_threadsafe(
                self._chat.reload_history(conv_id), self._loop
            )

    @Slot(result=str)
    def selfUserId(self) -> str:
        return self._me_id

    @Slot(result=str)
    def appVersion(self) -> str:
        return str(config.get("app.version"))

    # ---- shutdown ----
    def shutdown(self) -> None:
        if self._rt:
            asyncio.run_coroutine_threadsafe(self._rt.close(), self._loop)