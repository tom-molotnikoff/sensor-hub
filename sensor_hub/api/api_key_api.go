package api

import (
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

	if err := s.apiKeyService.UpdateApiKeyExpiry(ctx, id, user.Id, req.ExpiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to update expiry", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "expiry updated"})
}

func (s *Server) RevokeApiKey(c *gin.Context, id int) {
	ctx := c.Request.Context()
	user := c.MustGet("currentUser").(*gen.User)

	if err := s.apiKeyService.RevokeApiKey(ctx, id, user.Id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to revoke api key", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "api key revoked"})
}

func (s *Server) DeleteApiKey(c *gin.Context, id int) {
	ctx := c.Request.Context()
	user := c.MustGet("currentUser").(*gen.User)

	if err := s.apiKeyService.DeleteApiKey(ctx, id, user.Id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to delete api key", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "api key deleted"})
}
