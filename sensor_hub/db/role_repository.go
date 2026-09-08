package database

import (
	"context"
	"fmt"
	"log/slog"
)

type RoleRepository interface {
	GetPermissionsForUser(ctx context.Context, userId int) ([]string, error)
	GetAllRoles(ctx context.Context) ([]RoleInfo, error)
	GetAllPermissions(ctx context.Context) ([]PermissionInfo, error)
	GetPermissionsForRole(ctx context.Context, roleId int) ([]PermissionInfo, error)
	AssignPermissionToRole(ctx context.Context, roleId int, permissionId int) error
	RemovePermissionFromRole(ctx context.Context, roleId int, permissionId int) error
}

type RoleInfo struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type PermissionInfo struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SqlRoleRepository struct {
	db     *Handles
	logger *slog.Logger
}

func NewRoleRepository(db *Handles, logger *slog.Logger) *SqlRoleRepository {
	return &SqlRoleRepository{db: db, logger: logger.With("component", "role_repository")}
}

func (r *SqlRoleRepository) GetPermissionsForUser(ctx context.Context, userId int) ([]string, error) {
	query := `SELECT p.name FROM permissions p
	JOIN role_permissions rp ON p.id = rp.permission_id
	JOIN user_roles ur ON rp.role_id = ur.role_id
	WHERE ur.user_id = ?`
	rows, err := r.db.Reader.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("error querying permissions for user: %w", err)
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("error scanning permission row: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permission rows: %w", err)
	}
	return perms, nil
}

func (r *SqlRoleRepository) GetAllRoles(ctx context.Context) ([]RoleInfo, error) {
	rows, err := r.db.Reader.QueryContext(ctx, "SELECT id, name FROM roles")
	if err != nil {
		return nil, fmt.Errorf("error querying roles: %w", err)
	}
	defer rows.Close()
	var out []RoleInfo
	for rows.Next() {
		var ri RoleInfo
		if err := rows.Scan(&ri.Id, &ri.Name); err != nil {
			return nil, fmt.Errorf("error scanning role row: %w", err)
		}
		out = append(out, ri)
	}
	return out, nil
}

func (r *SqlRoleRepository) GetAllPermissions(ctx context.Context) ([]PermissionInfo, error) {
	rows, err := r.db.Reader.QueryContext(ctx, "SELECT id, name, description FROM permissions")
	if err != nil {
		return nil, fmt.Errorf("error querying permissions: %w", err)
	}
	defer rows.Close()
	var out []PermissionInfo
	for rows.Next() {
		var p PermissionInfo
		if err := rows.Scan(&p.Id, &p.Name, &p.Description); err != nil {
			return nil, fmt.Errorf("error scanning permission row: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *SqlRoleRepository) GetPermissionsForRole(ctx context.Context, roleId int) ([]PermissionInfo, error) {
	rows, err := r.db.Reader.QueryContext(ctx, "SELECT p.id, p.name, p.description FROM permissions p JOIN role_permissions rp ON p.id = rp.permission_id WHERE rp.role_id = ?", roleId)
	if err != nil {
		return nil, fmt.Errorf("error querying role permissions: %w", err)
	}
	defer rows.Close()
	var out []PermissionInfo
	for rows.Next() {
		var p PermissionInfo
		if err := rows.Scan(&p.Id, &p.Name, &p.Description); err != nil {
			return nil, fmt.Errorf("error scanning role permission row: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *SqlRoleRepository) AssignPermissionToRole(ctx context.Context, roleId int, permissionId int) error {
	_, err := r.db.Writer.ExecContext(ctx, "INSERT OR IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)", roleId, permissionId)
	if err != nil {
		return fmt.Errorf("error assigning permission to role: %w", err)
	}
	return nil
}

func (r *SqlRoleRepository) RemovePermissionFromRole(ctx context.Context, roleId int, permissionId int) error {
	_, err := r.db.Writer.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id = ? AND permission_id = ?", roleId, permissionId)
	if err != nil {
		return fmt.Errorf("error removing permission from role: %w", err)
	}
	return nil
}
