package projects

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/projects"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

const encryptionKey = "labuh-deploy-key-encryption-key-32b!!"

// UserProvider mendefinisikan interface dependency untuk user mapping pada team management.
type UserProvider interface {
	GetAll() ([]*models.User, error)
}

// Handler mengelola HTTP request untuk domain projects, deploy keys, dan teams.
type Handler struct {
	projectRepo   *ProjectRepo
	deployKeyRepo *DeployKeyRepo
	teamRepo      *TeamRepo
	userProvider  UserProvider
}

func NewHandler(projectRepo *ProjectRepo, deployKeyRepo *DeployKeyRepo, teamRepo *TeamRepo, userProvider UserProvider) *Handler {
	return &Handler{
		projectRepo:   projectRepo,
		deployKeyRepo: deployKeyRepo,
		teamRepo:      teamRepo,
		userProvider:  userProvider,
	}
}

// --- Projects Handlers ---

func (h *Handler) ProjectsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := parseInt(r.URL.Query().Get("page"), 1)
	search := r.URL.Query().Get("search")
	limit := 10
	offset := (page - 1) * limit

	var projectList []*models.Project
	var total int
	var err error

	if search != "" {
		projectList, total, err = h.projectRepo.SearchPaginatedWithContext(ctx, search, limit, offset)
	} else {
		projectList, total, err = h.projectRepo.GetAllPaginatedWithContext(ctx, limit, offset)
	}

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	envCounts := make(map[string]int)
	for _, p := range projectList {
		envs, _ := h.projectRepo.GetEnvironmentsByProjectIDWithContext(ctx, p.ID)
		envCounts[p.ID] = len(envs)
	}

	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(projects.ProjectsTable(projectList, envCounts, page, totalPages, search)).ServeHTTP(w, r)
		return
	}

	templ.Handler(layouts.AppLayout(projects.ProjectsListPage(projectList, envCounts, page, totalPages, search))).ServeHTTP(w, r)
}

func (h *Handler) ProjectsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	slug := GenerateSlug(name)

	_, err := h.projectRepo.CreateWithContext(r.Context(), models.CreateProjectInput{
		Name:        name,
		Slug:        slug,
		Description: descPtr,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ProjectsGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			id = parts[1]
		}
	}
	if id == "" || id == "projects" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	project, err := h.projectRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	_ = project

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<div>Project detail placeholder</div>"))
}

func (h *Handler) ProjectsDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			id = parts[1]
		}
	}
	if id == "" || id == "projects" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.projectRepo.DeleteWithContext(r.Context(), id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ProjectsCreatePage(w http.ResponseWriter, r *http.Request) {
	templ.Handler(layouts.AppLayout(projects.ProjectCreatePage())).ServeHTTP(w, r)
}

// --- Deploy Keys Handlers ---

func (h *Handler) DeployKeysList(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	keys, err := h.deployKeyRepo.GetByProjectIDWithContext(r.Context(), projectID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.DeployKeysPage(projectID, keys))).ServeHTTP(w, r)
}

func (h *Handler) DeployKeysCreate(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = "Deploy Key"
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	publicKey := "ssh-ed25519 " + base64.StdEncoding.EncodeToString(pub)
	privateKey := base64.StdEncoding.EncodeToString(priv)

	fingerprintHash := sha256.Sum256(pub)
	fingerprintStr := base64.StdEncoding.EncodeToString(fingerprintHash[:])

	encryptedPrivateKey, err := encryptAES(privateKey, encryptionKey)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_, err = h.deployKeyRepo.CreateWithContext(r.Context(), models.CreateDeployKeyInput{
		ProjectID:           projectID,
		Name:                name,
		PublicKey:           publicKey,
		PrivateKeyEncrypted: encryptedPrivateKey,
		Fingerprint:         fingerprintStr,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+projectID+"/deploy-keys")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeployKeysDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.deployKeyRepo.DeleteWithContext(r.Context(), id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/deploy-keys")
	w.WriteHeader(http.StatusOK)
}

// --- Teams Handlers ---

func (h *Handler) TeamsList(w http.ResponseWriter, r *http.Request) {
	teams, err := h.teamRepo.GetAllWithContext(r.Context())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.TeamsPage(teams))).ServeHTTP(w, r)
}

func (h *Handler) TeamsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := h.teamRepo.CreateWithContext(r.Context(), models.CreateTeamInput{
		Name: name,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/teams")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TeamsDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	team, err := h.teamRepo.GetByIDWithContext(ctx, id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	members, err := h.teamRepo.GetMembersWithContext(ctx, id)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	userMap := make(map[string]*models.User)
	if h.userProvider != nil {
		users, _ := h.userProvider.GetAll()
		for _, u := range users {
			userMap[u.ID] = u
		}
	}

	templ.Handler(layouts.AppLayout(settings.TeamDetailPage(team, members, userMap))).ServeHTTP(w, r)
}

func (h *Handler) TeamsAddMember(w http.ResponseWriter, r *http.Request) {
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

	_, err := h.teamRepo.AddMemberWithContext(r.Context(), models.CreateTeamMemberInput{
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

func (h *Handler) TeamsRemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "user_id")
	if teamID == "" || userID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.teamRepo.RemoveMemberWithContext(r.Context(), teamID, userID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/teams/"+teamID)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TeamsUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
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
	if err := h.teamRepo.UpdateMemberRoleWithContext(r.Context(), teamID, userID, role); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/teams/"+teamID)
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil || val < 1 {
		return defaultValue
	}
	return val
}

func encryptAES(plaintext, keyString string) (string, error) {
	key := []byte(keyString)
	if len(key) > 32 {
		key = key[:32]
	}
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	plaintextBytes := []byte(plaintext)
	ciphertext := make([]byte, len(plaintextBytes))
	stream.XORKeyStream(ciphertext, plaintextBytes)

	result := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}
