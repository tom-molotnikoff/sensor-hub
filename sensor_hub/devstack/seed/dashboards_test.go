package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var widgetRegistry = filepath.Join("..", "..", "ui", "sensor_hub_ui", "src", "dashboard", "widgets", "index.ts")

var registeredWidget = regexp.MustCompile(`registerWidget\(\s*\{\s*type:\s*['"]([^'"]+)['"]`)

func unseededWidgetTypes(registry string, dashboards []seededDashboard) ([]string, error) {
	matches := registeredWidget.FindAllStringSubmatch(registry, -1)
	if len(matches) == 0 {
		return nil, errors.New("found no widget types registered with registerWidget")
	}
	if calls := strings.Count(registry, "registerWidget("); calls != len(matches) {
		return nil, fmt.Errorf("found %d registerWidget calls but read the type of only %d, so type must be the first key", calls, len(matches))
	}
	seeded := map[string]bool{}
	for _, dashboard := range dashboards {
		for _, w := range dashboard.widgets {
			seeded[w.kind] = true
		}
	}
	var missing []string
	for _, match := range matches {
		if !seeded[match[1]] {
			missing = append(missing, match[1])
		}
	}
	return missing, nil
}

func TestSeededDashboards_UseEveryRegisteredWidgetType(t *testing.T) {
	registry, err := os.ReadFile(widgetRegistry)
	require.NoError(t, err)

	missing, err := unseededWidgetTypes(string(registry), seededDashboards)
	require.NoError(t, err, widgetRegistry)
	for _, missing := range missing {
		t.Errorf("widget type %q is registered but on no seeded dashboard", missing)
	}
}

func TestUnseededWidgetTypes_NamesANewTypeAndIgnoresAliases(t *testing.T) {
	registry := `
		registerWidget({ type: 'gauge', label: 'Gauge' });
		registerAlias('dial', 'gauge');
		registerWidget({
			type: "brand-new-widget",
		});`

	missing, err := unseededWidgetTypes(registry, []seededDashboard{{widgets: []widget{{kind: "gauge"}}}})

	require.NoError(t, err)
	assert.Equal(t, []string{"brand-new-widget"}, missing)
}

func TestUnseededWidgetTypes_FailsWhenATypeCannotBeRead(t *testing.T) {
	registry := `
		registerWidget({ type: 'gauge' });
		registerWidget({ label: 'Late type', type: 'late-type' });`

	_, err := unseededWidgetTypes(registry, []seededDashboard{{widgets: []widget{{kind: "gauge"}}}})

	assert.Error(t, err)
}

func TestUnseededWidgetTypes_FailsWhenNoTypeIsRegistered(t *testing.T) {
	_, err := unseededWidgetTypes("registerAlias('dial', 'gauge');", seededDashboards)

	assert.Error(t, err)
}

func dashboardsOf(t *testing.T, db *database.Handles, username string) map[string]gen.Dashboard {
	t.Helper()
	ctx := context.Background()
	user, _, err := database.NewUserRepository(db, discardLogger()).GetUserByUsername(ctx, username)
	require.NoError(t, err)
	require.NotNil(t, user, username)
	s := &seeder{dashboards: service.NewDashboardService(database.NewDashboardRepository(db, discardLogger()), discardLogger())}
	owned, err := s.ownedDashboards(ctx, user.Id)
	require.NoError(t, err)
	return owned
}

func widgetTypesOf(t *testing.T, dashboard gen.Dashboard) []string {
	t.Helper()
	var config gen.DashboardConfig
	require.NoError(t, json.Unmarshal([]byte(dashboard.Config), &config))
	var types []string
	for _, w := range config.Widgets {
		types = append(types, w.Type)
	}
	return types
}

func TestSeed_FirstRunCreatesTheDashboardsWithHomeAsTheDefault(t *testing.T) {
	db := openTempDatabase(t)

	runSeed(t, db)

	owned := dashboardsOf(t, db, adminUsername)
	require.Len(t, owned, len(seededDashboards))
	for _, seeded := range seededDashboards {
		dashboard, ok := owned[seeded.name]
		require.True(t, ok, seeded.name)
		assert.Equal(t, seeded.isDefault, dashboard.IsDefault, seeded.name)
		assert.Len(t, widgetTypesOf(t, dashboard), len(seeded.widgets), seeded.name)
	}
	assert.True(t, owned["Home"].IsDefault)
}

func TestSeed_FirstRunSharesADashboardWithTheViewer(t *testing.T) {
	db := openTempDatabase(t)

	runSeed(t, db)

	received := dashboardsOf(t, db, viewerUsername)
	require.Len(t, received, 1)
	for _, dashboard := range received {
		assert.Equal(t, widgetTypesOf(t, dashboardsOf(t, db, adminUsername)[viewerDashboard.name]), widgetTypesOf(t, dashboard))
	}
}

func TestSeed_FirstRunGrantsTheViewerReadAccess(t *testing.T) {
	db := openTempDatabase(t)

	runSeed(t, db)

	users := database.NewUserRepository(db, discardLogger())
	viewer, _, err := users.GetUserByUsername(context.Background(), viewerUsername)
	require.NoError(t, err)
	permissions, err := database.NewRoleRepository(db, discardLogger()).GetPermissionsForUser(context.Background(), viewer.Id)
	require.NoError(t, err)
	assert.Subset(t, permissions, viewerGrants)
}

func TestSeed_RerunKeepsEditsToSeededDashboards(t *testing.T) {
	db := openTempDatabase(t)
	runSeed(t, db)
	ctx := context.Background()
	dashboards := service.NewDashboardService(database.NewDashboardRepository(db, discardLogger()), discardLogger())
	owned := dashboardsOf(t, db, adminUsername)
	adminID := owned["Home"].UserId
	require.NoError(t, dashboards.ServiceDeleteDashboard(ctx, adminID, owned["Home"].Id))
	renamed := "My climate"
	require.NoError(t, dashboards.ServiceUpdateDashboard(ctx, adminID, owned["Climate"].Id, gen.UpdateDashboardRequest{
		Name: &renamed, Config: &gen.DashboardConfig{Widgets: []gen.DashboardWidget{}},
	}))

	runSeed(t, db)

	after := dashboardsOf(t, db, adminUsername)
	assert.NotContains(t, after, "Home", "a deleted seeded dashboard stays deleted")
	assert.NotContains(t, after, "Climate")
	require.Contains(t, after, renamed, "an edited seeded dashboard keeps the edit")
	assert.Empty(t, widgetTypesOf(t, after[renamed]))
	assert.Len(t, dashboardsOf(t, db, viewerUsername), 1)
}
