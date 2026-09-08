package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	db "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupApiKeyRouter(method, path string, handler gin.HandlerFunc, userID int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.Use(func(c *gin.Context) {
		c.Set("currentUser", &gen.User{Id: userID})
		c.Next()
	})
	apiGroup.Handle(method, path, handler)
	return router
}

func withApiKeyID(s *Server, h func(*gin.Context, int)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid key id"})
			return
		}
		h(c, id)
	}
}

// --- ListApiKeys ---

func TestListApiKeys_Success(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	keys := []db.ApiKey{{Id: 1, Name: "my-key", UserId: 1}}
	mockSvc.On("ListApiKeysForUser", mock.Anything, 1).Return(keys, nil)

	router := setupApiKeyRouter("GET", "/api-keys", s.ListApiKeys, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/api-keys", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "my-key")
	mockSvc.AssertExpectations(t)
}

func TestListApiKeys_Error(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("ListApiKeysForUser", mock.Anything, 1).Return([]db.ApiKey{}, errors.New("db error"))

	router := setupApiKeyRouter("GET", "/api-keys", s.ListApiKeys, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/api-keys", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- CreateApiKey ---

func TestCreateApiKey_Success(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	expires := time.Now().Add(24 * time.Hour)
	mockSvc.On("CreateApiKey", mock.Anything, "test-key", 1, mock.AnythingOfType("*time.Time")).Return("prefix.secret", nil)

	body, _ := json.Marshal(map[string]interface{}{"name": "test-key", "expires_at": expires})
	router := setupApiKeyRouter("POST", "/api-keys", s.CreateApiKey, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "prefix.secret")
	mockSvc.AssertExpectations(t)
}

func TestCreateApiKey_MissingName(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	body := []byte(`{"expires_at": null}`)
	router := setupApiKeyRouter("POST", "/api-keys", s.CreateApiKey, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateApiKey_Error(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("CreateApiKey", mock.Anything, "test-key", 1, mock.Anything).Return("", errors.New("db error"))

	body := []byte(`{"name":"test-key"}`)
	router := setupApiKeyRouter("POST", "/api-keys", s.CreateApiKey, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- UpdateApiKeyExpiry ---

func TestUpdateApiKeyExpiry_Success(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	expires := time.Now().Add(48 * time.Hour)
	mockSvc.On("UpdateApiKeyExpiry", mock.Anything, 5, 1, mock.AnythingOfType("*time.Time")).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"expires_at": expires})
	router := setupApiKeyRouter("PATCH", "/api-keys/:id/expiry", withApiKeyID(s, s.UpdateApiKeyExpiry), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/api-keys/5/expiry", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "expiry updated")
	mockSvc.AssertExpectations(t)
}

func TestUpdateApiKeyExpiry_InvalidID(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	router := setupApiKeyRouter("PATCH", "/api-keys/:id/expiry", withApiKeyID(s, s.UpdateApiKeyExpiry), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/api-keys/abc/expiry", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateApiKeyExpiry_Error(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("UpdateApiKeyExpiry", mock.Anything, 5, 1, mock.Anything).Return(errors.New("not found"))

	body := []byte(`{"expires_at": null}`)
	router := setupApiKeyRouter("PATCH", "/api-keys/:id/expiry", withApiKeyID(s, s.UpdateApiKeyExpiry), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/api-keys/5/expiry", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- RevokeApiKey ---

func TestRevokeApiKey_Success(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("RevokeApiKey", mock.Anything, 3, 1).Return(nil)

	router := setupApiKeyRouter("POST", "/api-keys/:id/revoke", withApiKeyID(s, s.RevokeApiKey), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/api-keys/3/revoke", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "api key revoked")
	mockSvc.AssertExpectations(t)
}

func TestRevokeApiKey_InvalidID(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	router := setupApiKeyRouter("POST", "/api-keys/:id/revoke", withApiKeyID(s, s.RevokeApiKey), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/api-keys/abc/revoke", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRevokeApiKey_Error(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("RevokeApiKey", mock.Anything, 3, 1).Return(errors.New("not found"))

	router := setupApiKeyRouter("POST", "/api-keys/:id/revoke", withApiKeyID(s, s.RevokeApiKey), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/api-keys/3/revoke", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}

// --- DeleteApiKey ---

func TestDeleteApiKey_Success(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("DeleteApiKey", mock.Anything, 7, 1).Return(nil)

	router := setupApiKeyRouter("DELETE", "/api-keys/:id", withApiKeyID(s, s.DeleteApiKey), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/api-keys/7", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "api key deleted")
	mockSvc.AssertExpectations(t)
}

func TestDeleteApiKey_InvalidID(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	router := setupApiKeyRouter("DELETE", "/api-keys/:id", withApiKeyID(s, s.DeleteApiKey), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/api-keys/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteApiKey_Error(t *testing.T) {
	mockSvc := new(MockApiKeyService)
	s := &Server{apiKeyService: mockSvc}

	mockSvc.On("DeleteApiKey", mock.Anything, 7, 1).Return(errors.New("forbidden"))

	router := setupApiKeyRouter("DELETE", "/api-keys/:id", withApiKeyID(s, s.DeleteApiKey), 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/api-keys/7", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}
