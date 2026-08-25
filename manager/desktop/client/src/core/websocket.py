"""WebSocket client for realtime events (messages, presence, sync)."""
from __future__ import annotations

import asyncio
import json
from typing import Callable

import aiohttp

EventHandler = Callable[[dict], None]


class RealtimeClient:
    def __init__(self, ws_url: str, token: str) -> None:
        self._url = ws_url.rstrip("/") + "/ws"
        self._token = token
        self._session: aiohttp.ClientSession | None = None
        self._ws: aiohttp.ClientWebSocketResponse | None = None
        self._closed = False
        self._handlers: dict[str, list[EventHandler]] = {}
        self._task: asyncio.Task | None = None
        self._connected = False

    def on(self, event: str, handler: EventHandler) -> None:
        self._handlers.setdefault(event, []).append(handler)

    async def connect(self) -> None:
        self._closed = False
        self._session = aiohttp.ClientSession()
        self._ws = await self._session.ws_connect(
            self._url, headers={"Authorization": f"Bearer {self._token}"}
        )
        self._connected = True
        self._task = asyncio.create_task(self._reader())

    async def _reader(self) -> None:
        try:
            async for raw in self._ws:
                try:
                    event = json.loads(raw.data)
                except (TypeError, json.JSONDecodeError):
                    mtype = getattr(raw, "type", None)
                    if mtype == aiohttp.WSMsgType.CLOSED or mtype is None:
                        break
                    continue
                name = event.get("event", "message")
                for h in self._handlers.get(name, []):
                    try:
                        h(event.get("data", event))
                    except Exception:
                        pass
        finally:
            self._connected = False

    async def send(self, payload: dict) -> None:
        if self._ws and not self._ws.closed:
            await self._ws.send_str(json.dumps(payload))

    async def close(self) -> None:
        self._closed = True
        if self._task:
            self._task.cancel()
        if self._ws and not self._ws.closed:
            await self._ws.close()
        if self._session and not self._session.closed:
            await self._session.close()

    @property
    def connected(self) -> bool:
        return self._connected