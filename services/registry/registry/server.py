"""
Astartes Strategy Registry — FastAPI Service
=============================================

Serves encrypted strategy bundles to authenticated clients.
Master publishes bundles after nightly pipeline; clients pull via catalog/download.

Endpoints:
    GET  /healthz                              — health check
    GET  /api/v1/catalog                       — list subscribed strategies
    GET  /api/v1/strategies/{id}/latest        — download latest bundle
    GET  /api/v1/strategies/{id}/versions      — list versions
    GET  /api/v1/strategies/{id}/versions/{v}  — download specific version
    POST /api/v1/publish                       — master uploads a bundle (internal auth)
    POST /api/v1/admin/clients                 — register a new client
    POST /api/v1/admin/clients/{id}/grant      — grant strategy access
    POST /api/v1/admin/clients/{id}/revoke     — revoke client
    GET  /api/v1/admin/audit                   — audit log

    --- Billing & P&L ---
    POST /api/v1/report/daily-pnl              — client engine reports daily P&L
    GET  /api/v1/billing/status                — client views their billing
    GET  /api/v1/admin/billing/overview        — admin revenue dashboard
    POST /api/v1/admin/billing/plans           — create/update pricing plans
    POST /api/v1/admin/billing/assign          — assign a client to a plan
    POST /api/v1/admin/billing/close-period    — close a monthly billing period
    POST /api/v1/admin/billing/mark-paid       — mark a period as paid
    POST /api/v1/admin/billing/verify-pnl      — cross-check P&L via broker API
"""

import hashlib
import hmac as hmac_mod
import json
import logging
import os
import secrets
from datetime import datetime, date, timezone
from decimal import Decimal
from pathlib import Path
from typing import Optional

import asyncpg
import bcrypt
from fastapi import Depends, FastAPI, Header, HTTPException, Request, UploadFile
from fastapi.responses import Response
from pydantic import BaseModel

from .config import Config

_log = logging.getLogger("registry")

app = FastAPI(title="Astartes Strategy Registry", version="1.0.0")

# ── Config (from environment via Config class) ──────────────

DATABASE_URL = Config.librarium.dsn
MASTER_AUTH_TOKEN = Config.registry.master_token
BUNDLES_DIR = Path(Config.registry.bundles_dir)
GCS_BUCKET = Config.registry.gcs_bucket

DISCORD_BOT_TOKEN = Config.discord.bot_token
DISCORD_CHANNEL_MARKETPLACE = Config.discord.channel_marketplace
DISCORD_CHANNEL_MARKETPLACE_ALERTS = Config.discord.channel_alerts

db_pool: Optional[asyncpg.Pool] = None


# ── Discord Admin Notifications ─────────────────────────────

import aiohttp as _aiohttp

_discord_session: Optional[_aiohttp.ClientSession] = None
_DISCORD_API = "https://discord.com/api/v10"


async def _discord_send(channel_id: str, embed: dict):
    """Send a Discord embed via bot token. Fire-and-forget, never raises."""
    if not DISCORD_BOT_TOKEN or not channel_id:
        return
    global _discord_session
    try:
        if _discord_session is None or _discord_session.closed:
            _discord_session = _aiohttp.ClientSession(
                timeout=_aiohttp.ClientTimeout(total=10),
                headers={
                    "Authorization": f"Bot {DISCORD_BOT_TOKEN}",
                    "Content-Type": "application/json",
                },
            )
        await _discord_session.post(
            f"{_DISCORD_API}/channels/{channel_id}/messages",
            json={"embeds": [embed]},
        )
    except Exception as e:
        _log.debug(f"Discord notify failed: {e}")


async def _notify_marketplace(title: str, description: str, color: int = 0x3498DB,
                              fields: list = None):
    """Send to #marketplace-admin channel."""
    embed = {
        "title": title,
        "description": description,
        "color": color,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "footer": {"text": "Astartes Registry"},
    }
    if fields:
        embed["fields"] = fields
    await _discord_send(DISCORD_CHANNEL_MARKETPLACE, embed)


async def _notify_marketplace_alert(title: str, description: str, color: int = 0xE74C3C,
                                    fields: list = None):
    """Send to #marketplace-alerts channel (integrity, payment failures, etc.)."""
    embed = {
        "title": title,
        "description": description,
        "color": color,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "footer": {"text": "Astartes Registry | ALERT"},
    }
    if fields:
        embed["fields"] = fields
    await _discord_send(
        DISCORD_CHANNEL_MARKETPLACE_ALERTS or DISCORD_CHANNEL_MARKETPLACE, embed
    )


# ── Lifecycle ────────────────────────────────────────────────

@app.on_event("startup")
async def startup():
    global db_pool
    db_pool = await asyncpg.create_pool(DATABASE_URL, min_size=2, max_size=10)
    BUNDLES_DIR.mkdir(parents=True, exist_ok=True)

    # Apply schema if tables don't exist
    async with db_pool.acquire() as conn:
        exists = await conn.fetchval(
            "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='registry_clients')"
        )
        if not exists:
            # Try multiple paths: Docker (/app/sql/), local dev (../../sql/)
            for schema_path in [
                Path("/app/registry_schema.sql"),
                Path(__file__).parent.parent / "registry_schema.sql",
            ]:
                if schema_path.exists():
                    break
            if schema_path.exists():
                await conn.execute(schema_path.read_text())
                _log.info("Registry schema applied")
            else:
                _log.warning(f"Schema not found at {schema_path}")
    _log.info("Registry started")


@app.on_event("shutdown")
async def shutdown():
    if db_pool:
        await db_pool.close()


# ── Auth helpers ─────────────────────────────────────────────

async def _get_client(api_key: str) -> Optional[dict]:
    """Look up a client by API key. Returns dict or None."""
    if not api_key or not db_pool:
        return None
    rows = await db_pool.fetch(
        "SELECT id, name, api_key_hash, public_key_pem, status FROM registry_clients WHERE status = 'ACTIVE'"
    )
    for row in rows:
        if bcrypt.checkpw(api_key.encode(), row["api_key_hash"].encode()):
            return dict(row)
    return None


async def require_client(authorization: str = Header(default="")) -> dict:
    """Dependency: extract and validate client API key from Bearer token."""
    if not authorization.startswith("Bearer "):
        raise HTTPException(401, "Missing or invalid Authorization header")
    api_key = authorization[7:]
    client = await _get_client(api_key)
    if not client:
        raise HTTPException(401, "Invalid API key")
    return client


def require_master(authorization: str = Header(default="")) -> None:
    """Dependency: validate master auth token."""
    if not MASTER_AUTH_TOKEN:
        raise HTTPException(500, "REGISTRY_MASTER_TOKEN not configured")
    if not authorization.startswith("Bearer "):
        raise HTTPException(401, "Missing Authorization header")
    if authorization[7:] != MASTER_AUTH_TOKEN:
        raise HTTPException(403, "Invalid master token")


async def _audit(client_id, strategy_id, version, action, detail=None, ip=None):
    """Write an audit log entry."""
    if db_pool:
        await db_pool.execute(
            """INSERT INTO registry_audit_log (client_id, strategy_id, version, action, detail, ip_address)
               VALUES ($1, $2, $3, $4, $5, $6)""",
            client_id, strategy_id, version, action, detail, ip,
        )


# ── Health ───────────────────────────────────────────────────

@app.get("/healthz")
async def healthz():
    return {"status": "ok", "service": "registry"}


# ── Catalog ──────────────────────────────────────────────────

