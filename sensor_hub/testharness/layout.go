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

var layoutViewerGrants = []string{"view_sensors", "view_readings", "view_alerts", "view_notifications"}

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
	"mqtt":            createLayoutMQTT,
	"alerts":          createLayoutAlerts,
	"notifications":   createLayoutNotifications,
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
	retired := gen.DashboardWidget{Id: "retired", Type: "retired-widget", Config: map[string]interface{}{}}
	retired.Layout.Y, retired.Layout.W, retired.Layout.H = 12, 4, 2
	config.Widgets = []gen.DashboardWidget{retired, readings, uptime, healthPie, typePie, stats, timeline}

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
		retentionHours       *int
	}{
		{"attic-bulb", "mqtt-zigbee2mqtt", "good", true, ptr(120)},
		{"back-door-contact", "mqtt-zigbee2mqtt", "bad", true, ptr(720)},
		{"garage-temp", "mqtt-zigbee2mqtt", "unknown", true, nil},
		{"hallway-motion", "mqtt-zigbee2mqtt", "good", false, nil},
		{"kitchen-plug", "mqtt-zigbee2mqtt", "good", true, ptr(48)},
		{"loft-hygrometer", "sensor-hub-http-temperature", "bad", false, nil},
		{"porch-light", "mqtt-zigbee2mqtt", "good", true, nil},
	}
	for _, sensor := range sensors {
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO sensors (name, sensor_driver, config, health_status, health_reason, enabled, retention_hours) VALUES (?, ?, '{}', ?, 'fixture', ?, ?)",
			sensor.name, sensor.driver, sensor.health, sensor.enabled, sensor.retentionHours); err != nil {
			return fmt.Errorf("failed to insert sensor %s: %w", sensor.name, err)
		}
	}
	return nil
}

func ptr(value int) *int {
	return &value
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

func createLayoutMQTT(ctx context.Context, env *Env) error {
	brokers := []struct {
		name, host string
		enabled    bool
	}{
		{"Garage Mosquitto", "mqtt.garage.lan", true},
		{"Loft Relay", "10.0.0.42", false},
	}
	for _, broker := range brokers {
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO mqtt_brokers (name, type, host, port, enabled) VALUES (?, 'external', ?, 1883, ?)",
			broker.name, broker.host, broker.enabled); err != nil {
			return fmt.Errorf("failed to insert broker %s: %w", broker.name, err)
		}
	}
	topics := []string{
		"zigbee2mqtt/#", "zigbee2mqtt/bridge/devices", "zigbee2mqtt/attic/+", "zigbee2mqtt/garage/+",
		"zigbee2mqtt/kitchen/+", "zigbee2mqtt/hallway/+", "zigbee2mqtt/loft/+", "zigbee2mqtt/porch/+",
		"zigbee2mqtt/office/+", "zigbee2mqtt/bedroom/+", "zigbee2mqtt/garden/+", "zigbee2mqtt/utility/+",
	}
	for index, topic := range topics {
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO mqtt_subscriptions (broker_id, topic_pattern, driver_type, enabled) SELECT id, ?, 'mqtt-zigbee2mqtt', ? FROM mqtt_brokers WHERE name = 'Garage Mosquitto'",
			topic, index%4 != 3); err != nil {
			return fmt.Errorf("failed to insert subscription %s: %w", topic, err)
		}
	}
	return nil
}

