package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"example/sensorHub/api/middleware"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupGenRouter builds a router using gen.RegisterHandlersWithOptions, which is
// the new registration approach that replaces the hand-written Register*Routes calls.
func setupGenRouter(server *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api")
	gen.RegisterHandlersWithOptions(apiGroup, server, gen.GinServerOptions{
		Middlewares: []gen.MiddlewareFunc{RouteAuthAndPermissionMiddleware()},
	})
	return router
}

// TestRouteMiddleware_BlocksUnauthenticatedAccessToProtectedRoute verifies that
// a request to an auth-required endpoint without a session cookie returns 401.
func TestRouteMiddleware_BlocksUnauthenticatedAccessToProtectedRoute(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	router := setupGenRouter(&Server{authService: mockAuth})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRouteMiddleware_AllowsPublicRouteWithoutAuth verifies that a public endpoint
// (GetHealth) is accessible without any authentication.
func TestRouteMiddleware_AllowsPublicRouteWithoutAuth(t *testing.T) {
	router := setupGenRouter(&Server{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouteMiddleware_BlocksInsufficientPermission verifies that an authenticated
// user without the required permission receives 403 on a permission-gated route.
func TestRouteMiddleware_BlocksInsufficientPermission(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	// Return a user with NO permissions (empty slice populated on the user object)
	userWithNoPerms := &gen.User{
		Id:          1,
		Username:    "testuser",
		Roles:       []string{"viewer"},
		Permissions: []string{}, // no permissions
	}
	mockAuth.On("ValidateSession", mock.Anything, "valid-token").Return(userWithNoPerms, nil)

	router := setupGenRouter(&Server{authService: mockAuth})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRouteMiddleware_PropertyDefinitionsRequiresViewProperties verifies the
// definitions endpoint fails permission checks the same way the existing
// properties endpoints do.
func TestRouteMiddleware_PropertyDefinitionsRequiresViewProperties(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	userWithNoPerms := &gen.User{
		Id:          1,
		Username:    "testuser",
		Roles:       []string{"viewer"},
		Permissions: []string{},
	}
	mockAuth.On("ValidateSession", mock.Anything, "valid-token").Return(userWithNoPerms, nil)

	router := setupGenRouter(&Server{authService: mockAuth})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/properties/definitions", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRouteMiddleware_PropertyDefinitionsRequiresAuthentication(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	router := setupGenRouter(&Server{authService: mockAuth})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/properties/definitions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRouteMiddleware_BlocksInsufficientPermissionForSendSensorCommand(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	userWithNoPerms := &gen.User{
		Id:          1,
		Username:    "testuser",
		Roles:       []string{"viewer"},
		Permissions: []string{},
	}
	mockAuth.On("ValidateSession", mock.Anything, "valid-token").Return(userWithNoPerms, nil)

	router := setupGenRouter(&Server{authService: mockAuth})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/sensors/7/command", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRouteMiddleware_BlocksInsufficientPermissionForGetSensorCommandHistory(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	userWithNoPerms := &gen.User{
		Id:          1,
		Username:    "testuser",
		Roles:       []string{"viewer"},
		Permissions: []string{},
	}
	mockAuth.On("ValidateSession", mock.Anything, "valid-token").Return(userWithNoPerms, nil)

	router := setupGenRouter(&Server{authService: mockAuth})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sensors/by-id/7/commands", nil)
	req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
