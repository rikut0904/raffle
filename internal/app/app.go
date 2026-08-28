package app

import (
	"fmt"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"presentation-raffle/internal/infrastructure/auth"
	"presentation-raffle/internal/infrastructure/config"
	"presentation-raffle/internal/infrastructure/database"
	raffleinfra "presentation-raffle/internal/infrastructure/repository/raffle"
	userinfra "presentation-raffle/internal/infrastructure/repository/user"
	"presentation-raffle/internal/presentation/http/handler"
	"presentation-raffle/internal/presentation/http/router"
	"presentation-raffle/internal/usecase"
)

func Run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(session.Middleware(newSessionStore(cfg)))

	commonID := auth.NewCommonID(cfg)
	adminHandler, adminErr := buildAdminHandler(cfg, commonID)
	if adminErr != nil && adminHandler == nil {
		return adminErr
	}

	if adminErr != nil {
		e.Logger.Warn(adminErr)
	}

	router.Register(e, adminHandler, commonID)

	return e.Start(cfg.ServerAddr)
}

func newSessionStore(cfg config.Config) sessions.Store {
	if cfg.SessionEncryptionSecret != "" {
		return sessions.NewCookieStore(
			[]byte(cfg.SessionSecret),
			[]byte(cfg.SessionEncryptionSecret),
		)
	}

	return sessions.NewCookieStore([]byte(cfg.SessionSecret))
}

func buildAdminHandler(cfg config.Config, commonID *auth.CommonID) (*handler.AdminHandler, error) {
	db, err := database.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		return handler.NewUnavailableAdminHandler(commonID, fmt.Sprintf("database unavailable: %v", err)), err
	}

	userRepository := userinfra.NewPostgresUserRepository(db)
	raffleRepository := raffleinfra.NewPostgresRaffleRepository(db)
	if cfg.AutoMigrate {
		if err := database.Migrate(db); err != nil {
			return handler.NewUnavailableAdminHandler(commonID, fmt.Sprintf("migration failed: %v", err)), err
		}
	}

	adminUsecase := usecase.NewAdminUsecase(userRepository, raffleRepository)
	return handler.NewAdminHandler(adminUsecase, commonID), nil
}
