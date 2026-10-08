package api

import (
	"errors"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreateApiKey(c *gin.Context) {
	ctx := c.Request.Context()
	var req gen.CreateApiKeyJSONRequestBody
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	user := c.MustGet("currentUser").(*gen.User)

	fullKey, err := s.apiKeyService.CreateApiKey(ctx, req.Name, user.Id, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create api key", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"key":     fullKey,
		"message": "Store this key securely. It will not be shown again.",
	})
}

func (s *Server) ListApiKeys(c *gin.Context) {
	ctx := c.Request.Context()
	user := c.MustGet("currentUser").(*gen.User)

	keys, err := s.apiKeyService.ListApiKeysForUser(ctx, user.Id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to list api keys", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, keys)
}

func (s *Server) UpdateApiKeyExpiry(c *gin.Context, id int) {
	var req gen.UpdateApiKeyExpiryJSONRequestBody
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	ctx := c.Request.Context()
	user := c.MustGet("currentUser").(*gen.User)

	if err := s.apiKeyService.UpdateApiKeyExpiry(ctx, id, user, req.ExpiresAt); err != nil {
		respondApiKeyError(c, err, "failed to update expiry")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "expiry updated"})
}

func (s *Server) RevokeApiKey(c *gin.Context, id int) {
	ctx := c.Request.Context()
	user := c.MustGet("currentUser").(*gen.User)

	if err := s.apiKeyService.RevokeApiKey(ctx, id, user); err != nil {
		respondApiKeyError(c, err, "failed to revoke api key")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "api key revoked"})
}

func (s *Server) DeleteApiKey(c *gin.Context, id int) {
	ctx := c.Request.Context()
	user := c.MustGet("currentUser").(*gen.User)

	if err := s.apiKeyService.DeleteApiKey(ctx, id, user); err != nil {
		respondApiKeyError(c, err, "failed to delete api key")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "api key deleted"})
}

// respondApiKeyError answers 404 for a key the caller may not touch, exactly
// as for one that does not exist, so another user's key ids are not revealed.
func respondApiKeyError(c *gin.Context, err error, message string) {
	if errors.Is(err, database.ErrApiKeyNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"message": "api key not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": message, "error": err.Error()})
}