@app.get("/api/v1/catalog")
async def catalog(client: dict = Depends(require_client)):
    """List strategies the client is subscribed to."""
    rows = await db_pool.fetch(
        """SELECT s.id, s.display_name, s.description, s.tier,
                  v.version AS latest_version, v.published_at AS latest_published,
                  v.bundle_size, v.compiled
           FROM registry_subscriptions sub
           JOIN registry_strategies s ON s.id = sub.strategy_id
           LEFT JOIN LATERAL (
               SELECT version, published_at, bundle_size, compiled
               FROM registry_versions
               WHERE strategy_id = s.id
               ORDER BY published_at DESC LIMIT 1
           ) v ON TRUE
           WHERE sub.client_id = $1 AND sub.active = TRUE""",
        client["id"],
    )
    await _audit(client["id"], None, None, "CATALOG", ip=None)
    return {
        "strategies": [
            {
                "id": r["id"],
                "display_name": r["display_name"],
                "description": r["description"],
                "tier": r["tier"],
                "latest_version": r["latest_version"],
                "latest_published": r["latest_published"].isoformat() if r["latest_published"] else None,
                "bundle_size": r["bundle_size"],
                "compiled": r["compiled"],
            }
            for r in rows
        ]
    }


# ── Download ─────────────────────────────────────────────────

async def _get_bundle_bytes(bundle_path: str) -> bytes:
    """Read bundle from local disk or GCS."""
    p = Path(bundle_path)
    if p.exists():
        return p.read_bytes()
    if bundle_path.startswith("gs://") and GCS_BUCKET:
        from google.cloud import storage as gcs
        client = gcs.Client()
        # gs://bucket/path -> bucket, path
        parts = bundle_path[5:].split("/", 1)
        bucket = client.bucket(parts[0])
        blob = bucket.blob(parts[1])
        return blob.download_as_bytes()
    raise FileNotFoundError(f"Bundle not found: {bundle_path}")


@app.get("/api/v1/strategies/{strategy_id}/latest")
async def download_latest(strategy_id: str, client: dict = Depends(require_client)):
    """Download the latest bundle for a strategy."""
    # Check subscription
    sub = await db_pool.fetchrow(
        "SELECT 1 FROM registry_subscriptions WHERE client_id=$1 AND strategy_id=$2 AND active=TRUE",
        client["id"], strategy_id,
    )
    if not sub:
        raise HTTPException(403, "Not subscribed to this strategy")

    row = await db_pool.fetchrow(
        """SELECT version, bundle_path, manifest_hash
           FROM registry_versions
           WHERE strategy_id = $1
           ORDER BY published_at DESC LIMIT 1""",
        strategy_id,
    )
    if not row:
        raise HTTPException(404, "No versions published for this strategy")

    try:
        bundle_bytes = await _get_bundle_bytes(row["bundle_path"])
    except FileNotFoundError:
        raise HTTPException(404, "Bundle file not found")

    await _audit(client["id"], strategy_id, row["version"], "DOWNLOAD")
    return Response(
        content=bundle_bytes,
        media_type="application/octet-stream",
        headers={
            "X-Bundle-Version": row["version"],
            "X-Bundle-Hash": row["manifest_hash"],
            "Content-Disposition": f'attachment; filename="{strategy_id}_v{row["version"]}.astartes"',
        },
    )


@app.get("/api/v1/strategies/{strategy_id}/versions")
async def list_versions(strategy_id: str, client: dict = Depends(require_client)):
    """List all versions for a strategy."""
    sub = await db_pool.fetchrow(
        "SELECT 1 FROM registry_subscriptions WHERE client_id=$1 AND strategy_id=$2 AND active=TRUE",
        client["id"], strategy_id,
    )
    if not sub:
        raise HTTPException(403, "Not subscribed to this strategy")

    rows = await db_pool.fetch(
        """SELECT version, changelog, published_at, bundle_size, compiled, manifest_hash
           FROM registry_versions WHERE strategy_id = $1
           ORDER BY published_at DESC LIMIT 50""",
        strategy_id,
    )
    return {
        "strategy_id": strategy_id,
        "versions": [
            {
                "version": r["version"],
                "changelog": r["changelog"],
                "published_at": r["published_at"].isoformat(),
                "bundle_size": r["bundle_size"],
                "compiled": r["compiled"],
                "hash": r["manifest_hash"],
            }
            for r in rows
        ],
    }


@app.get("/api/v1/strategies/{strategy_id}/versions/{version}")
async def download_version(strategy_id: str, version: str, client: dict = Depends(require_client)):
    """Download a specific version bundle."""
    sub = await db_pool.fetchrow(
        "SELECT 1 FROM registry_subscriptions WHERE client_id=$1 AND strategy_id=$2 AND active=TRUE",
        client["id"], strategy_id,
    )
    if not sub:
        raise HTTPException(403, "Not subscribed to this strategy")

    row = await db_pool.fetchrow(
        "SELECT bundle_path, manifest_hash FROM registry_versions WHERE strategy_id=$1 AND version=$2",
        strategy_id, version,
    )
    if not row:
        raise HTTPException(404, "Version not found")

    try:
        bundle_bytes = await _get_bundle_bytes(row["bundle_path"])
    except FileNotFoundError:
        raise HTTPException(404, "Bundle file not found")

    await _audit(client["id"], strategy_id, version, "DOWNLOAD")
    return Response(
        content=bundle_bytes,
        media_type="application/octet-stream",
        headers={
            "X-Bundle-Version": version,
            "X-Bundle-Hash": row["manifest_hash"],
        },
    )


# ── Publish (master-only) ───────────────────────────────────

class PublishRequest(BaseModel):
    strategy_id: str
    version: str
    display_name: str = ""
    description: str = ""
    changelog: str = ""
    compiled: bool = False


@app.post("/api/v1/publish")
async def publish(request: Request, _: None = Depends(require_master)):
    """Receive a bundle upload from the master pipeline.

    Expects multipart form:
        - metadata: JSON string with strategy_id, version, etc.
        - bundle: the .astartes file bytes
    """
    form = await request.form()

    metadata_raw = form.get("metadata")
    if not metadata_raw:
        raise HTTPException(400, "Missing 'metadata' form field")
    metadata = json.loads(metadata_raw)

    bundle_file = form.get("bundle")
    if not bundle_file:
        raise HTTPException(400, "Missing 'bundle' form field")
    bundle_bytes = await bundle_file.read()

    strategy_id = metadata.get("strategy_id", "")
    version = metadata.get("version", "")
    if not strategy_id or not version:
        raise HTTPException(400, "strategy_id and version required in metadata")

    # Ensure strategy exists
    exists = await db_pool.fetchval(
        "SELECT 1 FROM registry_strategies WHERE id = $1", strategy_id
    )
    if not exists:
        await db_pool.execute(
            """INSERT INTO registry_strategies (id, display_name, description)
               VALUES ($1, $2, $3)""",
            strategy_id,
            metadata.get("display_name", strategy_id),
            metadata.get("description", ""),
        )
        _log.info(f"Created strategy: {strategy_id}")

    # Write bundle to disk
    bundle_filename = f"{strategy_id}_v{version}.astartes"
    bundle_path = BUNDLES_DIR / bundle_filename
    bundle_path.write_bytes(bundle_bytes)

    # Optionally upload to GCS
    gcs_path = None
    if GCS_BUCKET:
        try:
            from google.cloud import storage as gcs
            client = gcs.Client()
            bucket = client.bucket(GCS_BUCKET)
            blob = bucket.blob(f"bundles/{bundle_filename}")
            blob.upload_from_string(bundle_bytes, content_type="application/octet-stream")
            gcs_path = f"gs://{GCS_BUCKET}/bundles/{bundle_filename}"
            _log.info(f"Uploaded to GCS: {gcs_path}")
        except Exception as e:
            _log.warning(f"GCS upload failed (using local path): {e}")

    manifest_hash = hashlib.sha256(bundle_bytes).hexdigest()
    store_path = gcs_path or str(bundle_path)

    # Upsert version
    await db_pool.execute(
        """INSERT INTO registry_versions (strategy_id, version, changelog, bundle_path, manifest_hash, bundle_size, compiled)
           VALUES ($1, $2, $3, $4, $5, $6, $7)
           ON CONFLICT (strategy_id, version) DO UPDATE
           SET bundle_path = EXCLUDED.bundle_path,
               manifest_hash = EXCLUDED.manifest_hash,
               bundle_size = EXCLUDED.bundle_size,
               compiled = EXCLUDED.compiled,
               published_at = NOW()""",
        strategy_id, version, metadata.get("changelog", ""),
        store_path, manifest_hash, len(bundle_bytes),
        metadata.get("compiled", False),
    )

    await _audit(None, strategy_id, version, "PUBLISH",
                 detail=f"{len(bundle_bytes)} bytes, compiled={metadata.get('compiled')}")

    _log.info(f"Published: {strategy_id} v{version} ({len(bundle_bytes):,} bytes)")
    return {
        "published": True,
        "strategy_id": strategy_id,
        "version": version,
        "bundle_size": len(bundle_bytes),
        "path": store_path,
    }


