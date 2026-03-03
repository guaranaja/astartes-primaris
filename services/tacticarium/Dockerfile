ARG PYTHON_BASE_IMAGE=astartes/python-base:3.12
FROM ${PYTHON_BASE_IMAGE} AS base

WORKDIR /app

# Install marine SDK
COPY sdk/marine-sdk /tmp/marine-sdk
RUN pip install --no-cache-dir /tmp/marine-sdk && rm -rf /tmp/marine-sdk

# Copy strategy code
COPY . .

EXPOSE 9000

# Marine entrypoint handles the full lifecycle:
# wake → orient → decide → act → report → sleep
ENTRYPOINT ["python3", "-m", "marine_sdk.runner"]
