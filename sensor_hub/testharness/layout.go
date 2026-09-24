//go:build integration

package testharness

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

const (
	layoutViewerUser = "testviewer"
	layoutViewerPass = "viewerpassword123"
)

type LayoutOptions struct {
	SeedPath   string
	UI         fs.FS
	ListenAddr string
	Fixtures   []string
}

type layoutFixture func(ctx context.Context, env *Env) error

var layoutFixtures = map[string]layoutFixture{
	"dashboard":      createLayoutDashboard,
	"health-history": createLayoutHealthHistory,
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

func createLayoutDashboard(ctx context.Context, env *Env) error {
	admin, _, err := database.NewUserRepository(env.DB, slog.Default()).GetUserByUsername(ctx, env.AdminUser)
	if err != nil {
		return fmt.Errorf("failed to look up harness admin: %w", err)
	}

	var config gen.DashboardConfig
	config.Breakpoints.Lg, config.Breakpoints.Md, config.Breakpoints.Sm = 12, 8, 4
	readings := gen.DashboardWidget{Id: "readings-chart", Type: "readings-chart", Config: map[string]interface{}{}}
	readings.Layout.W, readings.Layout.H = 12, 4
	uptime := gen.DashboardWidget{Id: "uptime", Type: "uptime", Config: map[string]interface{}{"sensorId": 1}}
	uptime.Layout.Y, uptime.Layout.W, uptime.Layout.H = 4, 3, 3
	config.Widgets = []gen.DashboardWidget{readings, uptime}

	dashboards := service.NewDashboardService(database.NewDashboardRepository(env.DB, slog.Default()), slog.Default())
	if _, err := dashboards.ServiceCreateDashboard(ctx, admin.Id, gen.CreateDashboardRequest{Name: "Layout", Config: config}); err != nil {
		return fmt.Errorf("failed to create dashboard: %w", err)
	}
	return nil
}

func createLayoutHealthHistory(ctx context.Context, env *Env) error {
	if _, err := env.DB.Writer.ExecContext(ctx,
		"INSERT INTO sensor_health_history (sensor_id, health_status, recorded_at) VALUES (1, 'good', datetime('now', '-29 days'))"); err != nil {
		return fmt.Errorf("failed to insert health history: %w", err)
	}
	return nil
}