# ── Admin: Client Management ────────────────────────────────

class RegisterClientRequest(BaseModel):
    name: str
    contact_email: str = ""
    public_key_pem: str  # Client's X25519 public key


@app.post("/api/v1/admin/clients")
async def register_client(body: RegisterClientRequest, _: None = Depends(require_master)):
    """Register a new client and return their API key (shown once)."""
    api_key = secrets.token_urlsafe(32)
    api_key_hash = bcrypt.hashpw(api_key.encode(), bcrypt.gensalt()).decode()

    client_id = await db_pool.fetchval(
        """INSERT INTO registry_clients (name, contact_email, api_key_hash, public_key_pem)
           VALUES ($1, $2, $3, $4) RETURNING id""",
        body.name, body.contact_email, api_key_hash, body.public_key_pem,
    )

    await _audit(client_id, None, None, "REGISTER", detail=f"name={body.name}")
    _log.info(f"Registered client: {body.name} ({client_id})")

    await _notify_marketplace(
        "New Client Registered",
        f"**{body.name}** has been registered.",
        color=0x2ECC71,
        fields=[
            {"name": "Client ID", "value": str(client_id), "inline": True},
            {"name": "Email", "value": body.contact_email or "—", "inline": True},
        ],
    )

    return {
        "client_id": str(client_id),
        "name": body.name,
        "api_key": api_key,  # Shown once — client must save this
    }


class GrantRequest(BaseModel):
    strategy_id: str


@app.post("/api/v1/admin/clients/{client_id}/grant")
async def grant_strategy(client_id: str, body: GrantRequest, _: None = Depends(require_master)):
    """Grant a client access to a strategy."""
    await db_pool.execute(
        """INSERT INTO registry_subscriptions (client_id, strategy_id, active)
           VALUES ($1, $2, TRUE)
           ON CONFLICT (client_id, strategy_id) DO UPDATE SET active = TRUE, revoked_at = NULL""",
        client_id, body.strategy_id,
    )
    await _audit(client_id, body.strategy_id, None, "GRANT")
    _log.info(f"Granted {body.strategy_id} to {client_id}")
    return {"granted": True, "client_id": client_id, "strategy_id": body.strategy_id}


@app.post("/api/v1/admin/clients/{client_id}/revoke")
async def revoke_client(client_id: str, _: None = Depends(require_master)):
    """Revoke a client permanently — they can no longer sync or download."""
    await db_pool.execute(
        "UPDATE registry_clients SET status='REVOKED', revoked_at=NOW() WHERE id=$1",
        client_id,
    )
    await db_pool.execute(
        "UPDATE registry_subscriptions SET active=FALSE, revoked_at=NOW() WHERE client_id=$1",
        client_id,
    )
    await db_pool.execute(
        "UPDATE client_billing SET status='CANCELLED', cancelled_at=NOW() WHERE client_id=$1",
        client_id,
    )
    await _audit(client_id, None, None, "REVOKE")
    _log.info(f"Revoked client: {client_id}")
    return {"revoked": True, "client_id": client_id}


@app.post("/api/v1/admin/clients/{client_id}/suspend")
async def suspend_client(client_id: str, _: None = Depends(require_master)):
    """Suspend a client — bundle delivery stops, but can be resumed.

    Use for: missed payments, pausing service, temporary holds.
    Client's engine will keep running the last-downloaded bundle until it expires.
    """
    await db_pool.execute(
        "UPDATE registry_clients SET status='SUSPENDED' WHERE id=$1",
        client_id,
    )
    await db_pool.execute(
        "UPDATE registry_subscriptions SET active=FALSE WHERE client_id=$1",
        client_id,
    )
    await db_pool.execute(
        "UPDATE client_billing SET status='SUSPENDED' WHERE client_id=$1 AND status='ACTIVE'",
        client_id,
    )
    await _audit(client_id, None, None, "SUSPEND")
    _log.info(f"Suspended client: {client_id}")

    await _notify_marketplace_alert(
        "Client Suspended",
        f"Client `{client_id}` has been suspended. Bundle delivery stopped.",
        color=0xE67E22,
    )

    return {"suspended": True, "client_id": client_id}


@app.post("/api/v1/admin/clients/{client_id}/resume")
async def resume_client(client_id: str, _: None = Depends(require_master)):
    """Resume a suspended client — restores bundle delivery."""
    await db_pool.execute(
        "UPDATE registry_clients SET status='ACTIVE' WHERE id=$1 AND status='SUSPENDED'",
        client_id,
    )
    await db_pool.execute(
        "UPDATE registry_subscriptions SET active=TRUE, revoked_at=NULL WHERE client_id=$1",
        client_id,
    )
    await db_pool.execute(
        "UPDATE client_billing SET status='ACTIVE' WHERE client_id=$1 AND status='SUSPENDED'",
        client_id,
    )
    await _audit(client_id, None, None, "RESUME")
    _log.info(f"Resumed client: {client_id}")
    return {"resumed": True, "client_id": client_id}


@app.get("/api/v1/admin/clients")
async def list_clients(_: None = Depends(require_master)):
    """List all registered clients."""
    rows = await db_pool.fetch(
        """SELECT c.id, c.name, c.contact_email, c.status, c.created_at,
                  COUNT(sub.strategy_id) FILTER (WHERE sub.active) AS active_subs
           FROM registry_clients c
           LEFT JOIN registry_subscriptions sub ON sub.client_id = c.id
           GROUP BY c.id ORDER BY c.created_at DESC"""
    )
    return {
        "clients": [
            {
                "id": str(r["id"]),
                "name": r["name"],
                "contact_email": r["contact_email"],
                "status": r["status"],
                "created_at": r["created_at"].isoformat(),
                "active_subscriptions": r["active_subs"],
            }
            for r in rows
        ]
    }


@app.get("/api/v1/admin/audit")
async def audit_log(
    limit: int = 50,
    client_id: Optional[str] = None,
    strategy_id: Optional[str] = None,
    _: None = Depends(require_master),
):
    """Query the audit log."""
    query = "SELECT * FROM registry_audit_log WHERE 1=1"
    params = []
    idx = 1

    if client_id:
        query += f" AND client_id = ${idx}"
        params.append(client_id)
        idx += 1
    if strategy_id:
        query += f" AND strategy_id = ${idx}"
        params.append(strategy_id)
        idx += 1

    query += f" ORDER BY timestamp DESC LIMIT ${idx}"
    params.append(limit)

    rows = await db_pool.fetch(query, *params)
    return {
        "events": [
            {
                "id": r["id"],
                "client_id": str(r["client_id"]) if r["client_id"] else None,
                "strategy_id": r["strategy_id"],
                "version": r["version"],
                "action": r["action"],
                "detail": r["detail"],
                "ip_address": str(r["ip_address"]) if r["ip_address"] else None,
                "timestamp": r["timestamp"].isoformat(),
            }
            for r in rows
        ]
    }


