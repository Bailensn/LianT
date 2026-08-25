"""LianT desktop client entrypoint (source-execution mode).

Run via the runtime interpreter:
    python src/main.py
or
    <root>/client/runtime/bin/python src/main.py
"""
from __future__ import annotations

import asyncio
import os
import signal
import sys
from pathlib import Path

# Ensure the `src` directory is importable as package root when invoked as a
# script (python src/main.py), so `from core...` / `from models...` resolve.
_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

from PySide6.QtCore import QUrl                 # noqa: E402
from PySide6.QtGui import QGuiApplication       # noqa: E402
from PySide6.QtQml import QQmlApplicationEngine # noqa: E402

from bridge.qml_bridge import QmlBridge  # noqa: E402
from core.config import config           # noqa: E402


def main() -> int:
    app = QGuiApplication(sys.argv)
    app.setApplicationName("LianT")
    app.setApplicationVersion(str(config.get("app.version")))

    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    sys.excepthook = lambda *a: print("FATAL:", a, file=sys.stderr)

    bridge = QmlBridge(loop)
    engine = QQmlApplicationEngine()
    engine.rootContext().setContextProperty("bridge", bridge)

    qml_root = config.get("path.client").parent / "qml"
    engine.load(QUrl.fromLocalFile(str(qml_root / "Main.qml")))
    if not engine.rootObjects():
        print("ERROR: failed to load Main.qml", file=sys.stderr)
        return 1

    # Drive the asyncio loop in a worker thread.
    def _run_loop():
        loop.run_forever()

    import threading
    th = threading.Thread(target=_run_loop, daemon=True)
    th.start()

    def _shutdown(*_):
        bridge.shutdown()
        loop.call_soon_threadsafe(loop.stop)
        app.quit()

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)
    app.aboutToQuit.connect(lambda: (bridge.shutdown(), loop.call_soon_threadsafe(loop.stop)))

    rc = app.exec()
    loop.call_soon_threadsafe(loop.stop)
    return rc


if __name__ == "__main__":
    raise SystemExit(main())