func createLayoutAlerts(ctx context.Context, env *Env) error {
	rules := []struct {
		sensor, measurement string
		high, low           float64
		rateLimit           int
		enabled             bool
	}{
		{"seed-sensor-01", "temperature", 28, 12, 3600, true},
		{"seed-sensor-01", "humidity", 70, 30, 1800, true},
		{"seed-sensor-02", "temperature", 30, 10, 3600, true},
		{"seed-sensor-02", "humidity", 65, 35, 900, false},
		{"seed-sensor-03", "temperature", 26, 16, 7200, true},
		{"seed-sensor-03", "humidity", 75, 25, 0, true},
		{"seed-sensor-04", "temperature", 32, 8, 3600, false},
		{"seed-sensor-04", "humidity", 60, 40, 3600, true},
		{"seed-sensor-05", "temperature", 24, 18, 45, true},
		{"seed-sensor-05", "humidity", 80, 20, 3600, true},
		{"seed-sensor-06", "temperature", 29, 11, 86400, true},
		{"seed-sensor-06", "humidity", 68, 32, 3600, false},
	}
	for _, rule := range rules {
		if _, err := env.DB.Writer.ExecContext(ctx,
			`INSERT INTO sensor_alert_rules (sensor_id, measurement_type_id, alert_type, high_threshold, low_threshold, rate_limit_seconds, enabled)
			 SELECT s.id, mt.id, 'numeric_range', ?, ?, ?, ? FROM sensors s, measurement_types mt WHERE s.name = ? AND mt.name = ?`,
			rule.high, rule.low, rule.rateLimit, rule.enabled, rule.sensor, rule.measurement); err != nil {
			return fmt.Errorf("failed to insert alert rule for %s %s: %w", rule.sensor, rule.measurement, err)
		}
	}
	for hours := 1; hours <= 12; hours++ {
		if _, err := env.DB.Writer.ExecContext(ctx,
			`INSERT INTO alert_sent_history (alert_rule_id, sensor_id, sent_at, alert_reason, reading_value)
			 SELECT r.id, r.sensor_id, datetime('now', ?), 'above high threshold', ? FROM sensor_alert_rules r
			 JOIN sensors s ON s.id = r.sensor_id WHERE s.name = 'seed-sensor-01' ORDER BY r.id LIMIT 1`,
			fmt.Sprintf("-%d hours", hours*6), 28+float64(hours)/4); err != nil {
			return fmt.Errorf("failed to insert alert history: %w", err)
		}
	}
	return nil
}

func createLayoutNotifications(ctx context.Context, env *Env) error {
	notifications := []struct{ category, severity, title, message string }{
		{"threshold_alert", "warning", "seed-sensor-01 temperature high", "Temperature reached 29.5°C, above the 28°C threshold."},
		{"threshold_alert", "error", "seed-sensor-03 humidity high", "Humidity reached 81%, above the 75% threshold for more than an hour."},
		{"config_change", "info", "Sensor added", "porch-light was added by testadmin."},
		{"user_management", "info", "User created", "testviewer was created with the viewer role."},
		{"threshold_alert", "warning", "seed-sensor-05 temperature low", "Temperature fell to 17.2°C, below the 18°C threshold."},
		{"config_change", "info", "Retention changed", "kitchen-plug now keeps readings for 48 hours."},
		{"threshold_alert", "error", "seed-sensor-02 temperature high", "Temperature reached 31.1°C, above the 30°C threshold."},
		{"user_management", "warning", "Password reset required", "testviewer must change their password at next sign-in."},
		{"config_change", "info", "Sensor disabled", "hallway-motion was disabled."},
		{"threshold_alert", "warning", "seed-sensor-04 humidity low", "Humidity fell to 38%, below the 40% threshold."},
		{"config_change", "info", "MQTT broker added", "Garage Mosquitto was added at mqtt.garage.lan:1883."},
		{"threshold_alert", "info", "seed-sensor-06 back in range", "Temperature is back between 11°C and 29°C."},
		{"user_management", "info", "Role changed", "testadmin granted manage_alerts to the viewer role."},
		{"config_change", "warning", "Sensor unhealthy", "back-door-contact has not reported for 2 hours."},
	}
	for index, notification := range notifications {
		result, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO notifications (category, severity, title, message, created_at) VALUES (?, ?, ?, ?, datetime('now', ?))",
			notification.category, notification.severity, notification.title, notification.message,
			fmt.Sprintf("-%d hours", index*3))
		if err != nil {
			return fmt.Errorf("failed to insert notification %q: %w", notification.title, err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to read notification id: %w", err)
		}
		if _, err := env.DB.Writer.ExecContext(ctx,
			"INSERT INTO user_notifications (user_id, notification_id, is_read) SELECT id, ?, ? FROM users WHERE username IN (?, ?)",
			id, index >= 6, env.AdminUser, layoutViewerUser); err != nil {
			return fmt.Errorf("failed to assign notification %q: %w", notification.title, err)
		}
	}
	return nil
}
