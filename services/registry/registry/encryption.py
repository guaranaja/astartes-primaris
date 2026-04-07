"""
Astartes Strategy Registry — Encryption & Bundle Utilities
===========================================================

Provides:
    - AES-256-GCM config encryption (per-bundle random data key)
    - X25519 key wrapping (per-client public key encrypts the data key)
    - Ed25519 bundle signing (master signs, clients verify)
    - Bundle packing/unpacking (.astartes tarball format)

Bundle format (.astartes = gzipped tar):
    manifest.json          — plaintext: strategy name, version, hashes, timestamp, valid_until
    config.enc             — AES-256-GCM encrypted config JSON (nonce prepended)
    strategy.so            — Cython-compiled strategy module
    *.onnx                 — ONNX model files
    keys/{client_id}.key   — AES data key encrypted with client's X25519 public key
    bundle.sig             — Ed25519 signature over SHA-256 of manifest + all file hashes
"""

import hashlib
import io
import json
import os
import secrets
import tarfile
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

from cryptography.hazmat.primitives.asymmetric.ed25519 import (
    Ed25519PrivateKey,
    Ed25519PublicKey,
)
from cryptography.hazmat.primitives.asymmetric.x25519 import (
    X25519PrivateKey,
    X25519PublicKey,
)
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.hashes import SHA256
from cryptography.hazmat.primitives.kdf.hkdf import HKDF
from cryptography.hazmat.primitives.serialization import (
    BestAvailableEncryption,
    Encoding,
    NoEncryption,
    PrivateFormat,
    PublicFormat,
)

# ── Constants ────────────────────────────────────────────────

AES_KEY_BYTES = 32       # 256-bit
AES_NONCE_BYTES = 12     # 96-bit nonce for GCM
BUNDLE_EXTENSION = ".astartes"
MANIFEST_FILENAME = "manifest.json"
CONFIG_ENC_FILENAME = "config.enc"
SIGNATURE_FILENAME = "bundle.sig"
KEYS_DIR = "keys"


# ── AES-256-GCM ─────────────────────────────────────────────

def generate_data_key() -> bytes:
    """Generate a random 256-bit AES data key."""
    return secrets.token_bytes(AES_KEY_BYTES)


def encrypt_config(config_json: bytes, data_key: bytes) -> bytes:
    """Encrypt config JSON with AES-256-GCM.

    Returns nonce (12 bytes) || ciphertext+tag.
    """
    aesgcm = AESGCM(data_key)
    nonce = os.urandom(AES_NONCE_BYTES)
    ciphertext = aesgcm.encrypt(nonce, config_json, None)
    return nonce + ciphertext


def decrypt_config(encrypted_blob: bytes, data_key: bytes) -> bytes:
    """Decrypt AES-256-GCM encrypted config.

    Expects nonce (12 bytes) || ciphertext+tag.
    """
    nonce = encrypted_blob[:AES_NONCE_BYTES]
    ciphertext = encrypted_blob[AES_NONCE_BYTES:]
    aesgcm = AESGCM(data_key)
    return aesgcm.decrypt(nonce, ciphertext, None)


# ── X25519 Key Exchange ─────────────────────────────────────

@dataclass
class ClientKeypair:
    """X25519 keypair for a client."""
    client_id: str
    private_key: X25519PrivateKey
    public_key: X25519PublicKey

    @classmethod
    def generate(cls, client_id: str) -> "ClientKeypair":
        private_key = X25519PrivateKey.generate()
        return cls(
            client_id=client_id,
            private_key=private_key,
            public_key=private_key.public_key(),
        )

    def private_key_pem(self, password: Optional[bytes] = None) -> bytes:
        encryption = BestAvailableEncryption(password) if password else NoEncryption()
        return self.private_key.private_bytes(
            Encoding.PEM, PrivateFormat.PKCS8, encryption
        )

    def public_key_pem(self) -> bytes:
        return self.public_key.public_bytes(Encoding.PEM, PublicFormat.SubjectPublicKeyInfo)

    @classmethod
    def from_private_pem(cls, client_id: str, pem: bytes, password: Optional[bytes] = None) -> "ClientKeypair":
        from cryptography.hazmat.primitives.serialization import load_pem_private_key
        private_key = load_pem_private_key(pem, password=password)
        if not isinstance(private_key, X25519PrivateKey):
            raise ValueError("PEM does not contain an X25519 private key")
        return cls(
            client_id=client_id,
            private_key=private_key,
            public_key=private_key.public_key(),
        )

    @classmethod
    def public_from_pem(cls, pem: bytes) -> X25519PublicKey:
        from cryptography.hazmat.primitives.serialization import load_pem_public_key
        public_key = load_pem_public_key(pem)
        if not isinstance(public_key, X25519PublicKey):
            raise ValueError("PEM does not contain an X25519 public key")
        return public_key


