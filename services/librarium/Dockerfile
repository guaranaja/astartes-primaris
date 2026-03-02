# Librarium uses official TimescaleDB image with custom migrations.
# See docker-compose.yml for runtime configuration.
# This Dockerfile is for building a custom image with pre-loaded extensions.

FROM timescale/timescaledb:latest-pg16

COPY migrations/ /docker-entrypoint-initdb.d/

EXPOSE 5432