# ── P&L Reporting (client engine → registry) ───────────────

PNL_REPORT_SECRET = Config.integrity.hmac_secret


class PnlReport(BaseModel):
    strategy_id: str
    trade_date: str  # YYYY-MM-DD
    gross_pnl: float
    net_pnl: float
    trade_count: int = 0
    ending_equity: float = 0.0
    report_hash: str = ""  # HMAC-SHA256 for tamper detection


def _verify_report_hmac(report: PnlReport, client_id: str) -> bool:
    """Verify HMAC if secret is configured. Skip for F&F phase."""
    if not PNL_REPORT_SECRET or not report.report_hash:
        return True
    payload = json.dumps({
        "client_id": client_id,
        "strategy_id": report.strategy_id,
        "trade_date": report.trade_date,
        "gross_pnl": report.gross_pnl,
        "net_pnl": report.net_pnl,
        "trade_count": report.trade_count,
        "ending_equity": report.ending_equity,
    }, sort_keys=True)
    expected = hmac_mod.new(
        PNL_REPORT_SECRET.encode(), payload.encode(), "sha256"
    ).hexdigest()
    return hmac_mod.compare_digest(expected, report.report_hash)


@app.post("/api/v1/report/daily-pnl")
async def report_daily_pnl(report: PnlReport, client: dict = Depends(require_client)):
    """Client engine submits daily P&L. Called automatically at EOD."""
    if not _verify_report_hmac(report, str(client["id"])):
        raise HTTPException(400, "Invalid report HMAC")

    trade_date = date.fromisoformat(report.trade_date)

    await db_pool.execute(
        """INSERT INTO pnl_reports
               (client_id, strategy_id, trade_date, gross_pnl, net_pnl,
                trade_count, ending_equity, report_hash)
           VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
           ON CONFLICT (client_id, strategy_id, trade_date) DO UPDATE SET
               gross_pnl = EXCLUDED.gross_pnl,
               net_pnl = EXCLUDED.net_pnl,
               trade_count = EXCLUDED.trade_count,
               ending_equity = EXCLUDED.ending_equity,
               report_hash = EXCLUDED.report_hash,
               reported_at = NOW()""",
        client["id"], report.strategy_id, trade_date,
        report.gross_pnl, report.net_pnl, report.trade_count,
        report.ending_equity, report.report_hash,
    )

    # Update high-water mark if equity is a new peak
    if report.ending_equity > 0:
        await db_pool.execute(
            """UPDATE client_billing
               SET hwm_equity = GREATEST(hwm_equity, $1)
               WHERE client_id = $2 AND status = 'ACTIVE'""",
            report.ending_equity, client["id"],
        )

    await _audit(client["id"], report.strategy_id, None, "PNL_REPORT",
                 detail=f"date={report.trade_date} net={report.net_pnl:+.2f} trades={report.trade_count}")

    _log.info(f"P&L report: {client['name']} {report.strategy_id} "
              f"{report.trade_date} net={report.net_pnl:+.2f}")
    return {"accepted": True, "trade_date": report.trade_date}


# ── Bidirectional Sync (client pushes P&L, pulls catalog + billing) ──

class SyncPnlEntry(BaseModel):
    strategy_id: str
    trade_date: str      # YYYY-MM-DD
    gross_pnl: float = 0
    net_pnl: float = 0
    trade_count: int = 0
    ending_equity: float = 0
    report_hash: str = ""


class SyncRequest(BaseModel):
    pnl_reports: list[SyncPnlEntry] = []
    env_meta: dict = {}  # runtime environment metadata


@app.post("/api/v1/sync")
async def sync(body: SyncRequest, client: dict = Depends(require_client)):
    """Bidirectional sync: client pushes P&L, receives catalog + billing summary.

    Called by client engine on each sync cycle (hourly default).
    Replaces separate calls to /catalog + /report/daily-pnl.
    """
    client_id = client["id"]

    # ── 1. Store P&L reports ──
    pnl_stored = 0
    for report in body.pnl_reports:
        try:
            trade_date = date.fromisoformat(report.trade_date)
            await db_pool.execute(
                """INSERT INTO pnl_reports
                       (client_id, strategy_id, trade_date, gross_pnl, net_pnl,
                        trade_count, ending_equity, report_hash)
                   VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
                   ON CONFLICT (client_id, strategy_id, trade_date) DO UPDATE SET
                       gross_pnl = EXCLUDED.gross_pnl,
                       net_pnl = EXCLUDED.net_pnl,
                       trade_count = EXCLUDED.trade_count,
                       ending_equity = EXCLUDED.ending_equity,
                       report_hash = EXCLUDED.report_hash,
                       reported_at = NOW()""",
                client_id, report.strategy_id, trade_date,
                report.gross_pnl, report.net_pnl, report.trade_count,
                report.ending_equity, report.report_hash,
            )
            pnl_stored += 1

            # Update HWM
            if report.ending_equity > 0:
                await db_pool.execute(
                    """UPDATE client_billing
                       SET hwm_equity = GREATEST(hwm_equity, $1)
                       WHERE client_id = $2 AND status = 'ACTIVE'""",
                    report.ending_equity, client_id,
                )
        except Exception as e:
            _log.warning(f"Sync P&L store error for {report.trade_date}: {e}")

    if pnl_stored:
        _log.info(f"Sync: stored {pnl_stored} P&L reports from {client['name']}")

    # ── 2. Get catalog (subscribed strategies + latest versions) ──
    catalog_rows = await db_pool.fetch(
        """SELECT s.id, s.display_name, v.version AS latest_version,
                  v.published_at, v.bundle_size
           FROM registry_subscriptions sub
           JOIN registry_strategies s ON s.id = sub.strategy_id
           LEFT JOIN LATERAL (
               SELECT version, published_at, bundle_size
               FROM registry_versions
               WHERE strategy_id = s.id
               ORDER BY published_at DESC LIMIT 1
           ) v ON TRUE
           WHERE sub.client_id = $1 AND sub.active = TRUE""",
        client_id,
    )

    strategies = [
        {
            "id": r["id"],
            "display_name": r["display_name"],
            "latest_version": r["latest_version"],
            "published_at": r["published_at"].isoformat() if r["published_at"] else None,
            "bundle_size": r["bundle_size"],
        }
        for r in catalog_rows
    ]

    # ── 3. Calculate billing summary ──
    billing = None
    plan_row = await db_pool.fetchrow(
        """SELECT cb.plan_id, bp.name AS plan_name, bp.flat_rate, bp.perf_pct,
                  cb.hwm_equity, cb.started_at
           FROM client_billing cb
           JOIN billing_plans bp ON bp.id = cb.plan_id
           WHERE cb.client_id = $1 AND cb.status = 'ACTIVE'
           LIMIT 1""",
        client_id,
    )

    if plan_row:
        # Current month P&L
        month_pnl = await db_pool.fetchrow(
            """SELECT COALESCE(SUM(net_pnl), 0) AS net_pnl,
                      COALESCE(SUM(trade_count), 0) AS trades,
                      COUNT(*) AS trading_days,
                      MAX(ending_equity) AS peak_equity
               FROM pnl_reports
               WHERE client_id = $1
                 AND trade_date >= date_trunc('month', CURRENT_DATE)""",
            client_id,
        )

        # Today's P&L
        today_pnl = await db_pool.fetchrow(
            """SELECT COALESCE(SUM(net_pnl), 0) AS net_pnl,
                      COALESCE(SUM(trade_count), 0) AS trades
               FROM pnl_reports
               WHERE client_id = $1 AND trade_date = CURRENT_DATE""",
            client_id,
        )

        # Pro-rated flat fee
        now = datetime.now(timezone.utc)
        days_in_month = 30  # simplified
        day_of_month = now.day
        accrued_flat = round(float(plan_row["flat_rate"]) * day_of_month / days_in_month, 2)

        # Running performance fee
        hwm = float(plan_row["hwm_equity"])
        peak = float(month_pnl["peak_equity"] or 0)
        perf_pct = float(plan_row["perf_pct"])
        perf_fee = 0.0
        if peak > hwm and perf_pct > 0:
            perf_fee = round((peak - hwm) * perf_pct / 100, 2)

        billing = {
            "plan": plan_row["plan_name"],
            "flat_rate": float(plan_row["flat_rate"]),
            "perf_pct": perf_pct,
            "hwm_equity": hwm,
            "today_pnl": float(today_pnl["net_pnl"]),
            "today_trades": today_pnl["trades"],
            "month_pnl": float(month_pnl["net_pnl"]),
            "month_trades": month_pnl["trades"],
            "month_trading_days": month_pnl["trading_days"],
            "peak_equity": peak,
            "accrued_flat_fee": accrued_flat,
            "running_perf_fee": perf_fee,
            "running_total_due": accrued_flat + perf_fee,
        }

    # ── 4. Silent integrity check ──
    if body.env_meta:
        await _check_client_integrity(client_id, client["name"], body.env_meta)

    await _audit(client_id, None, None, "SYNC",
                 detail=f"pnl={pnl_stored} strategies={len(strategies)}")

    return {
        "synced": True,
        "pnl_stored": pnl_stored,
        "strategies": strategies,
        "billing": billing,
    }


