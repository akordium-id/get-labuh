package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(
	projectsHandler *ProjectsHandler,
	applicationsHandler *ApplicationsHandler,
	deploymentsHandler *DeploymentsHandler,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middlewareLogger)
	r.Use(middlewareRecoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = w
	})

	r.Route("/projects", func(r chi.Router) {
		r.Get("/", projectsHandler.List)
		r.Post("/", projectsHandler.Create)
		r.Get("/{id}", projectsHandler.Get)
		r.Get("/{id}/applications", applicationsHandler.ListByProject)
	})

	r.Route("/applications", func(r chi.Router) {
		r.Post("/{id}/deploy", applicationsHandler.Deploy)
		r.Get("/{id}/status", applicationsHandler.GetStatus)
	})

	r.Route("/deployments", func(r chi.Router) {
		r.Get("/{id}", deploymentsHandler.Get)
	})

	return r
}

func middlewareLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = next
		_ = w
		_ = r
	})
}

func middlewareRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = next
		_ = w
		_ = r
	})
}
