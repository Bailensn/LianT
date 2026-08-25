"""HTTP transport helpers built on aiohttp."""
from __future__ import annotations

import asyncio
from typing import Any

import aiohttp


class APIClient:
    def __init__(self, base_url: str, token: str | None = None,
                 timeout: float = 15.0) -> None:
        self._base = base_url.rstrip("/")
        self._token = token
        self._timeout = timeout
        self._session: aiohttp.ClientSession | None = None

    async def _session_ensure(self) -> aiohttp.ClientSession:
        if self._session is None or self._session.closed:
            timeout = aiohttp.ClientTimeout(total=self._timeout)
            self._session = aiohttp.ClientSession(timeout=timeout)
        return self._session

    def _headers(self) -> dict[str, str]:
        h = {"Accept": "application/json", "User-Agent": "LianT-Client/1.0"}
        if self._token:
            h["Authorization"] = f"Bearer {self._token}"
        return h

    async def request(self, method: str, path: str,
                      json: Any | None = None) -> Any:
        session = await self._session_ensure()
        url = f"{self._base}/{path.lstrip('/')}"
        async with session.request(
            method, url, json=json, headers=self._headers()
        ) as resp:
            body = await resp.json(content_type=None)
            if resp.status >= 400:
                exc = APIError(resp.status, body)
                raise exc
            return body

    async def login(self, username: str, password: str) -> dict:
        return await self.request(
            "POST", "/auth/login", {"username": username, "password": password}
        )

    async def fetch_profile(self, user_id: str) -> dict:
        return await self.request(
            "GET", f"/users/{user_id}"
        )

    async def close(self) -> None:
        if self._session and not self._session.closed:
            await self._session.close()


class APIError(Exception):
    def __init__(self, status: int, payload: Any) -> None:
        self.status = status
        self.payload = payload
        msg = payload.get("error") or payload.get("message") if isinstance(
            payload, dict) else str(payload)
        super().__init__(f"HTTP {status}: {msg}")