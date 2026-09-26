package main

import (
	"context"
	"fmt"

	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"example/sensorHub/testharness/fixtures"
)

const (
	dashboardColumns = 12
	viewerUsername   = "viewer"
)

type widget struct {
	kind          string
	width, height int
	sensor        string
	sensors       []string
	measurement   string
	timeRange     string
	min, max      float64
	scaleMin      float64
	scaleMax      float64
	content       string
	property      string
}

type seededDashboard struct {
	name      string
	isDefault bool
	widgets   []widget
}

const homeNote = "## Dev stack\n\n" +
	"Seeded by `devstack/seed`. **Climate** is shared with viewer, and **Devices** has the switchable office-plug."

var homeDashboard = seededDashboard{
	name:      "Home",
	isDefault: true,
	widgets: []widget{
		{kind: "current-reading", width: 3, height: 3, sensor: "living-room-sensor", measurement: "temperature"},
		{kind: "current-reading", width: 3, height: 3, sensor: "kitchen-sensor", measurement: "humidity"},
		{kind: "gauge", width: 3, height: 3, sensor: "bedroom-sensor", measurement: "temperature", min: 10, max: 30},
		{kind: "gauge", width: 3, height: 3, sensor: "bedroom-sensor", measurement: "humidity", max: 100},
		{kind: "alert-summary", width: 6, height: 5},
		{kind: "notifications-feed", width: 6, height: 5},
		{kind: "weather-forecast", width: 8, height: 4},
		{kind: "markdown-note", width: 4, height: 4, content: homeNote},
	},
}

var climateDashboard = seededDashboard{
	name: "Climate",
	widgets: []widget{
		{kind: "readings-chart", width: 12, height: 4, measurement: "temperature", timeRange: "24h"},
		{kind: "comparison-chart", width: 12, height: 4, measurement: "humidity", timeRange: "7d",
			sensors: []string{"living-room-sensor", "bedroom-sensor", "kitchen-sensor"}},
		{kind: "heatmap", width: 4, height: 4, sensor: "living-room-sensor", measurement: "temperature", scaleMin: 16, scaleMax: 28},
		{kind: "min-max-avg", width: 4, height: 4, sensor: "kitchen-sensor", measurement: "temperature", timeRange: "7d"},
		{kind: "group-summary", width: 4, height: 4, measurement: "temperature"},
	},
}

var devicesDashboard = seededDashboard{
	name: "Devices",
	widgets: []widget{
		{kind: "sensor-toggle", width: 4, height: 3, sensor: "office-plug", property: "state"},
		{kind: "current-reading", width: 4, height: 3, sensor: "fridge-plug", measurement: "power"},
		{kind: "uptime", width: 4, height: 3, sensor: "front-door"},
		{kind: "sensor-detail", width: 6, height: 4, sensor: "office-plug"},
		{kind: "health-timeline", width: 6, height: 4, sensor: "front-door"},
		{kind: "sensor-health-pie", width: 4, height: 4},
		{kind: "sensor-type-pie", width: 4, height: 4},
		{kind: "reading-stats", width: 4, height: 4},
		{kind: "live-readings", width: 12, height: 6},
	},
}

var seededDashboards = []seededDashboard{homeDashboard, climateDashboard, devicesDashboard}

var viewerDashboard = climateDashboard

var viewerGrants = []string{"view_sensors", "view_readings"}

func (w widget) config(sensorIDs map[string]int) (map[string]any, error) {
	config := map[string]any{}
	if w.sensor != "" {
		id, ok := sensorIDs[w.sensor]
		if !ok {
			return nil, fmt.Errorf("no seeded sensor %s", w.sensor)
		}
		config["sensorId"] = id
	}
	if len(w.sensors) > 0 {
		ids := make([]int, 0, len(w.sensors))
		for _, name := range w.sensors {
			id, ok := sensorIDs[name]
			if !ok {
				return nil, fmt.Errorf("no seeded sensor %s", name)
			}
			ids = append(ids, id)
		}
		config["sensorIds"] = ids
	}
	for key, value := range map[string]string{
		"measurementType": w.measurement,
		"timeRange":       w.timeRange,
		"content":         w.content,
		"property":        w.property,
	} {
		if value != "" {
			config[key] = value
		}
	}
	for key, value := range map[string]float64{"min": w.min, "max": w.max, "scaleMin": w.scaleMin, "scaleMax": w.scaleMax} {
		if value != 0 {
			config[key] = value
		}
	}
	return config, nil
}

