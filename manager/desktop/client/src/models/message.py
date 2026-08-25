"""LianT message model."""
from dataclasses import dataclass, field
from typing import Any, Optional


@dataclass
class Message:
    id: str
    conv_id: str
    sender_id: str
    sender_name: str = ""
    content: str = ""
    msg_type: str = "text"  # text / image / file / system
    timestamp: float = 0.0
    status: str = "sent"  # sending / sent / delivered / read / failed
    local_only: bool = False
    extra: dict = field(default_factory=dict)

    @classmethod
    def from_dict(cls, d: dict) -> "Message":
        return cls(
            id=str(d.get("id", "")),
            conv_id=str(d.get("conv_id", "")),
            sender_id=str(d.get("sender_id", "")),
            sender_name=d.get("sender_name", ""),
            content=d.get("content", ""),
            msg_type=d.get("msg_type", "text"),
            timestamp=float(d.get("timestamp", 0.0)),
            status=d.get("status", "sent"),
            local_only=d.get("local_only", False),
            extra=d.get("extra", {}) or {},
        )

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "conv_id": self.conv_id,
            "sender_id": self.sender_id,
            "sender_name": self.sender_name,
            "content": self.content,
            "msg_type": self.msg_type,
            "timestamp": self.timestamp,
            "status": self.status,
            "local_only": self.local_only,
            "extra": self.extra,
        }


def make_local_message(conv_id: str, sender_id: str, sender_name: str,
                       content: str, msg_type: str = "text") -> Message:
    """Create a client-side placeholder message with a new id."""
    import time
    import uuid
    return Message(
        id=uuid.uuid4().hex,
        conv_id=conv_id,
        sender_id=sender_id,
        sender_name=sender_name,
        content=content,
        msg_type=msg_type,
        timestamp=time.time(),
        status="sending",
        local_only=True,
    )