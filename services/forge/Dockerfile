# Forge — Backtest engine and strategy optimizer.
# Runs on Cloud Run Jobs (batch workloads, not always-on).

FROM python:3.12-slim

WORKDIR /app

RUN pip install --no-cache-dir \
    numpy \
    pandas \
    scipy \
    numba

COPY . .

# Forge is a Cloud Run Job — no persistent port needed.
# The API surface (:8500) is for local dev / inter-service calls.
EXPOSE 8500

ENTRYPOINT ["python", "-m", "forge"]
