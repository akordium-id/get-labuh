# Module: UI Layer (Templ Pages)

Package `internal/web` — server-side UI via [Templ](https://templ.guide/) + HTMX + Alpine.js.

## Struktur

```
web/
├── components/     ← shared UI components (badge, card, modal, form elements)
├── layouts/        ← page layouts
│   └── AppLayout   ← layout utama dengan topbar + sidebar
└── pages/          ← full pages per domain
    ├── applications/
    ├── databases/
    ├── deployments/
    ├── compose/
    ├── projects/
    ├── templates/
    ├── settings/
    ├── lite/       ← halaman khusus mode Lite
    ├── dashboard.templ
    ├── login.templ
    └── terminal.templ
```

## Cara Kerja Templ

- File `.templ` adalah source — **edit hanya file `.templ`**
- File `*_templ.go` di-generate otomatis: `templ generate` (atau `make generate`)
- **Jangan edit `*_templ.go` manual** — akan tertimpa saat `templ generate`

## AppLayout

```go
// Signature layout utama
func AppLayout(content templ.Component, mode ...middleware.UserMode) templ.Component

// Penggunaan di handler
templ.Handler(layouts.AppLayout(pages.DashboardPage(user.Name), userMode)).ServeHTTP(w, r)
```

Layout mendeteksi `userMode` untuk render sidebar adaptive (Pro vs Lite).

## HTMX Patterns

```html
<!-- Partial update (handler return fragment) -->
<div hx-get="/applications/{id}/status" hx-trigger="every 2s" hx-swap="outerHTML">

<!-- Form submit dengan loading state -->
<form hx-post="/applications/{id}/deploy" hx-indicator="#spinner">

<!-- Toast via out-of-band response -->
<!-- Handler set header: HX-Trigger: {"showToast":{"type":"success","message":"Deployed!"}} -->
```

## Alpine.js Patterns

```html
<!-- Toggle state lokal -->
<div x-data="{ open: false }">

<!-- Terminal (xterm.js) -->
<div x-data="terminal('{appID}')" x-init="init()">
```

## Rules untuk AI Agent

1. Setelah membuat atau mengubah file `.templ`, jalankan `make generate` atau `templ generate ./...`
2. Page baru: buat di subfolder domain yang sesuai (`pages/applications/`, bukan langsung di `pages/`)
3. Component yang dipakai di \>2 halaman → pindahkan ke `components/`
4. Untuk SSE (build logs, monitoring stats): gunakan `text/event-stream` di handler, bukan HTMX biasa
5. Mode Lite: halaman spesifik Lite ada di `pages/lite/` — jangan campur dengan halaman Pro

## Sidebar Menu (Mode-Aware)

Sidebar secara otomatis menyesuaikan item berdasarkan mode dari context:
- **Pro**: Projects, Applications, Databases, Compose, Templates, Monitoring, Backup, Settings
- **Lite**: Hanya Applications (card view), Templates, Settings
