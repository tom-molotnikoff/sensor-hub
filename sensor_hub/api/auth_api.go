package api

import (
	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) Login(c *gin.Context) {
	ctx := c.Request.Context()
	var req gen.LoginRequest
	if err := c.BindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	token, csrf, mustChange, err := s.authService.Login(ctx, req.Username, req.Password, ip, userAgent)
	if err != nil {
		switch e := err.(type) {
		case *service.TooManyAttemptsError:
			slog.Warn("rejecting login: too many attempts",
				"ip", c.ClientIP(),
				"username", req.Username,
				"retry_after", e.RetryAfterSeconds,
				"failed_by_user", e.FailedByUser,
				"failed_by_ip", e.FailedByIP,
				"threshold", e.Threshold,
				"exponent", e.Exponent,
			)
			c.Header("Retry-After", fmt.Sprintf("%d", e.RetryAfterSeconds))
			c.IndentedJSON(http.StatusTooManyRequests, gin.H{"message": "too many failed login attempts, retry later", "retry_after": e.RetryAfterSeconds, "failed_by_user": e.FailedByUser, "failed_by_ip": e.FailedByIP, "threshold": e.Threshold, "exponent": e.Exponent})
			return
		default:
			c.IndentedJSON(http.StatusUnauthorized, gin.H{"message": "invalid credentials"})
			return
		}
	}
	cookieName := "sensor_hub_session"
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionCookieName != "" {
		cookieName = cfg.AuthSessionCookieName
	}
	secure := false
	if os.Getenv("SENSOR_HUB_PRODUCTION") == "true" || c.Request.TLS != nil {
		secure = true
	}

	ttlMinutes := 60 * 24 * 30
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionTTLMinutes > 0 {
		ttlMinutes = cfg.AuthSessionTTLMinutes
	}
	expires := time.Now().Add(time.Duration(ttlMinutes) * time.Minute)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	c.IndentedJSON(http.StatusOK, gen.LoginResponse{MustChangePassword: &mustChange, CsrfToken: &csrf})
}

func (s *Server) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	cookieName := "sensor_hub_session"
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionCookieName != "" {
		cookieName = cfg.AuthSessionCookieName
	}
	token, err := c.Cookie(cookieName)
	secure := false
	if os.Getenv("SENSOR_HUB_PRODUCTION") == "true" || c.Request.TLS != nil {
		secure = true
	}
	if err == nil && token != "" {
		_ = s.authService.Logout(ctx, token)
		http.SetCookie(c.Writer, &http.Cookie{Name: cookieName, Value: "", Path: "/", Expires: time.Unix(0, 0), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	}
	c.Status(http.StatusOK)
}

func (s *Server) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()
	u, exists := c.Get("currentUser")
	if !exists {
		c.Status(http.StatusUnauthorized)
		return
	}
	user, ok := u.(*gen.User)
	if !ok || user == nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	cookieName := "sensor_hub_session"
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionCookieName != "" {
		cookieName = cfg.AuthSessionCookieName
	}
	token, _ := c.Cookie(cookieName)
	var csrfPtr *string
	if token != "" {
		if t, err := s.authService.GetCSRFForToken(ctx, token); err == nil {
			csrfPtr = &t
		}
	}
	c.IndentedJSON(http.StatusOK, gen.MeResponse{User: user, CsrfToken: csrfPtr})
}

func (s *Server) ListSessions(c *gin.Context) {
	ctx := c.Request.Context()
	u, _ := c.Get("currentUser")
	user := u.(*gen.User)
	sessions, err := s.authService.ListSessionsForUser(ctx, user.Id)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to list sessions", "error": err.Error()})
		return
	}
	cookieName := "sensor_hub_session"
	if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionCookieName != "" {
		cookieName = cfg.AuthSessionCookieName
	}
	currentToken, _ := c.Cookie(cookieName)
	var currentSessionId int64
	if currentToken != "" {
		if sid, err := s.authService.GetSessionIdForToken(ctx, currentToken); err == nil {
			currentSessionId = sid
		}
	}

	out := make([]gen.SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		isCurrent := sess.Id == currentSessionId
		id := sess.Id
		uid := sess.UserId
		out = append(out, gen.SessionInfo{
			Id:             &id,
			UserId:         &uid,
			CreatedAt:      &sess.CreatedAt,
			ExpiresAt:      &sess.ExpiresAt,
			LastAccessedAt: &sess.LastAccessedAt,
			IpAddress:      &sess.IpAddress,
			UserAgent:      &sess.UserAgent,
			Current:        &isCurrent,
		})
	}
	c.IndentedJSON(http.StatusOK, out)
}

func (s *Server) RevokeSession(c *gin.Context, id int64) {
	ctx := c.Request.Context()

	u, _ := c.Get("currentUser")
	user := u.(*gen.User)

	sessions, err := s.authService.ListSessionsForUser(ctx, user.Id)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to list sessions", "error": err.Error()})
		return
	}
	owned := false
	for _, s := range sessions {
		if s.Id == id {
			owned = true
			break
		}
	}
	isAdmin := false
	for _, r := range user.Roles {
		if r == "admin" {
			isAdmin = true
			break
		}
	}
	if !owned && !isAdmin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	revokerId := user.Id
	if err := s.authService.RevokeSessionByIdWithActor(ctx, id, &revokerId, nil); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to revoke session", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}
