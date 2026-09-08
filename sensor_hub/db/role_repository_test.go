package database

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// GetPermissionsForUser tests
// ============================================================================

func TestRoleRepository_GetPermissionsForUser_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.name FROM permissions p").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).
			AddRow("read:sensors").
			AddRow("write:sensors").
			AddRow("admin:users"))

	perms, err := repo.GetPermissionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, perms, 3)
	assert.Contains(t, perms, "read:sensors")
	assert.Contains(t, perms, "write:sensors")
	assert.Contains(t, perms, "admin:users")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetPermissionsForUser_NoPermissions(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.name FROM permissions p").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	perms, err := repo.GetPermissionsForUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.Empty(t, perms)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetPermissionsForUser_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.name FROM permissions p").
		WithArgs(1).
		WillReturnError(errors.New("database error"))

	perms, err := repo.GetPermissionsForUser(context.Background(), 1)

	assert.Error(t, err)
	assert.Nil(t, perms)
	assert.Contains(t, err.Error(), "error querying permissions for user")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetAllRoles tests
// ============================================================================

func TestRoleRepository_GetAllRoles_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, name FROM roles").
		WillReturnRows(sqlmock.NewRows(roleColumns).
			AddRow(1, "admin").
			AddRow(2, "user").
			AddRow(3, "viewer"))

	roles, err := repo.GetAllRoles(context.Background())

	assert.NoError(t, err)
	assert.Len(t, roles, 3)
	assert.Equal(t, 1, roles[0].Id)
	assert.Equal(t, "admin", roles[0].Name)
	assert.Equal(t, "user", roles[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetAllRoles_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, name FROM roles").
		WillReturnRows(sqlmock.NewRows(roleColumns))

	roles, err := repo.GetAllRoles(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, roles)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetAllRoles_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, name FROM roles").
		WillReturnError(errors.New("database error"))

	roles, err := repo.GetAllRoles(context.Background())

	assert.Error(t, err)
	assert.Nil(t, roles)
	assert.Contains(t, err.Error(), "error querying roles")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetAllPermissions tests
// ============================================================================

func TestRoleRepository_GetAllPermissions_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, name, description FROM permissions").
		WillReturnRows(sqlmock.NewRows(permissionColumns).
			AddRow(1, "read:sensors", "Read sensor data").
			AddRow(2, "write:sensors", "Modify sensors").
			AddRow(3, "admin:users", "Manage users"))

	perms, err := repo.GetAllPermissions(context.Background())

	assert.NoError(t, err)
	assert.Len(t, perms, 3)
	assert.Equal(t, 1, perms[0].Id)
	assert.Equal(t, "read:sensors", perms[0].Name)
	assert.Equal(t, "Read sensor data", perms[0].Description)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetAllPermissions_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, name, description FROM permissions").
		WillReturnRows(sqlmock.NewRows(permissionColumns))

	perms, err := repo.GetAllPermissions(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, perms)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetAllPermissions_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT id, name, description FROM permissions").
		WillReturnError(errors.New("database error"))

	perms, err := repo.GetAllPermissions(context.Background())

	assert.Error(t, err)
	assert.Nil(t, perms)
	assert.Contains(t, err.Error(), "error querying permissions")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// GetPermissionsForRole tests
// ============================================================================

func TestRoleRepository_GetPermissionsForRole_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.id, p.name, p.description FROM permissions p JOIN role_permissions rp").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(permissionColumns).
			AddRow(1, "read:sensors", "Read sensor data").
			AddRow(2, "write:sensors", "Modify sensors"))

	perms, err := repo.GetPermissionsForRole(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, perms, 2)
	assert.Equal(t, "read:sensors", perms[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetPermissionsForRole_NoPermissions(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.id, p.name, p.description FROM permissions p JOIN role_permissions rp").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(permissionColumns))

	perms, err := repo.GetPermissionsForRole(context.Background(), 1)

	assert.NoError(t, err)
	assert.Empty(t, perms)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetPermissionsForRole_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.id, p.name, p.description FROM permissions p JOIN role_permissions rp").
		WithArgs(1).
		WillReturnError(errors.New("database error"))

	perms, err := repo.GetPermissionsForRole(context.Background(), 1)

	assert.Error(t, err)
	assert.Nil(t, perms)
	assert.Contains(t, err.Error(), "error querying role permissions")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// AssignPermissionToRole tests
// ============================================================================

func TestRoleRepository_AssignPermissionToRole_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT OR IGNORE INTO role_permissions").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.AssignPermissionToRole(context.Background(), 1, 2)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_AssignPermissionToRole_Duplicate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	// INSERT OR IGNORE should succeed even if duplicate
	mock.ExpectExec("INSERT OR IGNORE INTO role_permissions").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.AssignPermissionToRole(context.Background(), 1, 2)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_AssignPermissionToRole_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectExec("INSERT OR IGNORE INTO role_permissions").
		WithArgs(1, 2).
		WillReturnError(errors.New("database error"))

	err := repo.AssignPermissionToRole(context.Background(), 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error assigning permission to role")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// RemovePermissionFromRole tests
// ============================================================================

func TestRoleRepository_RemovePermissionFromRole_Success(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM role_permissions WHERE role_id = \\? AND permission_id = \\?").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.RemovePermissionFromRole(context.Background(), 1, 2)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_RemovePermissionFromRole_NotAssigned(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM role_permissions WHERE role_id = \\? AND permission_id = \\?").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.RemovePermissionFromRole(context.Background(), 1, 2)

	assert.NoError(t, err) // Not an error if nothing to remove
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_RemovePermissionFromRole_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectExec("DELETE FROM role_permissions WHERE role_id = \\? AND permission_id = \\?").
		WithArgs(1, 2).
		WillReturnError(errors.New("database error"))

	err := repo.RemovePermissionFromRole(context.Background(), 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error removing permission from role")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ============================================================================
// Edge case tests
// ============================================================================

func TestRoleRepository_GetPermissionsForUser_ZeroUserId(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.name FROM permissions p").
		WithArgs(0).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	perms, err := repo.GetPermissionsForUser(context.Background(), 0)

	assert.NoError(t, err)
	assert.Empty(t, perms)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleRepository_GetPermissionsForRole_ZeroRoleId(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRoleRepository(handles(db), slog.Default())

	mock.ExpectQuery("SELECT p.id, p.name, p.description FROM permissions p JOIN role_permissions rp").
		WithArgs(0).
		WillReturnRows(sqlmock.NewRows(permissionColumns))

	perms, err := repo.GetPermissionsForRole(context.Background(), 0)

	assert.NoError(t, err)
	assert.Empty(t, perms)
	assert.NoError(t, mock.ExpectationsWereMet())
}
