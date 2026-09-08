package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/faiq/labuh/internal/auth"
	"github.com/faiq/labuh/internal/database"
	"github.com/faiq/labuh/internal/database/repo"
	"github.com/faiq/labuh/internal/handler"
	"github.com/faiq/labuh/internal/web/layouts"
	"github.com/faiq/labuh/internal/web/pages"
	"github.com/faiq/labuh/internal/web/pages/projects"
)

func main() {
	ctx := context.Background()

	db, err := database.Connect("labuh.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(db, "internal/database/migrations/001_init.sql"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	userRepo := repo.NewUserRepo(db)
	sessionRepo := repo.NewSessionRepo(db)
	projectRepo := repo.NewProjectRepo(db)

	authHandler := handler.NewAuthHandler(userRepo, sessionRepo, false)
	projectsHandler := handler.NewProjectsHandler(projectRepo)

	authMiddleware := auth.RequireAuth(sessionRepo, userRepo, false)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Get("/auth/login", authHandler.LoginPage)
		r.Post("/auth/login", authHandler.Login)
		r.Get("/auth/logout", authHandler.Logout)
	})

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.UserFromContext(r.Context())
			if user == nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}
			templ.Handler(layouts.AppLayout(pages.DashboardPage(user.Name))).ServeHTTP(w, r)
		})
		r.Get("/projects", projectsHandler.List)
		r.Post("/projects", projectsHandler.Create)
		r.Get("/projects/{id}", projectsHandler.Get)
		r.Post("/projects/{id}/delete", projectsHandler.Delete)
		r.Get("/projects/create", func(w http.ResponseWriter, r *http.Request) {
			templ.Handler(layouts.AppLayout(projects.ProjectCreatePage())).ServeHTTP(w, r)
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	log.Printf("Labuh server starting on :%s", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
