package handler

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"presentation-raffle/internal/domain/entity"
	"presentation-raffle/internal/infrastructure/auth"
	"presentation-raffle/internal/usecase"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type AdminHandler struct {
	usecase      *usecase.AdminUsecase
	commonID     *auth.CommonID
	errorMessage string
}

const (
	loginPendingStateKey    = "common_id_login_state"
	loginPendingVerifierKey = "common_id_login_verifier"
	loginPendingExpiryKey   = "common_id_login_expiry"
	logoutPendingStateKey   = "common_id_logout_state"
)

func secureRequest(c echo.Context) bool {
	if c.IsTLS() {
		return true
	}
	forwardedProto := strings.TrimSpace(strings.Split(c.Request().Header.Get("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(forwardedProto, "https")
}

func NewAdminHandler(usecase *usecase.AdminUsecase, commonID *auth.CommonID) *AdminHandler {
	return &AdminHandler{usecase: usecase, commonID: commonID}
}

func NewUnavailableAdminHandler(commonID *auth.CommonID, message string) *AdminHandler {
	return &AdminHandler{commonID: commonID, errorMessage: message}
}

func (h *AdminHandler) BeginLogin(c echo.Context) error  { return h.beginAuth(c, "login") }
func (h *AdminHandler) BeginSignup(c echo.Context) error { return h.beginAuth(c, "signup") }
func (h *AdminHandler) beginAuth(c echo.Context, intent string) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}
	target, pending, err := h.commonID.AuthorizeURL(intent)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "認証を開始できません")
	}
	sess, err := session.Get("session", c)
	if err != nil {
		log.Printf("Failed to load session before Common ID authentication: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "セッションを確認できません")
	}
	sess.Values[loginPendingStateKey] = pending.State
	sess.Values[loginPendingVerifierKey] = pending.Verifier
	sess.Values[loginPendingExpiryKey] = strconv.FormatInt(pending.ExpiresAt.Unix(), 10)
	sess.Options.Path = "/"
	sess.Options.MaxAge = 600
	sess.Options.HttpOnly = true
	sess.Options.Secure = secureRequest(c)
	sess.Options.SameSite = http.SameSiteLaxMode
	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "認証を開始できません")
	}
	return c.Redirect(http.StatusFound, target)
}

func (h *AdminHandler) Callback(c echo.Context) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}
	if c.QueryParam("error") != "" {
		return c.Redirect(http.StatusFound, "/login?error=cancelled")
	}
	sess, _ := session.Get("session", c)
	pending, err := pendingFromSession(sess)
	if err != nil {
		h.clearSession(c)
		return c.Redirect(http.StatusFound, "/login?error=session_expired")
	}
	user, err := h.commonID.Exchange(c.Request().Context(), url.Values(c.QueryParams()), pending)
	if err != nil {
		log.Printf("Common ID callback exchange failed: %v", err)
		h.clearSession(c)
		return c.Redirect(http.StatusFound, "/login?error=session_expired")
	}
	saved, err := h.usecase.SyncUser(c.Request().Context(), user)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ユーザー情報を保存できません")
	}

	sess, _ = session.Get("session", c)
	delete(sess.Values, loginPendingStateKey)
	delete(sess.Values, loginPendingVerifierKey)
	delete(sess.Values, loginPendingExpiryKey)
	sess.Options.Path = "/"
	sess.Options.MaxAge = 86400 * 7 // 7 days
	sess.Options.HttpOnly = true
	sess.Options.Secure = secureRequest(c)
	sess.Options.SameSite = http.SameSiteLaxMode
	sess.Values["uid"] = saved.UID
	if err := sess.Save(c.Request(), c.Response()); err != nil {
		log.Printf("Failed to save session: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save session")
	}

	return c.Redirect(http.StatusFound, "/dashboard")
}

func (h *AdminHandler) clearSession(c echo.Context) {
	sess, err := session.Get("session", c)
	if err != nil {
		return
	}
	sess.Values = map[any]any{}
	sess.Options = &sessions.Options{Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(c), SameSite: http.SameSiteLaxMode}
	_ = sess.Save(c.Request(), c.Response())
}

func (h *AdminHandler) Logout(c echo.Context) error {
	sess, _ := session.Get("session", c)
	if h.errorMessage != "" {
		sess.Values = map[any]any{}
		sess.Options = &sessions.Options{Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(c), SameSite: http.SameSiteLaxMode}
		_ = sess.Save(c.Request(), c.Response())
		return c.Redirect(http.StatusFound, "/login")
	}
	logoutURL, state, err := h.commonID.LogoutURL()
	if err != nil {
		return c.Redirect(http.StatusFound, "/login")
	}
	sess.Values = map[any]any{logoutPendingStateKey: state}
	sess.Options = &sessions.Options{Path: "/", MaxAge: 600, HttpOnly: true, Secure: secureRequest(c), SameSite: http.SameSiteLaxMode}
	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to clear session")
	}
	return c.Redirect(http.StatusFound, logoutURL)
}

func (h *AdminHandler) LogoutCallback(c echo.Context) error {
	sess, _ := session.Get("session", c)
	expectedState, _ := sess.Values[logoutPendingStateKey].(string)
	_ = h.commonID.ValidateLogout(url.Values(c.QueryParams()), expectedState)
	sess.Values = map[any]any{}
	sess.Options = &sessions.Options{Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(c), SameSite: http.SameSiteLaxMode}
	_ = sess.Save(c.Request(), c.Response())
	return c.Redirect(http.StatusFound, "/login")
}

func pendingFromSession(sess *sessions.Session) (auth.Pending, error) {
	state, stateOK := sess.Values[loginPendingStateKey].(string)
	verifier, verifierOK := sess.Values[loginPendingVerifierKey].(string)
	expiry, expiryOK := sess.Values[loginPendingExpiryKey].(string)
	if !stateOK || !verifierOK || !expiryOK || state == "" || verifier == "" {
		return auth.Pending{}, fmt.Errorf("login pending data is missing")
	}
	expiresAt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		return auth.Pending{}, fmt.Errorf("login pending data is invalid")
	}
	return auth.Pending{State: state, Verifier: verifier, ExpiresAt: time.Unix(expiresAt, 0)}, nil
}

func (h *AdminHandler) GetMe(c echo.Context) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}

	userUID := c.Get("userUID").(string)
	user, err := h.usecase.GetUser(c.Request().Context(), userUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}
	return c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) ListRaffles(c echo.Context) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}

	userUID := c.Get("userUID").(string)
	raffles, err := h.usecase.ListRaffles(c.Request().Context(), userUID)
	if err != nil {
		log.Printf("ListRaffles error for uid %s: %v", userUID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, raffles)
}

func (h *AdminHandler) SaveRaffle(c echo.Context) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}

	userUID := c.Get("userUID").(string)
	var k entity.Raffle
	if err := c.Bind(&k); err != nil {
		fmt.Printf("Bind error: %v\n", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}
	k.UserUID = userUID
	saved, err := h.usecase.SaveRaffle(c.Request().Context(), k)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, saved)
}

func (h *AdminHandler) DeleteRaffle(c echo.Context) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}

	userUID := c.Get("userUID").(string)
	id := c.Param("id")
	err := h.usecase.DeleteRaffle(c.Request().Context(), id, userUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) GetRaffle(c echo.Context) error {
	if h.errorMessage != "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, h.errorMessage)
	}

	userUID := c.Get("userUID").(string)
	id := c.Param("id")
	raffle, err := h.usecase.GetRaffle(c.Request().Context(), userUID, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Raffle not found")
	}
	return c.JSON(http.StatusOK, raffle)
}
