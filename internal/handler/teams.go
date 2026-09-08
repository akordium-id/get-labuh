package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type TeamsHandler struct {
	teamRepo *repo.TeamRepo
	userRepo *repo.UserRepo
}

func NewTeamsHandler(teamRepo *repo.TeamRepo, userRepo *repo.UserRepo) *TeamsHandler {
	return &TeamsHandler{
		teamRepo: teamRepo,
		userRepo: userRepo,
	}
}

func (h *TeamsHandler) List(w http.ResponseWriter, r *http.Request) {
	teams, err := h.teamRepo.GetAll()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.TeamsPage(teams))).ServeHTTP(w, r)
}

func (h *TeamsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := h.teamRepo.Create(models.CreateTeamInput{
		Name: name,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/teams")
	w.WriteHeader(http.StatusOK)
}

func (h *TeamsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	team, err := h.teamRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	members, err := h.teamRepo.GetMembers(id)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	users, _ := h.userRepo.GetAll()
	userMap := make(map[string]*models.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	templ.Handler(layouts.AppLayout(settings.TeamDetailPage(team, members, userMap))).ServeHTTP(w, r)
}

func (h *TeamsHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("user_id")
	role := models.TeamRole(r.FormValue("role"))
	if userID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := h.teamRepo.AddMember(models.CreateTeamMemberInput{
		TeamID: id,
		UserID: userID,
		Role:   role,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/teams/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *TeamsHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "user_id")
	if teamID == "" || userID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.teamRepo.RemoveMember(teamID, userID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/teams/"+teamID)
	w.WriteHeader(http.StatusOK)
}

func (h *TeamsHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "user_id")
	if teamID == "" || userID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	role := models.TeamRole(r.FormValue("role"))
	if err := h.teamRepo.UpdateMemberRole(teamID, userID, role); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/teams/"+teamID)
	w.WriteHeader(http.StatusOK)
}
