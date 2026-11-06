"""Minimal config stub."""
from __future__ import annotations

from pathlib import Path
from typing import Any


class Config:
    def __init__(self) -> None:
        self._values: dict[str, Any] = {
            "app.name": "LianT",
            "app.version": "1.00",
        }

    def get(self, key: str, default: Any = None) -> Any:
        return self._values.get(key, default)

    def set(self, key: str, value: Any) -> None:
        self._values[key] = value


config = Config()