def wrap_data_key(data_key: bytes, client_public_key: X25519PublicKey) -> bytes:
    """Encrypt the AES data key for a specific client using X25519 + HKDF + AES-256-GCM.

    1. Generate ephemeral X25519 keypair
    2. ECDH with client's public key → shared secret
    3. HKDF to derive wrapping key
    4. AES-256-GCM encrypt the data key
    5. Return: ephemeral_public (32) || nonce (12) || ciphertext+tag

    The ephemeral public key is included so the client can reconstruct the shared secret.
    """
    ephemeral = X25519PrivateKey.generate()
    ephemeral_public = ephemeral.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)

    shared_secret = ephemeral.exchange(client_public_key)

    wrapping_key = HKDF(
        algorithm=SHA256(),
        length=AES_KEY_BYTES,
        salt=None,
        info=b"astartes-key-wrap-v1",
    ).derive(shared_secret)

    aesgcm = AESGCM(wrapping_key)
    nonce = os.urandom(AES_NONCE_BYTES)
    encrypted_data_key = aesgcm.encrypt(nonce, data_key, None)

    return ephemeral_public + nonce + encrypted_data_key


def unwrap_data_key(wrapped_key: bytes, client_private_key: X25519PrivateKey) -> bytes:
    """Decrypt the AES data key using the client's private key.

    Expects: ephemeral_public (32) || nonce (12) || ciphertext+tag
    """
    ephemeral_public_bytes = wrapped_key[:32]
    nonce = wrapped_key[32:32 + AES_NONCE_BYTES]
    encrypted_data_key = wrapped_key[32 + AES_NONCE_BYTES:]

    ephemeral_public = X25519PublicKey.from_public_bytes(ephemeral_public_bytes)
    shared_secret = client_private_key.exchange(ephemeral_public)

    wrapping_key = HKDF(
        algorithm=SHA256(),
        length=AES_KEY_BYTES,
        salt=None,
        info=b"astartes-key-wrap-v1",
    ).derive(shared_secret)

    aesgcm = AESGCM(wrapping_key)
    return aesgcm.decrypt(nonce, encrypted_data_key, None)


# ── Ed25519 Signing ──────────────────────────────────────────

@dataclass
class SigningKeypair:
    """Ed25519 keypair for bundle signing (master holds private, clients hold public)."""
    private_key: Ed25519PrivateKey
    public_key: Ed25519PublicKey

    @classmethod
    def generate(cls) -> "SigningKeypair":
        private_key = Ed25519PrivateKey.generate()
        return cls(private_key=private_key, public_key=private_key.public_key())

    def private_key_pem(self, password: Optional[bytes] = None) -> bytes:
        encryption = BestAvailableEncryption(password) if password else NoEncryption()
        return self.private_key.private_bytes(
            Encoding.PEM, PrivateFormat.PKCS8, encryption
        )

    def public_key_pem(self) -> bytes:
        return self.public_key.public_bytes(Encoding.PEM, PublicFormat.SubjectPublicKeyInfo)

    @classmethod
    def from_private_pem(cls, pem: bytes, password: Optional[bytes] = None) -> "SigningKeypair":
        from cryptography.hazmat.primitives.serialization import load_pem_private_key
        private_key = load_pem_private_key(pem, password=password)
        if not isinstance(private_key, Ed25519PrivateKey):
            raise ValueError("PEM does not contain an Ed25519 private key")
        return cls(private_key=private_key, public_key=private_key.public_key())

    @classmethod
    def public_from_pem(cls, pem: bytes) -> Ed25519PublicKey:
        from cryptography.hazmat.primitives.serialization import load_pem_public_key
        public_key = load_pem_public_key(pem)
        if not isinstance(public_key, Ed25519PublicKey):
            raise ValueError("PEM does not contain an Ed25519 public key")
        return public_key


def sign_bundle(data: bytes, signing_key: Ed25519PrivateKey) -> bytes:
    """Sign arbitrary data with Ed25519. Returns 64-byte signature."""
    return signing_key.sign(data)


def verify_bundle(data: bytes, signature: bytes, verify_key: Ed25519PublicKey) -> bool:
    """Verify Ed25519 signature. Returns True if valid, False otherwise."""
    try:
        verify_key.verify(signature, data)
        return True
    except Exception:
        return False


