package api

import (
	"example/sensorHub/api/middleware"
	gen "example/sensorHub/gen"

	"github.com/gin-gonic/gin"
)

// routePermissions maps "METHOD /api/full-path" to the required permission name.
// Routes not in this map require only authentication (no specific permission),
// or are fully public (no CookieAuthScopes set by the generated wrapper).
var routePermissions = map[string]string{
	// Alerts
	"GET /api/alerts":                          "view_alerts",
	"POST /api/alerts":                         "manage_alerts",
	"GET /api/alerts/sensor/:sensorId":         "view_alerts",
	"GET /api/alerts/sensor/:sensorId/history": "view_alerts",
	"GET /api/alerts/:id":                      "view_alerts",
	"PUT /api/alerts/:id":                      "manage_alerts",
	"DELETE /api/alerts/:id":                   "manage_alerts",

	// API Keys
	"GET /api/api-keys":              "manage_api_keys",
	"POST /api/api-keys":             "manage_api_keys",
	"PATCH /api/api-keys/:id/expiry": "manage_api_keys",
	"POST /api/api-keys/:id/revoke":  "manage_api_keys",
	"DELETE /api/api-keys/:id":       "manage_api_keys",

	// Dashboards
	"GET /api/dashboards":             "view_dashboards",
	"POST /api/dashboards":            "manage_dashboards",
	"GET /api/dashboards/:id":         "view_dashboards",
	"PUT /api/dashboards/:id":         "manage_dashboards",
	"DELETE /api/dashboards/:id":      "manage_dashboards",
	"POST /api/dashboards/:id/share":  "manage_dashboards",
	"PUT /api/dashboards/:id/default": "manage_dashboards",

	// MQTT Brokers
	"GET /api/mqtt/brokers":        "view_mqtt",
	"POST /api/mqtt/brokers":       "manage_mqtt",
	"GET /api/mqtt/brokers/:id":    "view_mqtt",
	"PUT /api/mqtt/brokers/:id":    "manage_mqtt",
	"DELETE /api/mqtt/brokers/:id": "manage_mqtt",

	// MQTT Subscriptions
	"GET /api/mqtt/subscriptions":        "view_mqtt",
	"POST /api/mqtt/subscriptions":       "manage_mqtt",
	"GET /api/mqtt/subscriptions/:id":    "view_mqtt",
	"PUT /api/mqtt/subscriptions/:id":    "manage_mqtt",
	"DELETE /api/mqtt/subscriptions/:id": "manage_mqtt",

	// MQTT Stats
	"GET /api/mqtt/stats": "view_mqtt",

	// Notifications
	"GET /api/notifications":               "view_notifications",
	"GET /api/notifications/unread-count":  "view_notifications",
	"POST /api/notifications/:id/read":     "view_notifications",
	"POST /api/notifications/:id/dismiss":  "manage_notifications",
	"POST /api/notifications/bulk/read":    "view_notifications",
	"POST /api/notifications/bulk/dismiss": "manage_notifications",
	"GET /api/notifications/preferences":   "view_notifications",
	"POST /api/notifications/preferences":  "manage_notifications",
	"GET /api/notifications/ws":            "view_notifications",

	// OAuth
	"GET /api/oauth/status":       "manage_oauth",
	"GET /api/oauth/authorize":    "manage_oauth",
	"POST /api/oauth/submit-code": "manage_oauth",
	"POST /api/oauth/reload":      "manage_oauth",

	// Properties
	"PATCH /api/properties":           "manage_properties",
	"GET /api/properties":             "view_properties",
	"GET /api/properties/ws":          "view_properties",
	"GET /api/properties/definitions": "view_properties",

	// Readings
	"GET /api/readings/between":    "view_readings",
	"GET /api/readings/ws/current": "view_readings",

	// Roles
	"GET /api/roles":                         "view_roles",
	"GET /api/roles/permissions":             "view_roles",
	"GET /api/roles/:id/permissions":         "view_roles",
	"POST /api/roles/:id/permissions":        "manage_roles",
	"DELETE /api/roles/:id/permissions/:pid": "manage_roles",

	// Sensors
	"GET /api/sensors":                             "view_sensors",
	"POST /api/sensors":                            "manage_sensors",
	"POST /api/sensors/:id/command":                "control_sensors",
	"PUT /api/sensors/:id":                         "manage_sensors",
	"DELETE /api/sensors/:name":                    "delete_sensors",
	"GET /api/sensors/:name":                       "view_sensors",
	"HEAD /api/sensors/:name":                      "view_sensors",
	"GET /api/sensors/driver/:driver":              "view_sensors",
	"POST /api/sensors/collect":                    "trigger_readings",
	"POST /api/sensors/collect/:sensorName":        "trigger_readings",
	"POST /api/sensors/disable/:sensorName":        "manage_sensors",
	"POST /api/sensors/enable/:sensorName":         "manage_sensors",
	"GET /api/sensors/health/:name":                "view_sensors",
	"GET /api/sensors/by-id/:id/capabilities":      "view_sensors",
	"GET /api/sensors/by-id/:id/commands":          "view_sensors",
	"GET /api/sensors/stats/total-readings":        "view_sensors",
	"GET /api/sensors/status/:status":              "view_sensors",
	"POST /api/sensors/approve/:id":                "manage_sensors",
	"POST /api/sensors/dismiss/:id":                "manage_sensors",
	"GET /api/sensors/by-id/:id/measurement-types": "view_sensors",

	// Measurement types
	"GET /api/measurement-types": "view_sensors",

	// Users
	"GET /api/users":                   "view_users",
	"POST /api/users":                  "manage_users",
	"DELETE /api/users/:id":            "manage_users",
	"PATCH /api/users/:id/must_change": "manage_users",
	"POST /api/users/:id/roles":        "manage_users",
}

// RouteAuthAndPermissionMiddleware returns a gen.MiddlewareFunc that:
//  1. Enforces authentication on routes that the generated wrapper marks as
//     requiring auth (by setting gen.CookieAuthScopes in the context).
//  2. Enforces a specific permission for routes listed in routePermissions.
//
// Routes not marked by the generated wrapper (Login, GetHealth, GetOpenApiSpec,
// ListDrivers) pass through without any authentication check.
func RouteAuthAndPermissionMiddleware() gen.MiddlewareFunc {
	return func(c *gin.Context) {
		_, requiresAuth := c.Get(gen.CookieAuthScopes)
		if !requiresAuth {
			return
		}

		middleware.AuthRequired()(c)
		if c.IsAborted() {
			return
		}

		key := c.Request.Method + " " + c.FullPath()
		if permission, ok := routePermissions[key]; ok {
			middleware.RequirePermission(permission)(c)
		}
	}
}
