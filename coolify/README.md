# Labuh Deployment on Coolify

Labuh is designed to work seamlessly with Coolify for easy deployment.

## Prerequisites

- Coolify instance running
- Docker Engine available on the host
- Domain name pointing to your server (optional, for production)

## Deployment Steps

1. In Coolify, create a new **Dockerfile** resource
2. Connect your Git repository
3. Use the following build configuration:
   - **Build Command**: `go build ./cmd/labuh`
   - **Start Command**: `./labuh`
   - **Port**: `3000`

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `PORT` | Server port | No (default: 3000) |
| `DATABASE_URL` | Database path | No (default: `labuh.db`) |
| `SESSION_SECRET` | Session encryption key | Yes |
| `LABUH_MASTER_KEY` | Master key for encryption | Yes |
| `TRUSTED_PROXIES` | Comma-separated proxy IPs | No |

## Volumes

Create a persistent volume at `/var/lib/labuh` to store:
- SQLite database
- Backups
- Deploy keys
- Application data

## Docker Socket

Labuh needs access to the Docker daemon socket to manage containers. In Coolify:

1. Go to your application settings
2. Under **Advanced**, enable **Docker socket mount**
3. This mounts `/var/run/docker.sock` into the container

## Post-Deployment

1. Access Labuh at `http://your-server-ip:3000`
2. Complete the initial setup wizard
3. Create your first project and deploy an application

## SSL/TLS

Coolify automatically provisions SSL certificates. Labuh should be accessed via the Coolify proxy.

For custom domains:
1. Add your domain in Coolify
2. Coolify will automatically configure Traefik/Caddy proxy

## Troubleshooting

- Check logs in Coolify dashboard
- Ensure Docker socket is properly mounted
- Verify `SESSION_SECRET` and `LABUH_MASTER_KEY` are set
- Ensure port 3000 is not blocked by firewall
