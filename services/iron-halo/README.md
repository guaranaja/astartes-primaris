# IRON HALO — Authentication & Security

> Shields the Imperium from unauthorized access.

## Responsibilities

- Broker API key storage and rotation
- Service-to-service authentication (mTLS)
- User authentication for Aurum dashboard
- Secrets management and injection
- Audit logging of all access

## Tech

- **Secrets**: HashiCorp Vault
- **Auth**: mTLS for services, JWT for users
- **Certificates**: Auto-rotating TLS certs via Vault PKI

## Secret Paths

```
secret/
├── brokers/
│   ├── ibkr/credentials
│   ├── tastytrade/credentials
│   └── apex/credentials
├── services/
│   ├── librarium/db-password
│   ├── vox/auth-token
│   └── ...
└── users/
    └── api-keys/
```

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 8200  | HTTP     | Vault API            |
| 8201  | HTTP     | Health               |