# ── Silent Integrity Monitoring ────────────────────────────

# Expected file hashes for the current client release.
# Updated each time a new client version is published.
# If empty, integrity checking is disabled (learning mode).
EXPECTED_CLIENT_HASHES: dict = json.loads(Config.integrity.expected_hashes)


async def _check_client_integrity(client_id, client_name: str, env_meta: dict):
    """Compare client file hashes against expected values. Silent — never surfaces to client."""
    file_hashes = env_meta.get("fh", {})
    runtime = env_meta.get("rt", "")
    os_platform = env_meta.get("os", "")

    if not file_hashes or not db_pool:
        return

    status = "OK"
    detail = None

    if EXPECTED_CLIENT_HASHES:
        mismatches = []
        for fname, expected_hash in EXPECTED_CLIENT_HASHES.items():
            actual = file_hashes.get(fname)
            if actual is None:
                mismatches.append(f"{fname}: MISSING")
            elif actual != expected_hash:
                mismatches.append(f"{fname}: {actual} != {expected_hash}")

        # Files present in client but not in expected = new files (suspicious but not conclusive)
        unexpected = set(file_hashes.keys()) - set(EXPECTED_CLIENT_HASHES.keys())

        if mismatches:
            status = "MODIFIED"
            detail = "; ".join(mismatches)
            if unexpected:
                detail += f" | unexpected: {','.join(unexpected)}"

            _log.warning(f"INTEGRITY [{client_name}]: {status} — {detail}")

            # Record as audit event (admin-visible, client-invisible)
            await _audit(client_id, None, None, "INTEGRITY_VIOLATION",
                         detail=f"{status}: {detail}")

            await _notify_marketplace_alert(
                "Integrity Violation",
                f"**{client_name}** has modified client files.",
                fields=[
                    {"name": "Status", "value": status, "inline": True},
                    {"name": "Modified", "value": detail[:200], "inline": False},
                ],
            )

    # Upsert latest check
    hashes_json = json.dumps(file_hashes)
    await db_pool.execute(
        """INSERT INTO client_integrity (client_id, file_hashes, runtime, os_platform, status, detail)
           VALUES ($1, $2::jsonb, $3, $4, $5, $6)
           ON CONFLICT (client_id) DO UPDATE SET
               checked_at = NOW(),
               file_hashes = EXCLUDED.file_hashes,
               runtime = EXCLUDED.runtime,
               os_platform = EXCLUDED.os_platform,
               status = EXCLUDED.status,
               detail = EXCLUDED.detail""",
        client_id, hashes_json, runtime, os_platform, status, detail,
    )


# ── Client Billing Status ──────────────────────────────────

@app.get("/api/v1/billing/status")
async def billing_status(client: dict = Depends(require_client)):
    """Client views their billing status, recent P&L, and upcoming fees."""
    # Active plans
    plans = await db_pool.fetch(
        """SELECT cb.plan_id, bp.name AS plan_name, bp.flat_rate, bp.perf_pct,
                  cb.status, cb.hwm_equity, cb.started_at
           FROM client_billing cb
           JOIN billing_plans bp ON bp.id = cb.plan_id
           WHERE cb.client_id = $1 AND cb.status = 'ACTIVE'""",
        client["id"],
    )

    # Recent P&L (last 30 days)
    recent_pnl = await db_pool.fetch(
        """SELECT trade_date, strategy_id, net_pnl, trade_count, ending_equity, verified
           FROM pnl_reports
           WHERE client_id = $1 AND trade_date >= CURRENT_DATE - 30
           ORDER BY trade_date DESC""",
        client["id"],
    )

    # Current month summary
    month_summary = await db_pool.fetchrow(
        """SELECT COALESCE(SUM(net_pnl), 0) AS month_pnl,
                  COALESCE(SUM(trade_count), 0) AS month_trades,
                  COUNT(*) AS trading_days
           FROM pnl_reports
           WHERE client_id = $1
             AND trade_date >= date_trunc('month', CURRENT_DATE)""",
        client["id"],
    )

    # Recent billing periods
    periods = await db_pool.fetch(
        """SELECT period_start, period_end, flat_fee, perf_fee, net_pnl, status, notes
           FROM billing_periods
           WHERE client_id = $1
           ORDER BY period_start DESC LIMIT 6""",
        client["id"],
    )

    return {
        "client": client["name"],
        "plans": [
            {
                "plan": r["plan_name"],
                "flat_rate": float(r["flat_rate"]),
                "perf_pct": float(r["perf_pct"]),
                "hwm_equity": float(r["hwm_equity"]),
                "status": r["status"],
                "since": r["started_at"].isoformat(),
            }
            for r in plans
        ],
        "current_month": {
            "net_pnl": float(month_summary["month_pnl"]),
            "trades": month_summary["month_trades"],
            "trading_days": month_summary["trading_days"],
        },
        "recent_pnl": [
            {
                "date": r["trade_date"].isoformat(),
                "strategy": r["strategy_id"],
                "net_pnl": float(r["net_pnl"]),
                "trades": r["trade_count"],
                "equity": float(r["ending_equity"]) if r["ending_equity"] else None,
                "verified": r["verified"],
            }
            for r in recent_pnl
        ],
        "billing_history": [
            {
                "period": f"{r['period_start']} to {r['period_end']}",
                "flat_fee": float(r["flat_fee"]) if r["flat_fee"] else None,
                "perf_fee": float(r["perf_fee"]) if r["perf_fee"] else None,
                "net_pnl": float(r["net_pnl"]) if r["net_pnl"] else None,
                "status": r["status"],
                "notes": r["notes"],
            }
            for r in periods
        ],
    }


# ── Admin: Billing ──────────────────────────────────────────

class CreatePlanRequest(BaseModel):
    id: str              # e.g. "eversor_standard"
    strategy_id: str
    name: str
    flat_rate: float     # $/month
    perf_pct: float = 0  # % of profits above HWM
    description: str = ""
    offsets: dict = {}   # tier-based config offsets for liquidity sandbagging
                         # e.g. {"signal_delay_bars": 1, "entry_jitter_ms": [500, 1500],
                         #       "max_contracts": 5, "entry_threshold_offset": 0.05}


