package router

import (
	"log"
	"net/http"
	"strings"

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
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
}

func secureRequest(c echo.Context) bool {
	if c.IsTLS() {
		return true
	}
	forwardedProto := strings.TrimSpace(strings.Split(c.Request().Header.Get("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(forwardedProto, "https")
}

type sessionFailureKind uint8

const (
	sessionFailureNone sessionFailureKind = iota
	sessionFailureAppSession
	sessionFailureCommonID
)

func getSessionUserUID(c echo.Context, commonID *auth.CommonID) (string, error, sessionFailureKind) {
	sess, err := session.Get("session", c)
	if err != nil {
		log.Printf("Session error: %v", err)
		return "", err, sessionFailureAppSession
	}

	userUID, ok := sess.Values["uid"].(string)
	if !ok || userUID == "" {
		log.Printf("No uid in session. values: %+v", sess.Values)
		delete(sess.Values, "uid")
		sess.Options = sessionOptions(-1)
		sess.Options.Secure = secureRequest(c)
		_ = sess.Save(c.Request(), c.Response())
		return "", echo.NewHTTPError(http.StatusUnauthorized, "ログインしてください"), sessionFailureAppSession
	}

	commonIDCookie, err := c.Cookie("common_id_session")
	if err != nil || commonIDCookie.Value == "" {
		log.Printf("Common ID session cookie is missing")
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDにログインしてください"), sessionFailureCommonID
	}
	if commonID == nil {
		log.Printf("Common ID client is unavailable")
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDにログインしてください"), sessionFailureCommonID
	}
	status, err := commonID.CheckSession(c.Request().Context(), commonIDCookie.Value)
	if err != nil {
		log.Printf("Common ID session check failed: %v", err)
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDセッションの有効期限が切れています"), sessionFailureCommonID
	}
	if status.CommonUserID != userUID {
		log.Printf("Common ID user mismatch: application_uid=%s common_id_uid=%s", userUID, status.CommonUserID)
		clearSession(c, sess)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Common IDセッションの有効期限が切れています"), sessionFailureCommonID
	}

	return userUID, nil, sessionFailureNone
}

func clearSession(c echo.Context, sess *sessions.Session) {
	sess.Values = map[any]any{}
	sess.Options = sessionOptions(-1)
	sess.Options.Secure = secureRequest(c)
	_ = sess.Save(c.Request(), c.Response())
}

func AuthMiddleware(commonID *auth.CommonID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userUID, err, _ := getSessionUserUID(c, commonID)
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
			userUID, err, failureKind := getSessionUserUID(c, commonID)
			if err != nil {
				if failureKind == sessionFailureCommonID {
					return c.Redirect(http.StatusFound, "/auth/logout")
				}
				return c.Redirect(http.StatusFound, "/login")
			}

			c.Set("userUID", userUID)
			return next(c)
		}
	}
}
