ARG PYTHON_BASE_IMAGE=astartes/python-base:3.12
FROM ${PYTHON_BASE_IMAGE}

WORKDIR /app

RUN pip install --no-cache-dir \
    nats-py \
    psycopg[binary] \
    redis \
    structlog \
    httpx

COPY . .

EXPOSE 8300

ENTRYPOINT ["python3", "-m", "auspex"]
