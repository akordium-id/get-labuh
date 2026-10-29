# Modul Deployments

Modul `deployments` mengelola status, lifecycle, dan log streaming untuk deployment aplikasi get-labuh.

## Struktur Modul

- `repo.go`: Data access layer untuk tabel `deployments` dengan context awareness.
- `handler.go`: Handler HTTP UI dan SSE stream log (`StreamDeploymentLogs`, `StreamContainerLogs`).
- `module.go`: Module container dan registrasi rute Chi.
- `deployments_test.go`: Unit test repo dan handler.

## Dependency Inversion

Modul ini mengisolasi akses aplikasi dengan interface `ApplicationProvider`:
```go
type ApplicationProvider interface {
    GetByID(id string) (*models.Application, error)
    UpdateStatus(id string, status models.AppStatus) error
}
```

## Rute yang Didaftarkan

- `GET /deployments/{id}` - Halaman detail deployment
- `GET /deployments/{id}/logs` - Halaman log deployment
- `POST /deployments/{id}/rollback` - Trigger rollback ke previous successful deployment
- `POST /deployments/{id}/start` - Memulai deployment
- `GET /deployments/{id}/logs/stream` - SSE stream live deploy log
- `GET /applications/{id}/logs/stream` - SSE stream container runtime log
