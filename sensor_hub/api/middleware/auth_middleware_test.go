package middleware

import (
	gen "example/sensorHub/gen"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuthRequired_ValidSession(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	user := &gen.User{Id: 1, Username: "testuser"}
	mockService.On("ValidateSession", mock.Anything, "valid-token").Return(user, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})

	AuthRequired()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	u, exists := c.Get("currentUser")
	assert.True(t, exists)
	assert.Equal(t, user, u)
}

func TestAuthRequired_NoCookie(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)

	AuthRequired()(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequired_InvalidSession(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	mockService.On("ValidateSession", mock.Anything, "invalid-token").Return(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "invalid-token"})

	AuthRequired()(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequired_MustChangePassword_Allowed(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	user := &gen.User{Id: 1, Username: "testuser", MustChangePassword: true}
	mockService.On("ValidateSession", mock.Anything, "valid-token").Return(user, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/users/password", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})

	AuthRequired()(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthRequired_MustChangePassword_Forbidden(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	user := &gen.User{Id: 1, Username: "testuser", MustChangePassword: true}
	mockService.On("ValidateSession", mock.Anything, "valid-token").Return(user, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/other", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})

	AuthRequired()(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func withApiKeyUser(t *testing.T, user *gen.User) {
	t.Helper()
	mockKeys := new(MockApiKeyService)
	if user == nil {
		mockKeys.On("ValidateApiKey", mock.Anything, "shk_key").Return(nil, nil)
	} else {
		mockKeys.On("ValidateApiKey", mock.Anything, "shk_key").Return(user, nil)
	}
	InitApiKeyMiddleware(mockKeys)
	t.Cleanup(func() { InitApiKeyMiddleware(nil) })
}

func apiKeyRequest(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Request.Header.Set("X-API-Key", "shk_key")
	return c, w
}

func TestAuthRequired_ApiKey(t *testing.T) {
	user := &gen.User{Id: 1, Username: "cli"}
	withApiKeyUser(t, user)
	c, w := apiKeyRequest("GET", "/api/sensors")

	AuthRequired()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, user, c.MustGet("currentUser"))
	assert.Equal(t, "api_key", c.GetString("authMethod"))
}

func TestAuthRequired_ApiKeyNotValid(t *testing.T) {
	withApiKeyUser(t, nil)
	c, w := apiKeyRequest("GET", "/api/sensors")

	AuthRequired()(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthRequired_ApiKeyMustChangePassword_Forbidden(t *testing.T) {
	withApiKeyUser(t, &gen.User{Id: 1, Username: "cli", MustChangePassword: true})
	c, w := apiKeyRequest("GET", "/api/sensors")

	AuthRequired()(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthRequired_ApiKeyMustChangePassword_Allowed(t *testing.T) {
	withApiKeyUser(t, &gen.User{Id: 1, Username: "cli", MustChangePassword: true})
	c, w := apiKeyRequest("PUT", "/api/users/password")

	AuthRequired()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, c.IsAborted())
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	user := &gen.User{Id: 1, Roles: []string{"admin"}}
	c.Set("currentUser", user)

	RequireAdmin()(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	user := &gen.User{Id: 1, Roles: []string{"user"}}
	c.Set("currentUser", user)

	RequireAdmin()(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAdmin_NoUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	RequireAdmin()(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
