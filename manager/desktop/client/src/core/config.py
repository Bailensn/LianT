"""Central configuration loaded from environment / config file."""
from __future__ import annotations

import json
import os
import platform
import sys
from pathlib import Path
from typing import Any


def _app_data_dir() -> Path:
    system = platform.system()
    if system == "Windows":
        base = os.environ.get("APPDATA") or str(Path.home() / "AppData/Roaming")
    elif system == "Darwin":
        base = os.environ.get("HOME") or "."
        return Path(base) / "Library/Application Support"
    else:
        base = os.environ.get("XDG_CONFIG_HOME") or (Path.home() / ".config")
    return Path(base)


def _runtime_root() -> Path:
    """Locate the LianT runtime root (parent of client/).

    Layout (source-execution):
      <root>/client/src/main.py
      <root>/client/runtime/bin/python
      <root>/client/resources
    """
    here = Path(__file__).resolve()  # .../client/src/core/config.py
    src_dir = here.parent.parent        # .../client/src
    return src_dir.parent               # .../client


class Config:
    """Application settings, resolved lazily, cacheable via `as_dict`."""

    def __init__(self) -> None:
        # The Go launcher points LIANT_CLIENT_SRC at the resolved client src dir
        # (e.g. /usr/lib/liant/src when installed). Prefer it over the
        # __file__-based guess so the client works regardless of how/where the
        # source tree was laid out at install time.
        env_src = os.environ.get("LIANT_CLIENT_SRC")
        if env_src:
            env_src_path = Path(env_src)
            self.root: Path = env_src_path.parent
            client_root = self.root
        else:
            self.root = _runtime_root()
            client_root = self.root

        self.app_data: Path = _app_data_dir() / "LianT"
        self.app_data.mkdir(parents=True, exist_ok=True)

        self._values: dict[str, Any] = {}

        # Defaults
        self.set("app.name", "LianT")
        self.set("app.version", os.environ.get("LIANT_VERSION", "0.0.0"))
        self.set("app.log_level", os.environ.get("LIANT_LOG_LEVEL", "INFO"))

        server = os.environ.get("LIANT_SERVER", "https://api.liant.dev")
        self.set("server.http", server.rstrip("/"))
        wss = server.replace("http://", "ws://", 1).replace("https://", "wss://", 1)
        self.set("server.ws", wss.rstrip("/"))

        src_dir = (env_src_path if env_src else client_root / "src")
        self.set("path.client", src_dir)
        self.set("path.runtime", client_root / "runtime")
        self.set("path.resources", client_root / "resources")
        self.set(
            "path.db",
            Path(os.environ.get("LIANT_DB", str(self.app_data / "liant.db"))),
        )

        # Schema version for local db migrations.
        self.set("db.schema_version", 1)

    # ---- helpers ----
    def set(self, key: str, value: Any) -> None:
        self._values[key] = value

    def get(self, key: str, default: Any = None) -> Any:
        return self._values.get(key, default)

    def load_file(self, path: Path | None = None) -> None:
        """Merge settings from a JSON file (env overrides remain authoritative)."""
        cfg = path or (self.app_data / "config.json")
        if not cfg.exists():
            return
        try:
            data: dict = json.loads(cfg.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return
        self._values.update(data)

    def save_file(self, path: Path | None = None) -> None:
        cfg = path or (self.app_data / "config.json")
        cfg.parent.mkdir(parents=True, exist_ok=True)
        dumpable = {
            k: str(v) if isinstance(v, Path) else v
            for k, v in self._values.items()
        }
        cfg.write_text(json.dumps(dumpable, ensure_ascii=False, indent=2),
                       encoding="utf-8")

    def as_dict(self) -> dict[str, Any]:
        return dict(self._values)


# Module-level convenience instance.
config = Config()