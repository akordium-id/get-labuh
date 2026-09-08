# Deployment Guide

## Production Deployment

### Docker

```bash
docker build -t labuh .
docker run -p 3000:3000 labuh
```

### Coolify

Labuh is designed to work with Coolify. Deploy using the following settings:

- **Build Command**: `go build ./cmd/labuh`
- **Start Command**: `./labuh`
- **Port**: 3000

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `3000` |
| `DATABASE_URL` | Database connection | `labuh.db` |
| `SESSION_SECRET` | Session encryption key | (required) |
| `TRUSTED_PROXIES` | Comma-separated proxy IPs | (none) |

### Database

For production, use PostgreSQL instead of SQLite:

```bash
export DATABASE_URL=postgres://user:pass@host:5432/labuh
```

### SSL/TLS

Run behind a reverse proxy like Nginx or Traefik:

```nginx
server {
    listen 80;
    server_name labuh.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name labuh.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Upgrading

```bash
git pull
go build ./cmd/labuh
systemctl restart labuh
```
