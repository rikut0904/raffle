package router

import (
	"log"
	"net/http"

	"presentation-raffle/internal/infrastructure/auth"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

func sessionOptions(maxAge int) *sessions.Options {
	return &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   maxAge,
		SameSite: http.SameSiteLaxMode,
	}
}

func getSessionUserUID(c echo.Context, commonID *auth.CommonID) (string, error) {
	sess, err := session.Get("session", c)
	if err != nil {
		log.Printf("Session error: %v", err)
		return "", err
	}

	userUID, ok := sess.Values["uid"].(string)
	if !ok || userUID == "" {
		log.Printf("No uid in session. values: %+v", sess.Values)
		delete(sess.Values, "uid")
		sess.Options = sessionOptions(-1)
		_ = sess.Save(c.Request(), c.Response())
		return "", echo.NewHTTPError(http.StatusUnauthorized, "ログインしてください")
	}

	commonIDCookie, err := c.Cookie("common_id_session")
	if err != nil || commonIDCookie.Value == "" {
		log.Printf("Common ID session cookie is missing")
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDにログインしてください")
	}
	if commonID == nil {
		log.Printf("Common ID client is unavailable")
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDにログインしてください")
	}
	status, err := commonID.CheckSession(c.Request().Context(), commonIDCookie.Value)
	if err != nil {
		log.Printf("Common ID session check failed: %v", err)
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDセッションの有効期限が切れています")
	}
	if status.CommonUserID != userUID {
		log.Printf("Common ID user mismatch: application_uid=%s common_id_uid=%s", userUID, status.CommonUserID)
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDセッションの有効期限が切れています")
	}

	return userUID, nil
}

func clearSession(c echo.Context, sess *sessions.Session) {
	sess.Values = map[any]any{}
	sess.Options = sessionOptions(-1)
	_ = sess.Save(c.Request(), c.Response())
}

func AuthMiddleware(commonID *auth.CommonID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userUID, err := getSessionUserUID(c, commonID)
			if err != nil {
				return err
			}

			c.Set("userUID", userUID)
			return next(c)
		}
	}
}

func PageAuthMiddleware(commonID *auth.CommonID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userUID, err := getSessionUserUID(c, commonID)
			if err != nil {
				return c.Redirect(http.StatusFound, "/auth/logout")
			}

			c.Set("userUID", userUID)
			return next(c)
		}
	}
}
