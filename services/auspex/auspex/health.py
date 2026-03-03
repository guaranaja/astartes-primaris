"""Health and metrics HTTP server for Auspex.

Exposes:
    GET /health  — readiness/liveness check
    GET /metrics — basic stats (bars written, ticks, buffer size, etc.)
"""

from __future__ import annotations

import json
import logging
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from threading import Thread
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .ibkr import IBKRConnector
    from .writer import LibrariumWriter
    from .vox import VoxPublisher

logger = logging.getLogger("auspex.health")

# Module-level references set by main
_ibkr: IBKRConnector | None = None
_writer: LibrariumWriter | None = None
_vox: VoxPublisher | None = None
_start_time: float = 0


def register(ibkr: IBKRConnector, writer: LibrariumWriter, vox: VoxPublisher) -> None:
    """Register components for health reporting."""
    global _ibkr, _writer, _vox, _start_time
    _ibkr = ibkr
    _writer = writer
    _vox = vox
    _start_time = time.time()


class _Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self._handle_health()
        elif self.path == "/metrics":
            self._handle_metrics()
        else:
            self.send_response(404)
            self.end_headers()

    def _handle_health(self):
        ibkr_ok = _ibkr.connected if _ibkr else False
        writer_ok = _writer is not None
        vox_ok = _vox.connected if _vox else False

        healthy = ibkr_ok and writer_ok
        status = 200 if healthy else 503

        body = {
            "status": "ok" if healthy else "degraded",
            "ibkr": "connected" if ibkr_ok else "disconnected",
            "librarium": "connected" if writer_ok else "disconnected",
            "vox": "connected" if vox_ok else "disconnected",
            "uptime_s": int(time.time() - _start_time) if _start_time else 0,
        }

        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def _handle_metrics(self):
        body = {
            "bars_written": _writer.bars_written if _writer else 0,
            "ticks_written": _writer.ticks_written if _writer else 0,
            "flush_count": _writer.flush_count if _writer else 0,
            "buffer_size": _writer.buffer_size if _writer else 0,
            "last_flush": _writer.last_flush if _writer else 0,
            "vox_messages": _vox.messages_published if _vox else 0,
            "ibkr_contracts": _ibkr.contract_count if _ibkr else 0,
            "ibkr_connected": _ibkr.connected if _ibkr else False,
        }

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def log_message(self, format, *args):
        # Suppress default HTTP log spam
        pass


def start_server(port: int) -> Thread:
    """Start the health server in a background thread."""
    server = HTTPServer(("0.0.0.0", port), _Handler)
    thread = Thread(target=server.serve_forever, daemon=True, name="auspex-health")
    thread.start()
    logger.info("Health server listening on :%d", port)
    return thread