@app.post("/api/v1/admin/billing/plans")
async def create_or_update_plan(body: CreatePlanRequest, _: None = Depends(require_master)):
    """Create or update a billing plan.

    The `offsets` field controls liquidity sandbagging — clients on cheaper
    tiers get more delay/jitter so they don't compete with the master for fills.
    These offsets are merged into the encrypted config at publish time.
    """
    offsets_json = json.dumps(body.offsets) if body.offsets else "{}"
    await db_pool.execute(
        """INSERT INTO billing_plans (id, strategy_id, name, flat_rate, perf_pct, description, offsets)
           VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
           ON CONFLICT (id) DO UPDATE SET
               name = EXCLUDED.name,
               flat_rate = EXCLUDED.flat_rate,
               perf_pct = EXCLUDED.perf_pct,
               description = EXCLUDED.description,
               offsets = EXCLUDED.offsets""",
        body.id, body.strategy_id, body.name, body.flat_rate, body.perf_pct,
        body.description, offsets_json,
    )
    _log.info(f"Plan upserted: {body.id} (${body.flat_rate}/mo + {body.perf_pct}% offsets={body.offsets})")
    return {"plan_id": body.id, "flat_rate": body.flat_rate, "perf_pct": body.perf_pct, "offsets": body.offsets}


@app.get("/api/v1/admin/billing/plans")
async def list_plans(_: None = Depends(require_master)):
    """List all billing plans."""
    rows = await db_pool.fetch(
        """SELECT bp.*, COUNT(cb.client_id) FILTER (WHERE cb.status = 'ACTIVE') AS active_clients
           FROM billing_plans bp
           LEFT JOIN client_billing cb ON cb.plan_id = bp.id
           GROUP BY bp.id ORDER BY bp.created_at"""
    )
    return {
        "plans": [
            {
                "id": r["id"],
                "strategy_id": r["strategy_id"],
                "name": r["name"],
                "flat_rate": float(r["flat_rate"]),
                "perf_pct": float(r["perf_pct"]),
                "description": r["description"],
                "offsets": json.loads(r["offsets"]) if r["offsets"] else {},
                "active_clients": r["active_clients"],
                "active": r["active"],
            }
            for r in rows
        ]
    }


class AssignPlanRequest(BaseModel):
    client_id: str
    plan_id: str
    starting_equity: float = 0  # initial HWM (usually account starting balance)
    notes: str = ""


@app.post("/api/v1/admin/billing/assign")
async def assign_plan(body: AssignPlanRequest, _: None = Depends(require_master)):
    """Assign a client to a billing plan (also grants strategy access)."""
    # Validate plan exists
    plan = await db_pool.fetchrow("SELECT * FROM billing_plans WHERE id = $1", body.plan_id)
    if not plan:
        raise HTTPException(404, f"Plan {body.plan_id} not found")

    # Upsert billing record
    await db_pool.execute(
        """INSERT INTO client_billing (client_id, plan_id, hwm_equity, notes)
           VALUES ($1, $2, $3, $4)
           ON CONFLICT (client_id, plan_id) DO UPDATE SET
               status = 'ACTIVE',
               hwm_equity = EXCLUDED.hwm_equity,
               notes = EXCLUDED.notes,
               cancelled_at = NULL""",
        body.client_id, body.plan_id, body.starting_equity, body.notes,
    )

    # Auto-grant strategy access
    await db_pool.execute(
        """INSERT INTO registry_subscriptions (client_id, strategy_id, active)
           VALUES ($1, $2, TRUE)
           ON CONFLICT (client_id, strategy_id) DO UPDATE SET active = TRUE, revoked_at = NULL""",
        body.client_id, plan["strategy_id"],
    )

    await _audit(body.client_id, plan["strategy_id"], None, "BILLING_ASSIGN",
                 detail=f"plan={body.plan_id} hwm={body.starting_equity}")
    _log.info(f"Assigned {body.client_id} to plan {body.plan_id}")

    await _notify_marketplace(
        "Client Assigned to Plan",
        f"Client `{body.client_id[:8]}…` → **{plan['name']}** (${float(plan['flat_rate']):,.0f}/mo + {float(plan['perf_pct']):.0f}%)",
        color=0x2ECC71,
        fields=[
            {"name": "Starting Equity", "value": f"${body.starting_equity:,.2f}", "inline": True},
            {"name": "Strategy", "value": plan["strategy_id"], "inline": True},
            {"name": "Notes", "value": body.notes or "—", "inline": False},
        ],
    )

    return {"assigned": True, "client_id": body.client_id, "plan_id": body.plan_id}


@app.get("/api/v1/admin/billing/overview")
async def billing_overview(_: None = Depends(require_master)):
    """Admin revenue dashboard — all clients, plans, P&L, fees."""
    # Per-client summary
    clients = await db_pool.fetch(
        """SELECT c.id, c.name, c.status AS client_status,
                  cb.plan_id, bp.name AS plan_name, bp.flat_rate, bp.perf_pct,
                  cb.status AS billing_status, cb.hwm_equity, cb.notes,
                  -- Current month P&L
                  (SELECT COALESCE(SUM(net_pnl), 0) FROM pnl_reports
                   WHERE client_id = c.id
                     AND trade_date >= date_trunc('month', CURRENT_DATE)) AS month_pnl,
                  -- All-time P&L
                  (SELECT COALESCE(SUM(net_pnl), 0) FROM pnl_reports
                   WHERE client_id = c.id) AS total_pnl,
                  -- Last report date
                  (SELECT MAX(trade_date) FROM pnl_reports
                   WHERE client_id = c.id) AS last_report
           FROM registry_clients c
           LEFT JOIN client_billing cb ON cb.client_id = c.id
           LEFT JOIN billing_plans bp ON bp.id = cb.plan_id
           ORDER BY c.name"""
    )

    # Revenue totals
    revenue = await db_pool.fetchrow(
        """SELECT COALESCE(SUM(flat_fee), 0) AS total_flat,
                  COALESCE(SUM(perf_fee), 0) AS total_perf,
                  COALESCE(SUM(flat_fee + perf_fee), 0) AS total_revenue,
                  COUNT(*) FILTER (WHERE status = 'PAID') AS paid_periods,
                  COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_periods
           FROM billing_periods"""
    )

    return {
        "clients": [
            {
                "id": str(r["id"]),
                "name": r["name"],
                "client_status": r["client_status"],
                "plan": r["plan_name"],
                "flat_rate": float(r["flat_rate"]) if r["flat_rate"] else None,
                "perf_pct": float(r["perf_pct"]) if r["perf_pct"] else None,
                "billing_status": r["billing_status"],
                "hwm_equity": float(r["hwm_equity"]) if r["hwm_equity"] else None,
                "month_pnl": float(r["month_pnl"]),
                "total_pnl": float(r["total_pnl"]),
                "last_report": r["last_report"].isoformat() if r["last_report"] else None,
                "notes": r["notes"],
            }
            for r in clients
        ],
        "revenue": {
            "total_flat": float(revenue["total_flat"]),
            "total_perf": float(revenue["total_perf"]),
            "total_revenue": float(revenue["total_revenue"]),
            "paid_periods": revenue["paid_periods"],
            "pending_periods": revenue["pending_periods"],
        },
    }


class ClosePeriodRequest(BaseModel):
    period_start: str  # YYYY-MM-DD (first of month)
    period_end: str    # YYYY-MM-DD (last of month)
    client_id: str = ""  # empty = all active clients


