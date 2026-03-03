"""Verify that required CPython stdlib extension modules are available.

Run during Docker image build to fail fast if any extension is missing.
Extensions are grouped by category so failures are easy to diagnose.
"""

import importlib
import sys

# Extensions required by the Imperium's Python services.
# Each tuple: (module_name, reason)
REQUIRED_EXTENSIONS = [
    # ── Core ──────────────────────────────────────────────
    ("_ctypes",     "FFI — required by numpy, numba, and most C-backed packages"),
    ("_decimal",    "Decimal arithmetic — required for precise financial calculations"),
    ("_json",       "Fast JSON — required by httpx, nats-py, structlog"),
    ("_struct",     "Binary packing — required by protocol parsing"),

    # ── Compression ───────────────────────────────────────
    ("zlib",        "Deflate compression — required by pip and package installs"),
    ("_bz2",       "Bzip2 compression — required by pandas I/O"),
    ("_lzma",      "LZMA compression — required by pandas I/O"),

    # ── Crypto & Networking ───────────────────────────────
    ("_ssl",        "TLS — required by httpx, nats-py, broker API clients"),
    ("_hashlib",    "Hashing — required by pip, security, and integrity checks"),
    ("_socket",     "Sockets — required by all network communication"),
    ("select",      "I/O multiplexing — required by async event loops"),

    # ── Database ──────────────────────────────────────────
    ("_sqlite3",    "SQLite — required by local caching and lightweight storage"),

    # ── Math & Science ────────────────────────────────────
    ("math",        "Math functions — required by strategy computations"),
    ("_random",     "Random number generation — required by simulations"),

    # ── System ────────────────────────────────────────────
    ("_multiprocessing", "Multiprocessing — required by Forge parallel workers"),
    ("_posixshmem",     "Shared memory — required by multiprocessing"),
    ("readline",        "Line editing — useful for interactive debugging"),
    ("_uuid",           "UUID generation — required by event IDs and tracing"),
]

def verify():
    missing = []
    print(f"Verifying {len(REQUIRED_EXTENSIONS)} stdlib extensions...")
    for module_name, reason in REQUIRED_EXTENSIONS:
        try:
            importlib.import_module(module_name)
            print(f"  OK  {module_name}")
        except ImportError as e:
            print(f"  FAIL  {module_name} — {reason}")
            print(f"        Error: {e}")
            missing.append((module_name, reason))

    print()
    if missing:
        print(f"FATAL: {len(missing)} required stdlib extension(s) missing:")
        for name, reason in missing:
            print(f"  - {name}: {reason}")
        print()
        print("Install the required system libraries and rebuild CPython.")
        sys.exit(1)
    else:
        print(f"All {len(REQUIRED_EXTENSIONS)} stdlib extensions verified.")

if __name__ == "__main__":
    verify()
