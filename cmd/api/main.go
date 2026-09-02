package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"seno/internal/config"
	"seno/internal/database"
	"seno/internal/handlers"
	"seno/internal/repositories"
	"seno/internal/server"
	"seno/internal/services"
	"seno/internal/utils/jwt"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Falha ao carregar configuração: %v", err)
	}

	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("Falha ao conectar ao banco de dados: %v", err)
	}
	defer func() { _ = database.Close(db) }()

	if err := database.Migrate(context.Background(), db); err != nil {
		log.Fatalf("Falha ao aplicar migrações: %v", err)
	}

	userRepo := repositories.NewUserRepository(db)
	credentialRepo := repositories.NewCredentialRepository(db)
	roleRepo := repositories.NewRoleRepository(db)
	refreshRepo := repositories.NewRefreshTokenRepository(db)

	jwtMgr := jwt.NewManager(cfg.JWT)

	authService := services.NewAuthService(userRepo, credentialRepo, roleRepo, refreshRepo, jwtMgr)
	userService := services.NewUserService(userRepo, roleRepo)

	authHandler := handlers.NewAuthHandler(authService)
	userHandler := handlers.NewUserHandler(userService)

	srv := server.New(cfg, jwtMgr, authHandler, userHandler, roleRepo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Servidor iniciado em http://%s:%s (ambiente: %s)",
			cfg.App.Host, cfg.App.Port, cfg.App.Env)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar servidor: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Encerrando servidor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Erro ao encerrar servidor: %v", err)
	}

	log.Println("Servidor encerrado")
}
