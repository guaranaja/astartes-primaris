FROM python:3.12-slim

WORKDIR /app

# Install dependencies — ib_insync for IBKR, psycopg for TimescaleDB, nats-py for Vox
RUN pip install --no-cache-dir \
    ib_insync \
    "psycopg[binary]" \
    psycopg_pool \
    nats-py \
    redis \
    structlog \
    httpx

COPY . .

EXPOSE 8300

ENTRYPOINT ["python", "-m", "auspex"]
