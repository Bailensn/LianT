"""LianT user model."""
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class UserProfile:
    id: str
    username: str
    nickname: Optional[str] = None
    avatar_url: Optional[str] = None
    signature: Optional[str] = None
    status: str = "offline"  # online / away / busy / offline

    @classmethod
    def from_dict(cls, d: dict) -> "UserProfile":
        return cls(
            id=str(d.get("id", "")),
            username=d.get("username", ""),
            nickname=d.get("nickname"),
            avatar_url=d.get("avatar_url"),
            signature=d.get("signature"),
            status=d.get("status", "offline"),
        )

    @property
    def display_name(self) -> str:
        return self.nickname or self.username or self.id