func (d seededDashboard) layout(sensorIDs map[string]int) ([]gen.DashboardWidget, error) {
	widgets := make([]gen.DashboardWidget, 0, len(d.widgets))
	x, y, rowHeight := 0, 0, 0
	for i, w := range d.widgets {
		config, err := w.config(sensorIDs)
		if err != nil {
			return nil, fmt.Errorf("%s widget on %s: %w", w.kind, d.name, err)
		}
		if x+w.width > dashboardColumns {
			x, y, rowHeight = 0, y+rowHeight, 0
		}
		placed := gen.DashboardWidget{Id: fmt.Sprintf("%s-%d", w.kind, i+1), Type: w.kind, Config: config}
		placed.Layout.X, placed.Layout.Y, placed.Layout.W, placed.Layout.H = x, y, w.width, w.height
		widgets = append(widgets, placed)
		x += w.width
		rowHeight = max(rowHeight, w.height)
	}
	return widgets, nil
}

func (s *seeder) createDashboards(ctx context.Context, userIDs, sensorIDs map[string]int) error {
	adminID := userIDs[adminUsername]
	existing, err := s.ownedDashboards(ctx, adminID)
	if err != nil {
		return err
	}
	for _, seeded := range seededDashboards {
		if dashboard, ok := existing[seeded.name]; ok {
			if err := s.finishDashboard(ctx, adminID, dashboard, seeded); err != nil {
				return err
			}
			continue
		}
		widgets, err := seeded.layout(sensorIDs)
		if err != nil {
			return err
		}
		id, err := fixtures.CreateDashboard(ctx, s.dashboards, adminID, fixtures.Dashboard{
			Name: seeded.name, Widgets: widgets, Default: seeded.isDefault,
		})
		if err != nil {
			return err
		}
		existing[seeded.name] = gen.Dashboard{Id: id, Name: seeded.name}
		s.logger.Info("created dashboard", "name", seeded.name, "widgets", len(widgets), "default", seeded.isDefault)
	}
	return s.shareWithViewer(ctx, adminID, userIDs[viewerUsername], existing[viewerDashboard.name].Id)
}

func (s *seeder) finishDashboard(ctx context.Context, ownerID int, dashboard gen.Dashboard, seeded seededDashboard) error {
	if seeded.isDefault && !dashboard.IsDefault {
		if err := s.dashboards.ServiceSetDefaultDashboard(ctx, ownerID, dashboard.Id); err != nil {
			return fmt.Errorf("failed to make dashboard %s the default: %w", seeded.name, err)
		}
	}
	s.logger.Info("dashboard exists, made sure of its default state", "name", seeded.name)
	return nil
}

func (s *seeder) shareWithViewer(ctx context.Context, adminID, viewerID, dashboardID int) error {
	received, err := s.ownedDashboards(ctx, viewerID)
	if err != nil {
		return err
	}
	if len(received) > 0 {
		s.logger.Info("viewer already has a dashboard, not sharing again", "username", viewerUsername)
		return nil
	}
	if err := s.dashboards.ServiceShareDashboard(ctx, adminID, dashboardID, viewerID); err != nil {
		return fmt.Errorf("failed to share dashboard %s with %s: %w", viewerDashboard.name, viewerUsername, err)
	}
	s.logger.Info("shared dashboard", "name", viewerDashboard.name, "with", viewerUsername)
	return nil
}

func (s *seeder) ownedDashboards(ctx context.Context, ownerID int) (map[string]gen.Dashboard, error) {
	dashboards, err := s.dashboards.ServiceListDashboards(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	owned := make(map[string]gen.Dashboard, len(dashboards))
	for _, dashboard := range dashboards {
		if dashboard.UserId == ownerID {
			owned[dashboard.Name] = dashboard
		}
	}
	return owned, nil
}

func (s *seeder) grantViewerReadAccess(ctx context.Context) error {
	if err := fixtures.GrantPermissions(ctx, s.roles, service.RoleViewer, viewerGrants); err != nil {
		return err
	}
	s.logger.Info("granted the viewer role read access for its dashboard", "permissions", viewerGrants)
	return nil
}
