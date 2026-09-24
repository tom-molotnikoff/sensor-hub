//go:build integration

package testharness

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

const (
	layoutViewerUser = "testviewer"
	layoutViewerPass = "viewerpassword123"
)

var layoutViewerGrants = []string{"view_sensors", "view_readings"}

type LayoutOptions struct {
	SeedPath   string
	UI         fs.FS
	ListenAddr string
	Fixtures   []string
}

type layoutFixture func(ctx context.Context, env *Env) error

var layoutFixtures = map[string]layoutFixture{
	"dashboard":       createLayoutDashboard,
	"health-history":  createLayoutHealthHistory,
	"sensors":         createLayoutSensors,
	"pending-sensors": createLayoutPendingSensors,
}

func StartLayoutServer(ctx context.Context, opts LayoutOptions) (*Env, func(), error) {
	fixtures := make([]layoutFixture, 0, len(opts.Fixtures))
	for _, name := range opts.Fixtures {
		fixture, ok := layoutFixtures[name]
		if !ok {
			return nil, func() {}, fmt.Errorf("unknown layout fixture %q", name)
		}
		fixtures = append(fixtures, fixture)
	}

	env, cleanup, err := startServer(serverOptions{seedPath: opts.SeedPath, ui: opts.UI, listenAddr: opts.ListenAddr})
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}

	if err := createLayoutUsers(ctx, env); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	for i, fixture := range fixtures {
		if err := fixture(ctx, env); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("layout fixture %q failed: %w", opts.Fixtures[i], err)
		}
	}
	return env, cleanup, nil
}

func createLayoutUsers(ctx context.Context, env *Env) error {
	users := database.NewUserRepository(env.DB, slog.Default())

	admin, _, err := users.GetUserByUsername(ctx, env.AdminUser)
	if err != nil {
		return fmt.Errorf("failed to look up harness admin: %w", err)
	}
	if admin == nil {
		return fmt.Errorf("harness admin %q does not exist", env.AdminUser)
	}
	if err := users.SetMustChangeFlag(ctx, admin.Id, false); err != nil {
		return fmt.Errorf("failed to clear harness admin password change: %w", err)
	}

	if err := grantViewerReadAccess(ctx, env); err != nil {
		return err
	}
	viewerID, err := service.NewUserService(users, nil, slog.Default()).CreateUser(ctx,
		gen.User{Username: layoutViewerUser, Roles: []string{service.RoleViewer}}, layoutViewerPass)
	if err != nil {
		return fmt.Errorf("failed to create viewer: %w", err)
	}
	if err := users.SetMustChangeFlag(ctx, viewerID, false); err != nil {
		return fmt.Errorf("failed to clear viewer password change: %w", err)
	}
	return nil
}

func grantViewerReadAccess(ctx context.Context, env *Env) error {
	roles := database.NewRoleRepository(env.DB, slog.Default())
	all, err := roles.GetAllRoles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}
	viewerRole := -1
	for _, role := range all {
		if role.Name == service.RoleViewer {
			viewerRole = role.Id
		}
	}
	if viewerRole < 0 {
		return fmt.Errorf("role %q does not exist", service.RoleViewer)
	}
	permissions, err := roles.GetAllPermissions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list permissions: %w", err)
	}
	granted := 0
	for _, permission := range permissions {
		if !slices.Contains(layoutViewerGrants, permission.Name) {
			continue
		}
		if err := roles.AssignPermissionToRole(ctx, viewerRole, permission.Id); err != nil {
			return fmt.Errorf("failed to grant %s to viewer: %w", permission.Name, err)
		}
		granted++
	}
	if granted != len(layoutViewerGrants) {
		return fmt.Errorf("granted %d of the viewer permissions %v", granted, layoutViewerGrants)
	}
	return nil
}

