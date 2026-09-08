package api

import (
	"bytes"
	"encoding/json"
	"errors"
	db "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupAuthRouter() (*gin.Engine, *gin.RouterGroup, *Server, *MockAuthService) {
	mockService := new(MockAuthService)
	s := &Server{authService: mockService}
	router := gin.New()
	apiGroup := router.Group("/api")
	return router, apiGroup, s, mockService
}

func TestLoginHandler_Success(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.POST("/auth/login", s.Login)

	reqBody := gen.LoginRequest{Username: "user", Password: "password"}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.On("Login", mock.Anything, "user", "password", mock.Anything, mock.Anything).Return("token", "csrf", false, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "csrf")
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.POST("/auth/login", s.Login)

	reqBody := gen.LoginRequest{Username: "user", Password: "wrong"}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.On("Login", mock.Anything, "user", "wrong", mock.Anything, mock.Anything).Return("", "", false, errors.New("invalid credentials"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginHandler_TooManyAttempts(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.POST("/auth/login", s.Login)

	reqBody := gen.LoginRequest{Username: "user", Password: "password"}
	jsonBody, _ := json.Marshal(reqBody)

	err := &service.TooManyAttemptsError{RetryAfterSeconds: 60}
	mockService.On("Login", mock.Anything, "user", "password", mock.Anything, mock.Anything).Return("", "", false, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestLogoutHandler(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.POST("/auth/logout", s.Logout)

	mockService.On("Logout", mock.Anything, "valid-token").Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMeHandler_Success(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.GET("/auth/me", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1, Username: "me"})
		s.GetCurrentUser(c)
	})

	mockService.On("GetCSRFForToken", mock.Anything, "valid-token").Return("csrf-token", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "me")
}

func TestListSessionsHandler(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.GET("/auth/sessions", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.ListSessions(c)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{{Id: 100}}, nil)
	mockService.On("GetSessionIdForToken", mock.Anything, "valid-token").Return(int64(100), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "100")
}

func TestRevokeSessionHandler_OwnSession(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{{Id: 100, UserId: 1}}, nil)
	revoker := 1
	mockService.On("RevokeSessionByIdWithActor", mock.Anything, int64(100), &revoker, (*string)(nil)).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/100", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoginHandler_InvalidJSON(t *testing.T) {
	router, api, s, _ := setupAuthRouter()
	api.POST("/auth/login", s.Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString("invalid-json"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid request body")
}

func TestLoginHandler_MustChangePassword(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.POST("/auth/login", s.Login)

	reqBody := gen.LoginRequest{Username: "user", Password: "password"}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.On("Login", mock.Anything, "user", "password", mock.Anything, mock.Anything).Return("token", "csrf", true, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "must_change_password")
	assert.Contains(t, w.Body.String(), "true")
}

func TestLogoutHandler_MissingCookie(t *testing.T) {
	router, api, s, _ := setupAuthRouter()
	api.POST("/auth/logout", s.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	router.ServeHTTP(w, req)

	// Logout is idempotent - always returns OK even without cookie
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogoutHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.POST("/auth/logout", s.Logout)

	mockService.On("Logout", mock.Anything, "valid-token").Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	// Handler ignores service errors - always returns OK
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMeHandler_MissingCookie(t *testing.T) {
	router, api, s, _ := setupAuthRouter()
	api.GET("/auth/me", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1, Username: "me"})
		s.GetCurrentUser(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	router.ServeHTTP(w, req)

	// Handler returns OK with empty csrf_token when no cookie
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "me")
}

func TestMeHandler_CSRFError(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.GET("/auth/me", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1, Username: "me"})
		s.GetCurrentUser(c)
	})

	mockService.On("GetCSRFForToken", mock.Anything, "valid-token").Return("", errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	// Handler silently ignores CSRF errors and returns empty csrf_token
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "me")
}

func TestListSessionsHandler_ServiceError(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.GET("/auth/sessions", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		s.ListSessions(c)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRevokeSessionHandler_InvalidID(t *testing.T) {
	router, api, s, _ := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRevokeSessionHandler_MissingID(t *testing.T) {
	router, api, s, _ := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/", nil)
	router.ServeHTTP(w, req)

	// Gin won't match this route without :id, so it returns 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRevokeSessionHandler_NotOwnedSession(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1, Roles: []string{"user"}})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{{Id: 100, UserId: 1}}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/200", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRevokeSessionHandler_AdminRevokingOthers(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1, Roles: []string{"admin"}})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{{Id: 100, UserId: 1}}, nil)
	revoker := 1
	mockService.On("RevokeSessionByIdWithActor", mock.Anything, int64(200), &revoker, (*string)(nil)).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/200", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRevokeSessionHandler_ListError(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/100", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRevokeSessionHandler_RevokeError(t *testing.T) {
	router, api, s, mockService := setupAuthRouter()
	api.DELETE("/auth/sessions/:id", func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: 1})
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
			return
		}
		s.RevokeSession(c, id)
	})

	mockService.On("ListSessionsForUser", mock.Anything, 1).Return([]db.SessionInfo{{Id: 100, UserId: 1}}, nil)
	revoker := 1
	mockService.On("RevokeSessionByIdWithActor", mock.Anything, int64(100), &revoker, (*string)(nil)).Return(errors.New("db error"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/100", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
