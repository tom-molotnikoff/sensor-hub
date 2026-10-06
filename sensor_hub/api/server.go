package api

import (
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

// Compile-time assertion: Server must implement gen.ServerInterface.
// If this fails, the generated OpenAPI spec and the Go server are out of sync.
var _ gen.ServerInterface = (*Server)(nil)

// Server holds all service dependencies for the API layer.
type Server struct {
	sensorService       service.SensorServiceInterface
	commandService      service.CommandServiceInterface
	readingsService     service.ReadingsServiceInterface
	authService         service.AuthServiceInterface
	userService         service.UserServiceInterface
	roleService         service.RoleServiceInterface
	alertService        service.AlertManagementServiceInterface
	notificationService service.NotificationServiceInterface
	apiKeyService       service.ApiKeyServiceInterface
	dashboardService    service.DashboardServiceInterface
	propertiesService   service.PropertiesServiceInterface
	mqttService         service.MQTTServiceInterface
	oauthService        OAuthAPIServiceInterface
	mqttStatsProvider   MQTTStatsProvider
	automationService   AutomationServiceInterface
}

// NewServer constructs a Server with all service dependencies.
func NewServer(
	sensorService service.SensorServiceInterface,
	commandService service.CommandServiceInterface,
	readingsService service.ReadingsServiceInterface,
	authService service.AuthServiceInterface,
	userService service.UserServiceInterface,
	roleService service.RoleServiceInterface,
	alertService service.AlertManagementServiceInterface,
	notificationService service.NotificationServiceInterface,
	apiKeyService service.ApiKeyServiceInterface,
	dashboardService service.DashboardServiceInterface,
	propertiesService service.PropertiesServiceInterface,
	mqttService service.MQTTServiceInterface,
	oauthService OAuthAPIServiceInterface,
	mqttStatsProvider MQTTStatsProvider,
	automationService AutomationServiceInterface,
) *Server {
	return &Server{
		sensorService:       sensorService,
		commandService:      commandService,
		readingsService:     readingsService,
		authService:         authService,
		userService:         userService,
		roleService:         roleService,
		alertService:        alertService,
		notificationService: notificationService,
		apiKeyService:       apiKeyService,
		dashboardService:    dashboardService,
		propertiesService:   propertiesService,
		mqttService:         mqttService,
		oauthService:        oauthService,
		mqttStatsProvider:   mqttStatsProvider,
		automationService:   automationService,
	}
}
