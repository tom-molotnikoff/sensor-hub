package api

import (
	"net/http"

	db "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
)

func (s *Server) ListRoles(c *gin.Context) {
	ctx := c.Request.Context()
	roles, err := s.roleService.ListRoles(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to list roles", "error": err.Error()})
		return
	}
	result := make([]gen.RoleInfo, len(roles))
	for i, r := range roles {
		result[i] = gen.RoleInfo{Id: r.Id, Name: r.Name}
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) ListPermissions(c *gin.Context) {
	ctx := c.Request.Context()
	perms, err := s.roleService.ListPermissions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to list permissions", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, convertPermissions(perms))
}

func (s *Server) GetRolePermissions(c *gin.Context, id int) {
	ctx := c.Request.Context()
	perms, err := s.roleService.ListPermissionsForRole(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to list role permissions", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, convertPermissions(perms))
}

func (s *Server) AssignPermission(c *gin.Context, id int) {
	ctx := c.Request.Context()
	var req gen.AssignPermissionJSONRequestBody
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	if err := s.roleService.AssignPermission(ctx, id, req.PermissionId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to assign permission", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (s *Server) RemovePermission(c *gin.Context, id int, pid int) {
	ctx := c.Request.Context()
	if err := s.roleService.RemovePermission(ctx, id, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to remove permission", "error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func convertPermissions(perms []db.PermissionInfo) []gen.PermissionInfo {
	result := make([]gen.PermissionInfo, len(perms))
	for i, p := range perms {
		desc := p.Description
		result[i] = gen.PermissionInfo{Id: p.Id, Name: p.Name, Description: &desc}
	}
	return result
}
