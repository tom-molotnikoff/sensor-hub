package fixtures

import (
	"context"
	"fmt"

	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

type Dashboard struct {
	Name    string
	Widgets []gen.DashboardWidget
	Default bool
}

func CreateDashboard(ctx context.Context, dashboards service.DashboardServiceInterface, ownerID int, dashboard Dashboard) (int, error) {
	id, err := dashboards.ServiceCreateDashboard(ctx, ownerID, gen.CreateDashboardRequest{
		Name:   dashboard.Name,
		Config: gen.DashboardConfig{Widgets: dashboard.Widgets},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create dashboard %s: %w", dashboard.Name, err)
	}
	if !dashboard.Default {
		return id, nil
	}
	if err := dashboards.ServiceSetDefaultDashboard(ctx, ownerID, id); err != nil {
		return 0, fmt.Errorf("failed to make dashboard %s the default: %w", dashboard.Name, err)
	}
	return id, nil
}
