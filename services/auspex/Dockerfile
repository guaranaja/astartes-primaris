FROM python:3.12-slim

WORKDIR /app

RUN pip install --no-cache-dir \
    nats-py \
    psycopg[binary] \
    redis \
    structlog \
    httpx

COPY . .

EXPOSE 8300

ENTRYPOINT ["python", "-m", "auspex"]