@app.post("/api/v1/admin/billing/close-period")
async def close_billing_period(body: ClosePeriodRequest, _: None = Depends(require_master)):
    """Close a monthly billing period — calculates flat + performance fees.

    Performance fee = perf_pct * max(0, ending_equity - hwm_at_period_start).
    Only charged on net-new profits above the high-water mark.
    """
    period_start = date.fromisoformat(body.period_start)
    period_end = date.fromisoformat(body.period_end)

    # Get all active client billing records (or specific client)
    query = """SELECT cb.client_id, cb.plan_id, cb.hwm_equity,
                      bp.flat_rate, bp.perf_pct, bp.strategy_id
               FROM client_billing cb
               JOIN billing_plans bp ON bp.id = cb.plan_id
               WHERE cb.status = 'ACTIVE'"""
    params = []
    if body.client_id:
        query += " AND cb.client_id = $1"
        params.append(body.client_id)

    records = await db_pool.fetch(query, *params)
    results = []

    for rec in records:
        client_id = rec["client_id"]
        hwm_start = float(rec["hwm_equity"])

        # Get P&L for this period
        period_pnl = await db_pool.fetchrow(
            """SELECT COALESCE(SUM(net_pnl), 0) AS net_pnl,
                      MAX(ending_equity) AS peak_equity
               FROM pnl_reports
               WHERE client_id = $1
                 AND strategy_id = $2
                 AND trade_date BETWEEN $3 AND $4""",
            client_id, rec["strategy_id"], period_start, period_end,
        )

        net_pnl = float(period_pnl["net_pnl"])
        peak_equity = float(period_pnl["peak_equity"] or 0)

        # Performance fee: only on gains above HWM
        perf_fee = 0.0
        hwm_end = hwm_start
        if peak_equity > hwm_start and float(rec["perf_pct"]) > 0:
            gain_above_hwm = peak_equity - hwm_start
            perf_fee = round(gain_above_hwm * float(rec["perf_pct"]) / 100, 2)
            hwm_end = peak_equity

        flat_fee = float(rec["flat_rate"])

        # Upsert billing period
        await db_pool.execute(
            """INSERT INTO billing_periods
                   (client_id, plan_id, period_start, period_end,
                    flat_fee, perf_fee, net_pnl, hwm_start, hwm_end)
               VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
               ON CONFLICT (client_id, plan_id, period_start) DO UPDATE SET
                   flat_fee = EXCLUDED.flat_fee,
                   perf_fee = EXCLUDED.perf_fee,
                   net_pnl = EXCLUDED.net_pnl,
                   hwm_end = EXCLUDED.hwm_end,
                   status = 'PENDING'""",
            client_id, rec["plan_id"], period_start, period_end,
            flat_fee, perf_fee, net_pnl, hwm_start, hwm_end,
        )

        # Update HWM on client billing
        if hwm_end > hwm_start:
            await db_pool.execute(
                "UPDATE client_billing SET hwm_equity = $1 WHERE client_id = $2 AND plan_id = $3",
                hwm_end, client_id, rec["plan_id"],
            )

        results.append({
            "client_id": str(client_id),
            "plan_id": rec["plan_id"],
            "net_pnl": net_pnl,
            "flat_fee": flat_fee,
            "perf_fee": perf_fee,
            "total_due": flat_fee + perf_fee,
            "hwm_start": hwm_start,
            "hwm_end": hwm_end,
        })

    await _audit(None, None, None, "CLOSE_PERIOD",
                 detail=f"{period_start} to {period_end}, {len(results)} clients")
    _log.info(f"Closed billing period {period_start} to {period_end}: {len(results)} clients")

    total_flat = sum(r["flat_fee"] for r in results)
    total_perf = sum(r["perf_fee"] for r in results)
    total_due = sum(r["total_due"] for r in results)

    await _notify_marketplace(
        f"Billing Period Closed | {period_start} to {period_end}",
        f"**{len(results)}** client(s) invoiced.",
        color=0xF39C12,
        fields=[
            {"name": "Total Flat Fees", "value": f"${total_flat:,.2f}", "inline": True},
            {"name": "Total Perf Fees", "value": f"${total_perf:,.2f}", "inline": True},
            {"name": "Total Revenue", "value": f"**${total_due:,.2f}**", "inline": True},
        ] + [
            {"name": r["client_id"][:8], "value": f"${r['total_due']:,.2f} (P&L: ${r['net_pnl']:+,.2f})", "inline": True}
            for r in results[:10]
        ],
    )

    return {"period": f"{period_start} to {period_end}", "invoices": results}


class MarkPaidRequest(BaseModel):
    client_id: str
    period_start: str  # YYYY-MM-DD
    plan_id: str
    notes: str = ""  # "Venmo 4/7", "waived", etc.


@app.post("/api/v1/admin/billing/mark-paid")
async def mark_paid(body: MarkPaidRequest, _: None = Depends(require_master)):
    """Mark a billing period as paid (manual settlement for F&F)."""
    result = await db_pool.execute(
        """UPDATE billing_periods
           SET status = 'PAID', paid_at = NOW(), notes = $1
           WHERE client_id = $2 AND plan_id = $3 AND period_start = $4""",
        body.notes or "manual", body.client_id, body.plan_id,
        date.fromisoformat(body.period_start),
    )
    if result == "UPDATE 0":
        raise HTTPException(404, "Billing period not found")

    await _audit(body.client_id, None, None, "MARK_PAID",
                 detail=f"plan={body.plan_id} period={body.period_start} notes={body.notes}")
    _log.info(f"Marked paid: {body.client_id} {body.plan_id} {body.period_start}")
    return {"paid": True}


class WaivePeriodRequest(BaseModel):
    client_id: str
    period_start: str
    plan_id: str
    notes: str = ""


@app.post("/api/v1/admin/billing/waive")
async def waive_period(body: WaivePeriodRequest, _: None = Depends(require_master)):
    """Waive a billing period (first month free, etc.)."""
    await db_pool.execute(
        """UPDATE billing_periods
           SET status = 'WAIVED', notes = $1
           WHERE client_id = $2 AND plan_id = $3 AND period_start = $4""",
        body.notes or "waived", body.client_id, body.plan_id,
        date.fromisoformat(body.period_start),
    )
    _log.info(f"Waived: {body.client_id} {body.plan_id} {body.period_start}")
    return {"waived": True}


# ── Admin: P&L Verification ───────────────────────────────

class VerifyPnlRequest(BaseModel):
    client_id: str
    trade_date: str         # YYYY-MM-DD
    strategy_id: str
    verified_pnl: float     # actual P&L from broker read-only API
    notes: str = ""


@app.post("/api/v1/admin/billing/verify-pnl")
async def verify_pnl(body: VerifyPnlRequest, _: None = Depends(require_master)):
    """Cross-check a client's reported P&L against broker data.

    Usage: pull P&L from client's read-only broker API, then post here.
    Flags discrepancies > $10 as mismatches.
    """
    trade_date = date.fromisoformat(body.trade_date)

    report = await db_pool.fetchrow(
        """SELECT id, net_pnl, verified FROM pnl_reports
           WHERE client_id = $1 AND strategy_id = $2 AND trade_date = $3""",
        body.client_id, body.strategy_id, trade_date,
    )
    if not report:
        raise HTTPException(404, "No P&L report found for that date")

    reported_pnl = float(report["net_pnl"])
    discrepancy = abs(reported_pnl - body.verified_pnl)
    match = discrepancy <= 10.0  # $10 tolerance for rounding/timing

    await db_pool.execute(
        """UPDATE pnl_reports
           SET verified = TRUE, verified_at = NOW(), verified_pnl = $1
           WHERE id = $2""",
        body.verified_pnl, report["id"],
    )

    detail = (f"reported={reported_pnl:+.2f} verified={body.verified_pnl:+.2f} "
              f"diff={discrepancy:.2f} {'MATCH' if match else 'MISMATCH'}")
    await _audit(body.client_id, body.strategy_id, None, "VERIFY_PNL", detail=detail)

    if not match:
        _log.warning(f"P&L MISMATCH: {body.client_id} {body.trade_date} {detail}")

    return {
        "verified": True,
        "match": match,
        "reported_pnl": reported_pnl,
        "verified_pnl": body.verified_pnl,
        "discrepancy": discrepancy,
    }


