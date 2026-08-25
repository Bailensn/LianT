"""Synchronization of contact list / profile snapshots from the server."""
from __future__ import annotations

from typing import Callable

from ..core.config import config
from ..core.database import Database
from ..core.network import APIClient
from ..models.user import UserProfile

SyncHandler = Callable[[list[UserProfile]], None]


class SyncService:
    def __init__(self, db: Database, api: APIClient) -> None:
        self._db = db
        self._api = api
        self._handlers: list[SyncHandler] = []

    def on_contacts(self, handler: SyncHandler) -> None:
        self._handlers.append(handler)

    async def sync_contacts(self) -> list[UserProfile]:
        try:
            payload = await self._api.request("GET", "/contacts")
        except Exception:
            return self._db.list_users()

        users = [UserProfile.from_dict(d) for d in (payload or [])]
        for u in users:
            self._db.upsert_user(u)
        for h in self._handlers:
            try:
                h(users)
            except Exception:
                pass
        return users