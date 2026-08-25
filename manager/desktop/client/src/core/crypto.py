"""Cryptographic primitives: password hashing and payload signing/encryption."""
from __future__ import annotations

import base64
import hashlib
import hmac
import os

from cryptography.fernet import Fernet


def random_key() -> bytes:
    """Generate a 32-byte random key (Fernet format)."""
    return Fernet.generate_key()


def hash_password(password: str, salt: bytes | None = None) -> bytes:
    """PBKDF2-HMAC-SHA256 password hash. Returns salt||hash."""
    salt = salt or os.urandom(16)
    dk = hashlib.pbkdf2_hmac(
        "sha256", password.encode("utf-8"), salt, iterations=120_000
    )
    return salt + dk


def verify_password(password: str, stored: bytes) -> bool:
    """Verify a stored salt||hash blob."""
    if len(stored) < 16:
        return False
    salt, expected = stored[:16], stored[16:]
    return hmac.compare_digest(hash_password(password, salt)[16:], expected)


class Symmetric:
    """Symmetric encryption wrapper for message payloads / sensitive config."""

    def __init__(self, key: bytes | None = None) -> None:
        self._fernet = Fernet(key if key else random_key())

    @classmethod
    def from_password(cls, password: str, salt: bytes) -> "Symmetric":
        key = base64.urlsafe_b64encode(
            hashlib.pbkdf2_hmac(
                "sha256", password.encode("utf-8"), salt, iterations=120_000
            )
        )
        return cls(key)

    def encrypt(self, plaintext: bytes) -> bytes:
        return self._fernet.encrypt(plaintext)

    def decrypt(self, token: bytes) -> bytes:
        return self._fernet.decrypt(token)


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()