package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCSRFMiddleware_GET_Bypass(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data", nil)

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddleware_Login_Bypass(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/auth/login", nil)

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddleware_ValidToken(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService) // Assuming this sets the global authService

	mockService.On("GetCSRFForToken", mock.Anything, "valid-token").Return("csrf-secret", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/data", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	c.Request.Header.Set("X-CSRF-Token", "csrf-secret")

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFMiddleware_MissingCookie(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/data", nil)

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCSRFMiddleware_MissingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/data", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFMiddleware_MismatchedToken(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	mockService.On("GetCSRFForToken", mock.Anything, "valid-token").Return("server-secret", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/data", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	c.Request.Header.Set("X-CSRF-Token", "client-secret")

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Only the exact token passes: a prefix, an extension or a same-length token
// differing in one byte is refused like any other mismatch.
func TestCSRFMiddleware_AcceptsOnlyTheExactToken(t *testing.T) {
	cases := map[string]int{
		"csrf-secret":       http.StatusOK,
		"csrf-secreT":       http.StatusForbidden,
		"csrf-secre":        http.StatusForbidden,
		"csrf-secret-extra": http.StatusForbidden,
	}
	for clientToken, want := range cases {
		t.Run(clientToken, func(t *testing.T) {
			mockService := new(MockAuthService)
			InitAuthMiddleware(mockService)
			mockService.On("GetCSRFForToken", mock.Anything, "valid-token").Return("csrf-secret", nil)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/data", nil)
			c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
			c.Request.Header.Set("X-CSRF-Token", clientToken)

			CSRFMiddleware()(c)

			assert.Equal(t, want, w.Code)
		})
	}
}

func TestCSRFMiddleware_ServiceError(t *testing.T) {
	mockService := new(MockAuthService)
	InitAuthMiddleware(mockService)

	mockService.On("GetCSRFForToken", mock.Anything, "valid-token").Return("", errors.New("db error"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/data", nil)
	c.Request.AddCookie(&http.Cookie{Name: "sensor_hub_session", Value: "valid-token"})
	c.Request.Header.Set("X-CSRF-Token", "csrf-secret")

	CSRFMiddleware()(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