@app.get("/api/v1/admin/billing/pnl-summary/{client_id}")
async def client_pnl_summary(client_id: str, _: None = Depends(require_master)):
    """Detailed P&L summary for a specific client."""
    # Daily P&L (last 90 days)
    daily = await db_pool.fetch(
        """SELECT trade_date, strategy_id, gross_pnl, net_pnl, trade_count,
                  ending_equity, verified, verified_pnl
           FROM pnl_reports
           WHERE client_id = $1 AND trade_date >= CURRENT_DATE - 90
           ORDER BY trade_date DESC""",
        client_id,
    )

    # Monthly aggregates
    monthly = await db_pool.fetch(
        """SELECT date_trunc('month', trade_date)::date AS month,
                  SUM(net_pnl) AS net_pnl,
                  SUM(trade_count) AS trades,
                  COUNT(*) AS trading_days,
                  MAX(ending_equity) AS peak_equity,
                  COUNT(*) FILTER (WHERE verified) AS verified_days,
                  COUNT(*) FILTER (WHERE verified AND ABS(net_pnl - COALESCE(verified_pnl, net_pnl)) > 10) AS mismatches
           FROM pnl_reports
           WHERE client_id = $1
           GROUP BY date_trunc('month', trade_date)
           ORDER BY month DESC LIMIT 12""",
        client_id,
    )

    return {
        "client_id": client_id,
        "daily": [
            {
                "date": r["trade_date"].isoformat(),
                "strategy": r["strategy_id"],
                "gross_pnl": float(r["gross_pnl"]) if r["gross_pnl"] else None,
                "net_pnl": float(r["net_pnl"]),
                "trades": r["trade_count"],
                "equity": float(r["ending_equity"]) if r["ending_equity"] else None,
                "verified": r["verified"],
                "verified_pnl": float(r["verified_pnl"]) if r["verified_pnl"] else None,
            }
            for r in daily
        ],
        "monthly": [
            {
                "month": r["month"].isoformat(),
                "net_pnl": float(r["net_pnl"]),
                "trades": r["trades"],
                "trading_days": r["trading_days"],
                "peak_equity": float(r["peak_equity"]) if r["peak_equity"] else None,
                "verified_days": r["verified_days"],
                "mismatches": r["mismatches"],
            }
            for r in monthly
        ],
    }


# ── Admin: Integrity Monitoring ────────────────────────────

@app.get("/api/v1/admin/integrity")
async def integrity_status(_: None = Depends(require_master)):
    """View integrity status of all clients. Flags modifications silently."""
    rows = await db_pool.fetch(
        """SELECT ci.client_id, c.name, ci.checked_at, ci.file_hashes,
                  ci.runtime, ci.os_platform, ci.status, ci.detail
           FROM client_integrity ci
           JOIN registry_clients c ON c.id = ci.client_id
           ORDER BY ci.status DESC, ci.checked_at DESC"""
    )
    return {
        "clients": [
            {
                "client_id": str(r["client_id"]),
                "name": r["name"],
                "checked_at": r["checked_at"].isoformat(),
                "status": r["status"],
                "detail": r["detail"],
                "runtime": r["runtime"],
                "os": r["os_platform"],
                "file_hashes": json.loads(r["file_hashes"]) if r["file_hashes"] else {},
            }
            for r in rows
        ],
        "expected_hashes": EXPECTED_CLIENT_HASHES or "not configured (learning mode)",
    }


@app.post("/api/v1/admin/integrity/learn/{client_id}")
async def learn_integrity(client_id: str, _: None = Depends(require_master)):
    """Capture a trusted client's current file hashes as the expected baseline.

    Use after deploying a new client version: pick a trusted client,
    call this endpoint, then set EXPECTED_CLIENT_HASHES env var to the
    returned hashes. Any client that deviates will be flagged.
    """
    row = await db_pool.fetchrow(
        "SELECT file_hashes FROM client_integrity WHERE client_id = $1",
        client_id,
    )
    if not row or not row["file_hashes"]:
        raise HTTPException(404, "No integrity data for this client — wait for next sync")

    hashes = json.loads(row["file_hashes"])
    env_value = json.dumps(hashes)

    _log.info(f"Learned integrity baseline from {client_id}: {hashes}")
    return {
        "learned_from": client_id,
        "file_hashes": hashes,
        "env_var": f'EXPECTED_CLIENT_HASHES={env_value!r}',
        "instruction": "Add this to your .env file or Docker environment to enable integrity enforcement.",
    }


# ── Stripe Webhook (future automation) ─────────────────────

STRIPE_WEBHOOK_SECRET = Config.stripe.webhook_secret
STRIPE_ENABLED = Config.stripe.enabled


@app.post("/api/v1/webhooks/stripe")
async def stripe_webhook(request: Request):
    """Handle Stripe payment events for automated billing.

    Set STRIPE_ENABLED=true and STRIPE_WEBHOOK_SECRET to activate.
    Events handled:
        - invoice.paid → mark billing period PAID
        - invoice.payment_failed → suspend client after 3 failures
        - customer.subscription.deleted → cancel client billing
    """
    if not STRIPE_ENABLED:
        raise HTTPException(404, "Stripe integration not enabled")

    try:
        import stripe
    except ImportError:
        raise HTTPException(500, "stripe package not installed")

    payload = await request.body()
    sig_header = request.headers.get("stripe-signature", "")

    try:
        event = stripe.Webhook.construct_event(payload, sig_header, STRIPE_WEBHOOK_SECRET)
    except (ValueError, stripe.error.SignatureVerificationError):
        raise HTTPException(400, "Invalid Stripe signature")

    event_type = event["type"]
    data = event["data"]["object"]

    if event_type == "invoice.paid":
        # Find client by Stripe customer ID and mark period paid
        client_billing = await db_pool.fetchrow(
            "SELECT client_id, plan_id FROM client_billing WHERE stripe_customer = $1",
            data["customer"],
        )
        if client_billing:
            # Mark the most recent pending period as paid
            await db_pool.execute(
                """UPDATE billing_periods
                   SET status = 'PAID', paid_at = NOW(), stripe_invoice = $1,
                       notes = 'Stripe auto-pay'
                   WHERE client_id = $2 AND plan_id = $3 AND status = 'PENDING'
                   ORDER BY period_start DESC LIMIT 1""",
                data["id"], client_billing["client_id"], client_billing["plan_id"],
            )
            _log.info(f"Stripe: invoice.paid for {data['customer']}")

    elif event_type == "invoice.payment_failed":
        client_billing = await db_pool.fetchrow(
            "SELECT client_id FROM client_billing WHERE stripe_customer = $1",
            data["customer"],
        )
        if client_billing:
            attempt = data.get("attempt_count", 1)
            if attempt >= 3:
                # Auto-suspend after 3 failed attempts
                await suspend_client(str(client_billing["client_id"]))
                _log.warning(f"Stripe: auto-suspended {data['customer']} after {attempt} failures")
            else:
                _log.warning(f"Stripe: payment failed for {data['customer']} (attempt {attempt})")

    elif event_type == "customer.subscription.deleted":
        rows = await db_pool.fetch(
            "SELECT client_id FROM client_billing WHERE stripe_sub = $1",
            data["id"],
        )
        for row in rows:
            await db_pool.execute(
                """UPDATE client_billing SET status = 'CANCELLED', cancelled_at = NOW()
                   WHERE client_id = $1 AND stripe_sub = $2""",
                row["client_id"], data["id"],
            )
            _log.info(f"Stripe: subscription cancelled for {row['client_id']}")

    return {"received": True}


