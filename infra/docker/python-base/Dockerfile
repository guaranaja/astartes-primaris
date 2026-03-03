# Python Base — Shared CPython image with stdlib extensions configured.
# All Python services in the Imperium inherit from this image so that
# stdlib extension availability is consistent and reproducible.
#
# Build:  docker build -t astartes/python-base:3.12 infra/docker/python-base/
# Usage:  FROM astartes/python-base:3.12

ARG PYTHON_VERSION=3.12.8

# ── Stage 1: Build CPython from source ────────────────────────────
FROM debian:bookworm-slim AS builder

ARG PYTHON_VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    # stdlib extension build dependencies
    libffi-dev \
    libssl-dev \
    zlib1g-dev \
    libbz2-dev \
    liblzma-dev \
    libsqlite3-dev \
    libreadline-dev \
    libncursesw5-dev \
    libgdbm-dev \
    libgdbm-compat-dev \
    uuid-dev \
    tk-dev \
    libmpdec-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

RUN curl -fsSL "https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz" \
    | tar xz --strip-components=1

# Configure CPython with all stdlib extensions enabled.
# --enable-loadable-sqlite-extensions: allow SQLite extension loading
# --with-system-ffi: use system libffi for _ctypes
# --with-ensurepip=upgrade: bundle pip
RUN ./configure \
    --prefix=/opt/python \
    --enable-loadable-sqlite-extensions \
    --with-system-ffi \
    --with-ensurepip=upgrade \
    --without-doc-strings \
    --disable-test-modules \
    2>&1 | tail -n 40

# Show which stdlib extension modules configure detected
RUN echo "=== Stdlib extension modules ===" && \
    grep -E "^checking for stdlib extension module" config.log | \
    sed 's/checking for stdlib extension module //' || true

# Build with parallel jobs
RUN make -j"$(nproc)" && make install

# Strip binaries to reduce image size
RUN find /opt/python -type f -name '*.so' -exec strip --strip-unneeded {} + 2>/dev/null || true && \
    find /opt/python -type f -name 'python3*' -executable -exec strip --strip-unneeded {} + 2>/dev/null || true

# ── Stage 2: Slim runtime image ──────────────────────────────────
FROM debian:bookworm-slim

# Runtime libraries for stdlib extensions
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libffi8 \
    libssl3 \
    zlib1g \
    libbz2-1.0 \
    liblzma5 \
    libsqlite3-0 \
    libreadline8 \
    libncursesw6 \
    libgdbm6 \
    libgdbm-compat4 \
    libuuid1 \
    libmpdec3 \
    libtk8.6 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /opt/python /opt/python

ENV PATH="/opt/python/bin:${PATH}" \
    LD_LIBRARY_PATH="/opt/python/lib" \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Verify stdlib extensions are available
COPY verify-extensions.py /tmp/verify-extensions.py
RUN python3 /tmp/verify-extensions.py && rm /tmp/verify-extensions.py

# Upgrade pip in the base image
RUN python3 -m pip install --no-cache-dir --upgrade pip setuptools

WORKDIR /app

CMD ["python3"]
