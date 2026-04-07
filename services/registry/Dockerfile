FROM python:3.12-slim AS builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends libpq5 curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY . .

RUN useradd -r -s /bin/false app && mkdir -p /app/data/published_bundles && chown -R app:app /app
USER app

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
    CMD curl -sf http://localhost:8701/healthz || exit 1

EXPOSE 8701

ENTRYPOINT ["python", "-m", "registry"]
