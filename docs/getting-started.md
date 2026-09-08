# Getting Started with Labuh

Labuh is a self-hosted Platform-as-a-Service (PaaS) that makes it easy to deploy and manage applications.

## Prerequisites

- Go 1.26+
- Docker
- SQLite (or PostgreSQL for production)
- Git

## Installation

1. Clone the repository:
```bash
git clone https://github.com/akordium-id/get-labuh.git
cd get-labuh
```

2. Build the server:
```bash
go build ./cmd/labuh
```

3. Build the CLI (optional):
```bash
go build ./cmd/cli
```

## Running

```bash
./labuh
```

The server starts on port 3000 by default. Set the `PORT` environment variable to change it.

## First Steps

1. Visit `http://localhost:3000/auth/login`
2. The first registered user becomes the admin
3. Create your first project
4. Deploy an application from a template or from scratch

## Configuration

Labuh uses environment variables and SQLite for storage. Key environment variables:

- `PORT` - Server port (default: 3000)
- `DATABASE_URL` - Database connection string
- `SESSION_SECRET` - Session encryption key

## Using the CLI

```bash
# Configure CLI
labuh-cli config --api-key YOUR_API_KEY --url http://localhost:3000

# List projects
labuh-cli projects

# Deploy an application
labuh-cli deploy APP_ID
```
