package handler

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/ssh"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type ServersHandler struct {
	serverRepo *repo.ServerRepo
}

func NewServersHandler(serverRepo *repo.ServerRepo) *ServersHandler {
	return &ServersHandler{
		serverRepo: serverRepo,
	}
}

func (h *ServersHandler) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.serverRepo.GetAll()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.ServersPage(servers))).ServeHTTP(w, r)
}

func (h *ServersHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	host := r.FormValue("host")
	port := 22
	if p := r.FormValue("port"); p != "" {
		var portVal int
		if _, err := fmt.Sscanf(p, "%d", &portVal); err == nil {
			port = portVal
		}
	}
	sshUser := r.FormValue("ssh_user")
	if sshUser == "" {
		sshUser = "root"
	}
	sshKeyPath := r.FormValue("ssh_key_path")

	_, err := h.serverRepo.Create(models.CreateServerInput{
		Name:       name,
		Host:       host,
		Port:       port,
		SSHUser:    sshUser,
		SSHKeyPath: sshKeyPath,
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

	err = ssh.TestConnection(server.Host, server.SSHUser, server.SSHKeyPath)
	if err != nil {
		_ = h.serverRepo.UpdateStatus(id, models.ServerStatusError)
		w.Header().Set("HX-Retarget", "#server-status-"+id)
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800\">Error</span>"))
		return
	}

	_ = h.serverRepo.UpdateStatus(id, models.ServerStatusOnline)
	w.Header().Set("HX-Retarget", "#server-status-"+id)
	w.Header().Set("HX-Reswap", "innerHTML")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800\">Online</span>"))
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
