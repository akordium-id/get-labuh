package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/ssh"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type ServersHandler struct {
	serverRepo *repo.ServerRepo
	dockerPool *docker.ClientPool
}

func NewServersHandler(serverRepo *repo.ServerRepo) *ServersHandler {
	return &ServersHandler{
		serverRepo: serverRepo,
	}
}

func (h *ServersHandler) SetDockerPool(pool *docker.ClientPool) {
	h.dockerPool = pool
}

func (h *ServersHandler) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.serverRepo.GetAll()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.ServersPage(servers))).ServeHTTP(w, r)
}

func (h *ServersHandler) CreateForm(w http.ResponseWriter, r *http.Request) {
	templ.Handler(layouts.AppLayout(settings.ServerCreatePage())).ServeHTTP(w, r)
}

func (h *ServersHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	host := strings.TrimSpace(r.FormValue("host"))
	port := 22
	if p := r.FormValue("port"); p != "" {
		var portVal int
		if _, err := fmt.Sscanf(p, "%d", &portVal); err == nil && portVal > 0 {
			port = portVal
		}
	}
	sshUser := strings.TrimSpace(r.FormValue("ssh_user"))
	if sshUser == "" {
		sshUser = "root"
	}
	sshKeyPath := strings.TrimSpace(r.FormValue("ssh_key_path"))
	sshKeyContent := strings.TrimSpace(r.FormValue("ssh_key_content"))

	var keyContentPtr *string
	if sshKeyContent != "" {
		keyContentPtr = &sshKeyContent
	}

	_, err := h.serverRepo.Create(models.CreateServerInput{
		Name:          name,
		Host:          host,
		Port:          port,
		SSHUser:       sshUser,
		SSHKeyPath:    sshKeyPath,
		SSHKeyContent: keyContentPtr,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/servers")
	w.WriteHeader(http.StatusOK)
}

func (h *ServersHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	server, err := h.serverRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Kasus server lokal
	if id == "local" || server.Host == "127.0.0.1" || server.Host == "localhost" {
		_ = h.serverRepo.UpdateStatus(id, models.ServerStatusOnline)
		w.Header().Set("HX-Retarget", "#server-status-"+id)
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800\">Online (Local Docker)</span>"))
		return
	}

	// Remote Server: Uji SSH + Docker Daemon Ping via Docker Client Pool
	var testErr error
	if h.dockerPool != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		cli, err := h.dockerPool.GetClient(ctx, &server.ID)
		if err != nil {
			testErr = err
		} else {
			testErr = cli.Ping(ctx)
		}
	} else {
		testErr = ssh.TestConnection(server.Host, server.SSHUser, server.SSHKeyPath)
	}

	if testErr != nil {
		_ = h.serverRepo.UpdateStatus(id, models.ServerStatusError)
		w.Header().Set("HX-Retarget", "#server-status-"+id)
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800\" title=\"%s\">Error</span>", testErr.Error())
		return
	}

	_ = h.serverRepo.UpdateStatus(id, models.ServerStatusOnline)
	w.Header().Set("HX-Retarget", "#server-status-"+id)
	w.Header().Set("HX-Reswap", "innerHTML")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800\">Online (Docker Active)</span>"))
}

func (h *ServersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.serverRepo.Delete(id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/servers")
	w.WriteHeader(http.StatusOK)
}
