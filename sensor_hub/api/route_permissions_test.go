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

// routesWithoutPermission are the routes that need no permission: public
// ones, and ones open to any signed-in user.
var routesWithoutPermission = map[string]bool{
	"GET /api/health":               true,
	"GET /api/openapi.yaml":         true,
	"GET /api/drivers":              true,
	"POST /api/auth/login":          true,
	"POST /api/auth/logout":         true,
	"GET /api/auth/me":              true,
	"GET /api/auth/sessions":        true,
	"DELETE /api/auth/sessions/:id": true,
	"PUT /api/users/password":       true,
	"GET /api/sensors/ws":           true,
	"GET /api/sensors/ws/:driver":   true,
}

// TestRoutePermissions_CoverEveryRoute makes every new route either name the
// permissions it needs or be listed as needing none.
func TestRoutePermissions_CoverEveryRoute(t *testing.T) {
	router := setupGenRouter(&Server{})
	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		registered[key] = true
		_, gated := routePermissions[key]
		assert.True(t, gated || routesWithoutPermission[key], "%s is in neither routePermissions nor routesWithoutPermission", key)
	}
	for key := range routePermissions {
		assert.True(t, registered[key], "routePermissions names %s, which is not a route", key)
	}
}

func TestRouteMiddleware_SavingAnAutomationNeedsControlSensorsToo(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware.InitAuthMiddleware(mockAuth)

	manager := &gen.User{
		Id:          1,
		Username:    "testuser",
		Roles:       []string{"custom"},
		Permissions: []string{"view_automations", "manage_automations"},
	}
	mockAuth.On("ValidateSession", mock.Anything, "valid-token").Return(manager, nil)

	router := setupGenRouter(&Server{authService: mockAuth})

	for _, request := range []struct{ method, path string }{
		{"POST", "/api/automations"},
		{"PUT", "/api/automations/3"},
		{"PUT", "/api/automations/3/enabled"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(request.method, request.path, nil)
		req.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, "%s %s", request.method, request.path)
	}
}
