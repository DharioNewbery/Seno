package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	chicors "github.com/go-chi/cors"

	"seno/internal/config"
	"seno/internal/handlers"
	appmw "seno/internal/middleware"
	"seno/internal/utils/jwt"
	"seno/pkg/response"
	"seno/web"
)

type Server struct {
	cfg        *config.Config
	httpServer *http.Server
}

func New(
	cfg *config.Config,
	jwtMgr *jwt.Manager,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	professorHandler *handlers.ProfessorHandler,
	classHandler *handlers.ClassHandler,
	roleChecker appmw.RoleChecker,
) *Server {
	r := chi.NewRouter()

	r.Use(chicors.Handler(chicors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.OK(w, "Serviço operacional", map[string]string{"status": "ok"})
	})

	r.Get("/routes", listRoutesHandler(r))

	r.Route("/api/v1", func(r chi.Router) {
		// Rotas públicas
		r.Group(func(r chi.Router) {
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/refresh", authHandler.Refresh)
		})

		// Rotas autenticadas
		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireAuth(jwtMgr))

			r.Get("/auth/me", authHandler.Me)
			r.Post("/auth/change-password", authHandler.ChangePassword)
			r.Get("/users", userHandler.List)
			r.Get("/users/{id}", userHandler.GetByID)

			r.Post("/classes/join", classHandler.Join)
			r.Get("/classes/mine", classHandler.Mine)
		})

		// Rotas de professor
		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireAuth(jwtMgr))
			r.Use(appmw.RequireRole(roleChecker, "professor"))

			r.Post("/classes", classHandler.Create)
			r.Get("/classes", classHandler.List)
		})

		// Rotas administrativas (superusuário)
		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireAuth(jwtMgr))
			r.Use(appmw.RequireRole(roleChecker, "super"))

			r.Post("/professors", professorHandler.Create)
			r.Get("/professors", professorHandler.List)
		})
	})

	// Interface web (arquivos estáticos embutidos em web/static)
	r.Handle("/*", web.Handler())

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.App.Host, cfg.App.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{cfg: cfg, httpServer: srv}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