func createLayoutDashboard(ctx context.Context, env *Env) error {
	admin, _, err := database.NewUserRepository(env.DB, slog.Default()).GetUserByUsername(ctx, env.AdminUser)
	if err != nil {
		return fmt.Errorf("failed to look up harness admin: %w", err)
	}

	var config gen.DashboardConfig
	config.Breakpoints.Lg, config.Breakpoints.Md, config.Breakpoints.Sm = 12, 8, 4
	readings := gen.DashboardWidget{Id: "readings-chart", Type: "readings-chart", Config: map[string]interface{}{"measurementType": "temperature"}}
	readings.Layout.W, readings.Layout.H = 12, 4
	uptime := gen.DashboardWidget{Id: "uptime", Type: "uptime", Config: map[string]interface{}{"sensorId": 1}}
	uptime.Layout.Y, uptime.Layout.W, uptime.Layout.H = 4, 3, 3
	healthPie := gen.DashboardWidget{Id: "sensor-health-pie", Type: "sensor-health-pie", Config: map[string]interface{}{}}
	healthPie.Layout.X, healthPie.Layout.Y, healthPie.Layout.W, healthPie.Layout.H = 3, 4, 4, 4
	typePie := gen.DashboardWidget{Id: "sensor-type-pie", Type: "sensor-type-pie", Config: map[string]interface{}{}}
	typePie.Layout.X, typePie.Layout.Y, typePie.Layout.W, typePie.Layout.H = 7, 4, 5, 3
	timeline := gen.DashboardWidget{Id: "health-timeline", Type: "health-timeline", Config: map[string]interface{}{"sensorId": 1}}
	timeline.Layout.Y, timeline.Layout.W, timeline.Layout.H = 8, 6, 4
	stats := gen.DashboardWidget{Id: "reading-stats", Type: "reading-stats", Config: map[string]interface{}{}}
	stats.Layout.X, stats.Layout.Y, stats.Layout.W, stats.Layout.H = 6, 8, 6, 4
	config.Widgets = []gen.DashboardWidget{readings, uptime, healthPie, typePie, timeline, stats}

	dashboards := service.NewDashboardService(database.NewDashboardRepository(env.DB, slog.Default()), slog.Default())
	if _, err := dashboards.ServiceCreateDashboard(ctx, admin.Id, gen.CreateDashboardRequest{Name: "Layout", Config: config}); err != nil {
		return fmt.Errorf("failed to create dashboard: %w", err)
	}
	return nil
}

func createLayoutHealthHistory(ctx context.Context, env *Env) error {
	type entry struct{ status, at string }
	var entries []entry
	for day := 29; day >= 3; day -= 2 {
		entries = append(entries, entry{"good", fmt.Sprintf("-%d days", day)})
	}
	entries = append(entries, entry{"bad", "-2 days"}, entry{"good", "-47 hours"})
	for _, row := range entries {
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO sensor_health_history (sensor_id, health_status, recorded_at) VALUES (1, ?, datetime('now', ?))",
			row.status, row.at); err != nil {
			return fmt.Errorf("failed to insert health history: %w", err)
		}
	}
	return nil
}

func createLayoutSensors(ctx context.Context, env *Env) error {
	sensors := []struct {
		name, driver, health string
		enabled              bool
	}{
		{"attic-bulb", "mqtt-zigbee2mqtt", "good", true},
		{"back-door-contact", "mqtt-zigbee2mqtt", "bad", true},
		{"garage-temp", "mqtt-zigbee2mqtt", "unknown", true},
		{"hallway-motion", "mqtt-zigbee2mqtt", "good", false},
		{"kitchen-plug", "mqtt-zigbee2mqtt", "good", true},
		{"loft-hygrometer", "sensor-hub-http-temperature", "bad", false},
		{"porch-light", "mqtt-zigbee2mqtt", "good", true},
	}
	for _, sensor := range sensors {
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO sensors (name, sensor_driver, config, health_status, health_reason, enabled) VALUES (?, ?, '{}', ?, 'fixture', ?)",
			sensor.name, sensor.driver, sensor.health, sensor.enabled); err != nil {
			return fmt.Errorf("failed to insert sensor %s: %w", sensor.name, err)
		}
	}
	return nil
}

func createLayoutPendingSensors(ctx context.Context, env *Env) error {
	sensors := []struct{ name, status, metadata string }{
		{"0xa4c1380b2e11ffff", "pending", `{"manufacturer":"Aqara","model":"MCCGQ11LM"}`},
		{"0x00158d0001a2b3c4", "pending", `{}`},
		{"0x54ef441000a1b2c3", "pending", `{"manufacturer":"SONOFF","model":"SNZB-02"}`},
		{"0x842e14fffe9d8a7b", "dismissed", `{}`},
		{"0x00124b0021c4d5e6", "dismissed", `{"model":"TS0201"}`},
	}
	for _, sensor := range sensors {
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO sensors (name, sensor_driver, config, health_status, health_reason, enabled, status, metadata) VALUES (?, 'mqtt-zigbee2mqtt', '{}', 'unknown', 'fixture', 1, ?, ?)",
			sensor.name, sensor.status, sensor.metadata); err != nil {
			return fmt.Errorf("failed to insert %s sensor %s: %w", sensor.status, sensor.name, err)
		}
	}
	return nil
}
