package fixtures

import (
	"context"
	"fmt"
	"slices"

	database "example/sensorHub/db"
	"example/sensorHub/service"
)

func GrantPermissions(ctx context.Context, roles service.RoleServiceInterface, roleName string, permissions []string) error {
	all, err := roles.ListRoles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}
	roleIndex := slices.IndexFunc(all, func(role database.RoleInfo) bool { return role.Name == roleName })
	if roleIndex < 0 {
		return fmt.Errorf("role %q does not exist", roleName)
	}
	known, err := roles.ListPermissions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list permissions: %w", err)
	}
	granted := 0
	for _, permission := range known {
		if !slices.Contains(permissions, permission.Name) {
			continue
		}
		if err := roles.AssignPermission(ctx, all[roleIndex].Id, permission.Id); err != nil {
			return fmt.Errorf("failed to grant %s to %s: %w", permission.Name, roleName, err)
		}
		granted++
	}
	if granted != len(permissions) {
		return fmt.Errorf("granted %d of the %s permissions %v", granted, roleName, permissions)
	}
	return nil
}
