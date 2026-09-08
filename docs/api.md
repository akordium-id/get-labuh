# API Documentation

Labuh provides a RESTful API under `/api/v1`. All API endpoints require authentication via API key.

## Authentication

Include your API key in the `Authorization` header:

```
Authorization: Bearer YOUR_API_KEY
```

Generate API keys from the web UI: Settings → API Keys

## Endpoints

### Projects

#### List Projects
```
GET /api/v1/projects
```

Response:
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "My Project",
      "slug": "my-project",
      "description": "..."
    }
  ]
}
```

#### Create Project
```
POST /api/v1/projects
```

Request body:
```json
{
  "name": "My Project",
  "slug": "my-project",
  "description": "Optional description"
}
```

#### Get Project
```
GET /api/v1/projects/{id}
```

### Applications

#### List Applications in Project
```
GET /api/v1/projects/{id}/applications
```

#### Trigger Deployment
```
POST /api/v1/applications/{id}/deploy
```

Response:
```json
{
  "data": {
    "deployment_id": "uuid"
  }
}
```

#### Get Application Status
```
GET /api/v1/applications/{id}/status
```

Response:
```json
{
  "data": {
    "id": "uuid",
    "name": "My App",
    "status": "running"
  }
}
```

### Deployments

#### Get Deployment Status
```
GET /api/v1/deployments/{id}
```

Response:
```json
{
  "data": {
    "id": "uuid",
    "application_id": "uuid",
    "status": "success",
    "commit_hash": "...",
    "created_at": "..."
  }
}
```

### Health Check

```
GET /health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "...",
  "checks": {
    "database": {
      "status": "healthy",
      "message": "ok"
    }
  }
}
```

### Metrics

```
GET /metrics
```

Response:
```json
{
  "timestamp": "...",
  "uptime": "...",
  "num_goroutine": 10,
  "num_gc": 5
}
```

## Error Responses

All errors follow this format:

```json
{
  "error": "Error message"
}
```

HTTP status codes:
- `200` - Success
- `201` - Created
- `400` - Bad Request
- `401` - Unauthorized
- `404` - Not Found
- `500` - Internal Server Error
