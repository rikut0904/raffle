package router

import (
	"net/http"
	"strings"

	"presentation-raffle/internal/infrastructure/auth"

	"github.com/labstack/echo/v4"

	"presentation-raffle/internal/presentation/http/handler"
)

func Register(e *echo.Echo, adminHandler *handler.AdminHandler, commonID *auth.CommonID) {
	e.HTTPErrorHandler = customHTTPErrorHandler

	e.Static("/css", "css")
	e.Static("/js", "js")

	e.GET("/", func(c echo.Context) error {
		return c.File("html/index.html")
	})
	e.GET("/dashboard", func(c echo.Context) error {
		return c.File("html/dashboard.html")
	}, PageAuthMiddleware(commonID))
	e.GET("/raffle", func(c echo.Context) error {
		return c.File("html/raffle.html")
	}, PageAuthMiddleware(commonID))
	e.GET("/login", func(c echo.Context) error {
		return c.File("html/login.html")
	})
	e.GET("/signin", func(c echo.Context) error {
		return c.File("html/signin.html")
	})

	e.GET("/auth/login", adminHandler.BeginLogin)
	e.GET("/auth/signup", adminHandler.BeginSignup)
	e.GET("/auth/callback", adminHandler.Callback)
	e.GET("/auth/logout", adminHandler.Logout)
	e.GET("/auth/logout/callback", adminHandler.LogoutCallback)
	e.POST("/api/auth/logout", adminHandler.Logout)

	api := e.Group("/api/dashboard")
	api.Use(AuthMiddleware(commonID))
	api.GET("/me", adminHandler.GetMe)
	api.GET("/raffles", adminHandler.ListRaffles)
	api.GET("/raffles/:id", adminHandler.GetRaffle)
	api.POST("/raffles", adminHandler.SaveRaffle)
	api.DELETE("/raffles/:id", adminHandler.DeleteRaffle)
}

func customHTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := http.StatusText(code)

	if httpError, ok := err.(*echo.HTTPError); ok {
		code = httpError.Code
		if msg, ok := httpError.Message.(string); ok {
			message = msg
		}
	}

	if code == http.StatusNotFound && !strings.HasPrefix(c.Request().URL.Path, "/api/") {
		_ = c.File("html/404.html")
		return
	}

	_ = c.JSON(code, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
