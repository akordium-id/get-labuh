package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/web/pages"
)

type TerminalHandler struct {
	dockerClient *docker.Client
	appRepo      *repo.ApplicationRepo
	dbRepo       *repo.DatabaseRepo
	composeRepo  *repo.ComposeRepo
}

func NewTerminalHandler(
	dockerClient *docker.Client,
	appRepo *repo.ApplicationRepo,
	dbRepo *repo.DatabaseRepo,
	composeRepo *repo.ComposeRepo,
) *TerminalHandler {
	return &TerminalHandler{
		dockerClient: dockerClient,
		appRepo:      appRepo,
		dbRepo:       dbRepo,
		composeRepo:  composeRepo,
	}
}

// ApplicationTerminal menampilkan halaman web console untuk aplikasi
func (h *TerminalHandler) ApplicationTerminal(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(appID)
	if err != nil {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	if app.ContainerID == nil || *app.ContainerID == "" {
		http.Error(w, "Container is not running. Please deploy or start the application first.", http.StatusBadRequest)
		return
	}

	title := app.Name + " (App)"
	subtitle := "Container: " + *app.ContainerName
	wsURL := fmt.Sprintf("/containers/%s/terminal/ws", *app.ContainerID)
	backURL := fmt.Sprintf("/applications/%s", app.ID)

	templ.Handler(pages.ContainerTerminalPage(title, subtitle, wsURL, backURL)).ServeHTTP(w, r)
}

// DatabaseTerminal menampilkan halaman web console untuk database
func (h *TerminalHandler) DatabaseTerminal(w http.ResponseWriter, r *http.Request) {
	dbID := chi.URLParam(r, "id")
	if dbID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db, err := h.dbRepo.GetByID(dbID)
	if err != nil {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}

	if db.ContainerID == nil || *db.ContainerID == "" {
		http.Error(w, "Database container is not running. Please start the database first.", http.StatusBadRequest)
		return
	}

	containerName := db.Slug
	if db.ContainerName != nil {
		containerName = *db.ContainerName
	}

	title := fmt.Sprintf("%s (%s)", db.Name, db.Engine)
	subtitle := "Container: " + containerName
	wsURL := fmt.Sprintf("/containers/%s/terminal/ws", *db.ContainerID)
	backURL := fmt.Sprintf("/databases/%s", db.ID)

	templ.Handler(pages.ContainerTerminalPage(title, subtitle, wsURL, backURL)).ServeHTTP(w, r)
}

type resizeMessage struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

// ContainerTerminalWS melayani koneksi WebSocket untuk streaming TTY dua arah
func (h *TerminalHandler) ContainerTerminalWS(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "container_id")
	if containerID == "" {
		http.Error(w, "Missing container_id", http.StatusBadRequest)
		return
	}

	if h.dockerClient == nil {
		http.Error(w, "Docker client unavailable", http.StatusInternalServerError)
		return
	}

	shell := r.URL.Query().Get("shell")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Upgrade HTTP connection ke WebSocket
	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("failed to accept websocket connection", "error", err)
		return
	}
	defer wsConn.Close(websocket.StatusInternalError, "closing session")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Mulai Docker Exec TTY session
	execID, attachResp, err := h.dockerClient.ExecTTY(ctx, containerID, []string{shell}, []string{
		"TERM=xterm-256color",
		"LANG=C.UTF-8",
	})

	// Fallback ke /bin/sh jika /bin/bash gagal
	if err != nil && shell == "/bin/bash" {
		shell = "/bin/sh"
		execID, attachResp, err = h.dockerClient.ExecTTY(ctx, containerID, []string{shell}, []string{
			"TERM=xterm-256color",
			"LANG=C.UTF-8",
		})
	}

	if err != nil {
		_ = wsConn.Write(ctx, websocket.MessageText, fmt.Appendf(nil, "\r\n\x1b[31m[Error]\x1b[0m Failed to start shell %s: %v\r\n", shell, err))
		return
	}
	defer attachResp.Close()

	// Goroutine 1: Baca dari Container Stdout & Stderr -> Kirim ke WebSocket
	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, err := attachResp.Reader.Read(buf)
			if n > 0 {
				writeCtx, writeCancel := context.WithTimeout(context.Background(), 5*time.Second)
				writeErr := wsConn.Write(writeCtx, websocket.MessageBinary, buf[:n])
				writeCancel()
				if writeErr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "closed") {
					slog.Debug("terminal exec reader closed", "error", err)
				}
				return
			}
		}
	}()

	// Goroutine 2 (Main loop): Baca dari WebSocket -> Tulis ke Container Stdin
	for {
		msgType, msgBytes, err := wsConn.Read(ctx)
		if err != nil {
			break
		}

		if msgType == websocket.MessageText {
			// Cek apakah pesan resize JSON dari xterm fitAddon
			var rsz resizeMessage
			if err := json.Unmarshal(msgBytes, &rsz); err == nil && rsz.Type == "resize" && rsz.Cols > 0 && rsz.Rows > 0 {
				_ = h.dockerClient.ResizeExec(ctx, execID, rsz.Rows, rsz.Cols)
				continue
			}
		}

		// Input keyboard langsung ditulis ke stdin container
		if len(msgBytes) > 0 {
			if _, err := attachResp.Conn.Write(msgBytes); err != nil {
				break
			}
		}
	}

	_ = wsConn.Close(websocket.StatusNormalClosure, "session ended")
}
