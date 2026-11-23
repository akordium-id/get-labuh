# Implementation Plan: Sub-Project 3 (Server Observability, Proactive Resource Alerting, & Extended Notifications)

Implementation plan for Sub-Project 3 (Opsi A) in `get-labuh`.

**Target Architecture:**
1. Database migration `031_observability_alerts_and_webhook_channel.sql` & expanded `models.NotificationChannel`.
2. Host Metrics Collector in `internal/observability/host.go` (`GetHostStats`).
3. Proactive Alert Daemon with Anti-Spam / Cooldown in `internal/observability/host_monitor.go`.
4. Generic Webhook Channel Sender in `internal/modules/notifications/notifier.go`.
5. Monitoring Handlers, Routes & Daemon Wiring in `settings` & `app`.
6. Templ UI in `settings/monitoring.templ` & updated `settings/notifications.templ`.
7. Full Integration & Regression Tests.

---

### Task 1: Database Migration `031` & Models Expansion

**Files:**
- Create: `internal/database/migrations/031_observability_alerts_and_webhook_channel.sql`
- Modify: `internal/models/notification.go`
- Create: `internal/models/notification_channel_test.go`

- [x] **Step 1: Write model test**
- [x] **Step 2: Create SQL migration `031`**
- [x] **Step 3: Update `models.NotificationChannel`**
- [x] **Step 4: Verify test passes**
- [x] **Step 5: Commit**

---

### Task 2: Notification Repo & Generic Webhook Sender

**Files:**
- Modify: `internal/modules/notifications/repo.go`
- Modify: `internal/modules/notifications/notifier.go`
- Create: `internal/modules/notifications/webhook_notifier_test.go`

- [x] **Step 1: Write webhook notifier tests with mock HTTP server**
- [x] **Step 2: Update `repo.go` for new columns and `server_alert` event**
- [x] **Step 3: Implement `sendWebhook` in `notifier.go`**
- [x] **Step 4: Verify tests pass**
- [x] **Step 5: Commit**

---

### Task 3: Host Metrics Collection (`internal/observability/host.go`)

**Files:**
- Create: `internal/observability/host.go`
- Create: `internal/observability/host_test.go`

- [x] **Step 1: Write host metrics test**
- [x] **Step 2: Implement `GetHostStats()` with `Statfs`, `/proc/meminfo`, `/proc/loadavg`**
- [x] **Step 3: Verify tests pass**
- [x] **Step 4: Commit**

---

### Task 4: Proactive Alert Daemon & Cooldown Engine (`internal/observability/host_monitor.go`)

**Files:**
- Create: `internal/observability/host_monitor.go`
- Create: `internal/observability/host_monitor_test.go`

- [x] **Step 1: Write host monitor state machine & cooldown tests**
- [x] **Step 2: Implement `HostMonitor` daemon**
- [x] **Step 3: Verify tests pass**
- [x] **Step 4: Commit**

---

### Task 5: Settings Handlers, Routes & Daemon Wiring

**Files:**
- Modify: `internal/modules/settings/handler.go`
- Modify: `internal/modules/settings/module.go`
- Modify: `internal/app/handlers.go`
- Create: `internal/modules/settings/monitoring_handler_test.go`

- [x] **Step 1: Write settings monitoring handler test**
- [x] **Step 2: Implement monitoring settings handlers & routes**
- [x] **Step 3: Wire `HostMonitor` in `internal/app/handlers.go`**
- [x] **Step 4: Verify tests pass**
- [x] **Step 5: Commit**

---

### Task 6: UI in Templ (`settings/monitoring.templ` & `settings/notifications.templ`)

**Files:**
- Create: `internal/web/pages/settings/monitoring.templ`
- Modify: `internal/web/pages/settings/notifications.templ`
- Generate: `templ generate`

- [x] **Step 1: Create `monitoring.templ` with realtime gauges and threshold form**
- [x] **Step 2: Update `notifications.templ` with Generic Webhook & Server Alert toggle**
- [x] **Step 3: Run `templ generate` and verify build**
- [x] **Step 4: Commit**

---

### Task 7: End-to-End Integration Tests & Full Regression Suite

**Files:**
- Create: `internal/testing/observability_alert_test.go`

- [x] **Step 1: Write integration test covering alert triggering, webhook dispatch, and resolved status**
- [x] **Step 2: Run full regression suite `go test -v ./...`**
- [x] **Step 3: Commit**
