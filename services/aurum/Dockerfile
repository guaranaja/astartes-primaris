# Aurum — Dashboard UI served by a minimal Node.js static server.
# In production on Cloud Run, this serves the SPA and proxies API calls to Primarch.

FROM node:20-alpine

WORKDIR /app
COPY server.js .
COPY public/ public/

# Cloud Run injects PORT env var — Aurum reads AURUM_PORT.
ENV AURUM_PORT=8080
EXPOSE 8080

CMD ["node", "server.js"]
