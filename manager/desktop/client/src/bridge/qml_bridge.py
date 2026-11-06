"""Minimal QML bridge stub."""
from __future__ import annotations

from PySide6.QtCore import QObject, Slot


class QmlBridge(QObject):
    @Slot(result=str)
    def appVersion(self) -> str:
        return "1.00"