# ── Bundle Format ────────────────────────────────────────────

@dataclass
class BundleManifest:
    """Metadata for a strategy bundle."""
    strategy_id: str
    version: str
    published_at: str          # ISO 8601
    valid_until: Optional[str]  # ISO 8601 or None for no expiry
    file_hashes: dict = field(default_factory=dict)  # filename -> SHA-256 hex

    def to_json(self) -> bytes:
        return json.dumps({
            "strategy_id": self.strategy_id,
            "version": self.version,
            "published_at": self.published_at,
            "valid_until": self.valid_until,
            "file_hashes": self.file_hashes,
        }, indent=2).encode("utf-8")

    @classmethod
    def from_json(cls, data: bytes) -> "BundleManifest":
        d = json.loads(data)
        return cls(
            strategy_id=d["strategy_id"],
            version=d["version"],
            published_at=d["published_at"],
            valid_until=d.get("valid_until"),
            file_hashes=d.get("file_hashes", {}),
        )

    def signable_bytes(self) -> bytes:
        """Canonical bytes for signing: manifest JSON with sorted hashes."""
        return json.dumps({
            "strategy_id": self.strategy_id,
            "version": self.version,
            "published_at": self.published_at,
            "valid_until": self.valid_until,
            "file_hashes": dict(sorted(self.file_hashes.items())),
        }, sort_keys=True, separators=(",", ":")).encode("utf-8")


def _sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _add_bytes_to_tar(tf: tarfile.TarFile, name: str, data: bytes):
    """Add in-memory bytes to a tarfile."""
    info = tarfile.TarInfo(name=name)
    info.size = len(data)
    info.mtime = int(time.time())
    tf.addfile(info, io.BytesIO(data))


def pack_bundle(
    strategy_id: str,
    version: str,
    config_json: bytes,
    strategy_so: Optional[bytes],
    onnx_files: dict[str, bytes],
    client_public_keys: dict[str, X25519PublicKey],
    signing_key: Ed25519PrivateKey,
    valid_until: Optional[str] = None,
) -> bytes:
    """Pack an encrypted strategy bundle.

    Args:
        strategy_id: e.g. "eversor_es_150k_30s"
        version: semver e.g. "1.42.0"
        config_json: raw config JSON bytes (plaintext — will be encrypted)
        strategy_so: compiled strategy .so bytes (or None if not compiled)
        onnx_files: {"exit_timing.onnx": bytes, ...}
        client_public_keys: {"client_uuid": X25519PublicKey, ...} — wrap data key per client
        signing_key: Ed25519 private key for signing the bundle
        valid_until: optional expiry ISO timestamp

    Returns:
        Gzipped tar bytes (.astartes bundle)
    """
    from datetime import datetime, timezone

    # Generate random AES data key for this bundle
    data_key = generate_data_key()

    # Encrypt config
    encrypted_config = encrypt_config(config_json, data_key)

    # Build file hashes
    file_hashes = {CONFIG_ENC_FILENAME: _sha256_hex(encrypted_config)}

    if strategy_so:
        file_hashes["strategy.so"] = _sha256_hex(strategy_so)

    for onnx_name, onnx_data in onnx_files.items():
        file_hashes[onnx_name] = _sha256_hex(onnx_data)

    # Wrap data key per client
    wrapped_keys: dict[str, bytes] = {}
    for client_id, pub_key in client_public_keys.items():
        wrapped_keys[client_id] = wrap_data_key(data_key, pub_key)
        file_hashes[f"{KEYS_DIR}/{client_id}.key"] = _sha256_hex(wrapped_keys[client_id])

    # Build manifest
    manifest = BundleManifest(
        strategy_id=strategy_id,
        version=version,
        published_at=datetime.now(timezone.utc).isoformat(),
        valid_until=valid_until,
        file_hashes=file_hashes,
    )
    manifest_bytes = manifest.to_json()

    # Sign manifest
    signature = sign_bundle(manifest.signable_bytes(), signing_key)

    # Pack tarball
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        _add_bytes_to_tar(tf, MANIFEST_FILENAME, manifest_bytes)
        _add_bytes_to_tar(tf, CONFIG_ENC_FILENAME, encrypted_config)
        _add_bytes_to_tar(tf, SIGNATURE_FILENAME, signature)

        if strategy_so:
            _add_bytes_to_tar(tf, "strategy.so", strategy_so)

        for onnx_name, onnx_data in onnx_files.items():
            _add_bytes_to_tar(tf, onnx_name, onnx_data)

        for client_id, wrapped in wrapped_keys.items():
            _add_bytes_to_tar(tf, f"{KEYS_DIR}/{client_id}.key", wrapped)

    return buf.getvalue()


