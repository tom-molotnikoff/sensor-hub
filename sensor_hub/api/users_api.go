package api

import (
	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	var req gen.CreateUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	roles := []string{}
	if req.Roles != nil {
		roles = *req.Roles
	}
	user := gen.User{Username: req.Username, Email: email, Roles: roles}
	id, err := s.userService.CreateUser(ctx, user, req.Password)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to create user", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	users, err := s.userService.ListUsers(ctx)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to list users", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, users)
}

func (s *Server) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()
	var req gen.ChangePasswordRequest
	if err := c.BindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	currentUserObj, _ := c.Get("currentUser")
	currentUser := currentUserObj.(*gen.User)
	if currentUser == nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	targetUserId := currentUser.Id
	if req.UserId != nil {
		targetUserId = *req.UserId
	}
	if currentUser.Id != targetUserId {
		isAdmin := false
		for _, r := range currentUser.Roles {
			if r == "admin" {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			c.Status(http.StatusForbidden)
			return
		}
	}

	keepToken := ""
	if currentUser.Id == targetUserId {
		cookieName := "sensor_hub_session"
		if cfg := appProps.AppConfig(); cfg != nil && cfg.AuthSessionCookieName != "" {
			cookieName = cfg.AuthSessionCookieName
		}
		if t, err := c.Cookie(cookieName); err == nil {
			keepToken = t
		}
	}
	if err := s.userService.ChangePassword(ctx, targetUserId, req.NewPassword, keepToken); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to change password", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (s *Server) DeleteUser(c *gin.Context, id int) {
	ctx := c.Request.Context()

	currentUserObj, _ := c.Get("currentUser")
	currentUser := currentUserObj.(*gen.User)
	isAdmin := false
	for _, r := range currentUser.Roles {
		if r == "admin" {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		c.Status(http.StatusForbidden)
		return
	}

	if currentUser.Id == id {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "cannot delete current user"})
		return
	}
	if err := s.userService.DeleteUser(ctx, id); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to delete user", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (s *Server) SetMustChangePassword(c *gin.Context, id int) {
	ctx := c.Request.Context()
	var req gen.SetMustChangePasswordJSONRequestBody
	if err := c.BindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	currentUserObj, _ := c.Get("currentUser")
	currentUser := currentUserObj.(*gen.User)
	if currentUser == nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	if currentUser.Id != id {
		isAdmin := false
		for _, r := range currentUser.Roles {
			if r == "admin" {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			c.Status(http.StatusForbidden)
			return
		}
	}
	if err := s.userService.SetMustChangeFlag(ctx, id, req.MustChange); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to update user flag", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (s *Server) SetUserRoles(c *gin.Context, id int) {
	ctx := c.Request.Context()
	var req gen.SetUserRolesJSONRequestBody
	if err := c.BindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	currentUserObj, _ := c.Get("currentUser")
	currentUser := currentUserObj.(*gen.User)
	if currentUser == nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	isAdmin := false
	for _, r := range currentUser.Roles {
		if r == "admin" {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		c.Status(http.StatusForbidden)
		return
	}
	if err := s.userService.SetUserRoles(ctx, id, req.Roles); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "failed to set roles", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}