@dataclass
class UnpackedBundle:
    """Result of unpacking a .astartes bundle."""
    manifest: BundleManifest
    encrypted_config: bytes
    signature: bytes
    strategy_so: Optional[bytes] = None
    onnx_files: dict = field(default_factory=dict)
    wrapped_keys: dict = field(default_factory=dict)  # client_id -> wrapped key bytes


def unpack_bundle(bundle_bytes: bytes) -> UnpackedBundle:
    """Unpack a .astartes bundle from bytes.

    Does NOT verify signature or decrypt — caller should do that.
    """
    buf = io.BytesIO(bundle_bytes)
    result = UnpackedBundle(
        manifest=BundleManifest("", "", "", None),
        encrypted_config=b"",
        signature=b"",
    )

    with tarfile.open(fileobj=buf, mode="r:gz") as tf:
        for member in tf.getmembers():
            f = tf.extractfile(member)
            if f is None:
                continue
            data = f.read()
            name = member.name

            if name == MANIFEST_FILENAME:
                result.manifest = BundleManifest.from_json(data)
            elif name == CONFIG_ENC_FILENAME:
                result.encrypted_config = data
            elif name == SIGNATURE_FILENAME:
                result.signature = data
            elif name == "strategy.so":
                result.strategy_so = data
            elif name.endswith(".onnx"):
                result.onnx_files[name] = data
            elif name.startswith(f"{KEYS_DIR}/") and name.endswith(".key"):
                client_id = name[len(f"{KEYS_DIR}/"):-len(".key")]
                result.wrapped_keys[client_id] = data

    return result


def verify_and_decrypt_bundle(
    bundle: UnpackedBundle,
    client_id: str,
    client_private_key: X25519PrivateKey,
    verify_key: Ed25519PublicKey,
) -> dict:
    """Verify bundle signature and decrypt config for a specific client.

    Args:
        bundle: Unpacked bundle from unpack_bundle()
        client_id: This client's ID
        client_private_key: This client's X25519 private key
        verify_key: Master's Ed25519 public key (for signature verification)

    Returns:
        Decrypted config as a Python dict.

    Raises:
        ValueError: If signature is invalid, client key not found, or integrity check fails.
    """
    # 1. Verify signature
    if not verify_bundle(bundle.manifest.signable_bytes(), bundle.signature, verify_key):
        raise ValueError("Bundle signature verification failed — bundle may be tampered")

    # 2. Verify file hashes
    actual_config_hash = _sha256_hex(bundle.encrypted_config)
    expected_config_hash = bundle.manifest.file_hashes.get(CONFIG_ENC_FILENAME)
    if actual_config_hash != expected_config_hash:
        raise ValueError(f"Config hash mismatch: expected {expected_config_hash}, got {actual_config_hash}")

    if bundle.strategy_so:
        actual_so_hash = _sha256_hex(bundle.strategy_so)
        expected_so_hash = bundle.manifest.file_hashes.get("strategy.so")
        if actual_so_hash != expected_so_hash:
            raise ValueError("Strategy .so hash mismatch")

    for onnx_name, onnx_data in bundle.onnx_files.items():
        actual_hash = _sha256_hex(onnx_data)
        expected_hash = bundle.manifest.file_hashes.get(onnx_name)
        if actual_hash != expected_hash:
            raise ValueError(f"ONNX file {onnx_name} hash mismatch")

    # 3. Find and unwrap this client's data key
    if client_id not in bundle.wrapped_keys:
        raise ValueError(f"No wrapped key found for client {client_id} — access may be revoked")

    data_key = unwrap_data_key(bundle.wrapped_keys[client_id], client_private_key)

    # 4. Decrypt config
    config_bytes = decrypt_config(bundle.encrypted_config, data_key)
    return json.loads(config_bytes)


# ── Check bundle expiry ──────────────────────────────────────

def is_bundle_expired(manifest: BundleManifest) -> bool:
    """Check if a bundle has passed its valid_until timestamp."""
    if manifest.valid_until is None:
        return False
    from datetime import datetime, timezone
    expiry = datetime.fromisoformat(manifest.valid_until)
    if expiry.tzinfo is None:
        expiry = expiry.replace(tzinfo=timezone.utc)
    return datetime.now(timezone.utc) > expiry
