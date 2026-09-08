package api

import (
	"context"
	db "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/notifications"
	"example/sensorHub/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"time"
)

// setupTestRouter creates a test router registering a single GET handler.
// Used by alert and other handler tests that pass gin.HandlerFunc directly.
func setupTestRouter(route string, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiGroup := router.Group("/api")
	apiGroup.GET(route, handler)
	return router
}

type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Login(ctx context.Context, username, password, ip, userAgent string) (string, string, bool, error) {
	args := m.Called(ctx, username, password, ip, userAgent)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

func (m *MockAuthService) ValidateSession(ctx context.Context, rawToken string) (*gen.User, error) {
	args := m.Called(ctx, rawToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.User), args.Error(1)
}

func (m *MockAuthService) Logout(ctx context.Context, rawToken string) error {
	args := m.Called(ctx, rawToken)
	return args.Error(0)
}

func (m *MockAuthService) ChangePassword(ctx context.Context, userId int, newPassword string) error {
	args := m.Called(ctx, userId, newPassword)
	return args.Error(0)
}

func (m *MockAuthService) CreateInitialAdminIfNone(ctx context.Context, username, password string) error {
	args := m.Called(ctx, username, password)
	return args.Error(0)
}

func (m *MockAuthService) ListSessionsForUser(ctx context.Context, userId int) ([]db.SessionInfo, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]db.SessionInfo), args.Error(1)
}

func (m *MockAuthService) RevokeSessionById(ctx context.Context, sessionId int64) error {
	args := m.Called(ctx, sessionId)
	return args.Error(0)
}

func (m *MockAuthService) RevokeSessionByIdWithActor(ctx context.Context, sessionId int64, revokedByUserId *int, reason *string) error {
	args := m.Called(ctx, sessionId, revokedByUserId, reason)
	return args.Error(0)
}

func (m *MockAuthService) GetCSRFForToken(ctx context.Context, rawToken string) (string, error) {
	args := m.Called(ctx, rawToken)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) GetSessionIdForToken(ctx context.Context, rawToken string) (int64, error) {
	args := m.Called(ctx, rawToken)
	return args.Get(0).(int64), args.Error(1)
}

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) CreateUser(ctx context.Context, user gen.User, password string) (int, error) {
	args := m.Called(ctx, user, password)
	return args.Int(0), args.Error(1)
}

func (m *MockUserService) ListUsers(ctx context.Context) ([]gen.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.User), args.Error(1)
}

func (m *MockUserService) GetUserById(ctx context.Context, id int) (*gen.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.User), args.Error(1)
}

func (m *MockUserService) ChangePassword(ctx context.Context, userId int, newPassword string, keepToken string) error {
	args := m.Called(ctx, userId, newPassword, keepToken)
	return args.Error(0)
}

func (m *MockUserService) DeleteUser(ctx context.Context, userId int) error {
	args := m.Called(ctx, userId)
	return args.Error(0)
}

func (m *MockUserService) SetMustChangeFlag(ctx context.Context, userId int, mustChange bool) error {
	args := m.Called(ctx, userId, mustChange)
	return args.Error(0)
}

func (m *MockUserService) SetUserRoles(ctx context.Context, userId int, roles []string) error {
	args := m.Called(ctx, userId, roles)
	return args.Error(0)
}

type MockRoleService struct {
	mock.Mock
}

func (m *MockRoleService) ListRoles(ctx context.Context) ([]db.RoleInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.RoleInfo), args.Error(1)
}

func (m *MockRoleService) ListPermissions(ctx context.Context) ([]db.PermissionInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.PermissionInfo), args.Error(1)
}

func (m *MockRoleService) ListPermissionsForRole(ctx context.Context, roleId int) ([]db.PermissionInfo, error) {
	args := m.Called(ctx, roleId)
	return args.Get(0).([]db.PermissionInfo), args.Error(1)
}

func (m *MockRoleService) AssignPermission(ctx context.Context, roleId int, permissionId int) error {
	args := m.Called(ctx, roleId, permissionId)
	return args.Error(0)
}

func (m *MockRoleService) RemovePermission(ctx context.Context, roleId int, permissionId int) error {
	args := m.Called(ctx, roleId, permissionId)
	return args.Error(0)
}

type MockSensorService struct {
	mock.Mock
}

func (m *MockSensorService) ServiceAddSensor(ctx context.Context, sensor gen.Sensor) error {
	args := m.Called(ctx, sensor)
	return args.Error(0)
}

func (m *MockSensorService) ServiceUpdateSensorById(ctx context.Context, sensor gen.Sensor, retentionHoursPresent bool) error {
	args := m.Called(ctx, sensor, retentionHoursPresent)
	return args.Error(0)
}

func (m *MockSensorService) ServiceDeleteSensorByName(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockSensorService) ServiceGetSensorByName(ctx context.Context, name string) (*gen.Sensor, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

func (m *MockSensorService) ServiceGetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

func (m *MockSensorService) ServiceGetSensorCapabilities(ctx context.Context, id int) ([]gen.Capability, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.Capability), args.Error(1)
}

func (m *MockSensorService) ServiceGetAllSensors(ctx context.Context) ([]gen.Sensor, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorService) ServiceGetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error) {
	args := m.Called(ctx, sensorDriver)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorService) ServiceGetSensorIdByName(ctx context.Context, name string) (int, error) {
	args := m.Called(ctx, name)
	return args.Int(0), args.Error(1)
}

func (m *MockSensorService) ServiceSensorExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockSensorService) ServiceCollectAndStoreAllSensorReadings(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSensorService) ServiceCollectFromSensorByName(ctx context.Context, sensorName string) error {
	args := m.Called(ctx, sensorName)
	return args.Error(0)
}

func (m *MockSensorService) ServiceCollectReadingToValidateSensor(ctx context.Context, sensor gen.Sensor) error {
	args := m.Called(ctx, sensor)
	return args.Error(0)
}

func (m *MockSensorService) ServiceStartPeriodicSensorCollection(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockSensorService) ServiceDiscoverSensors(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSensorService) ServiceValidateSensorConfig(ctx context.Context, sensor gen.Sensor) error {
	args := m.Called(ctx, sensor)
	return args.Error(0)
}

func (m *MockSensorService) ServiceUpdateSensorHealthById(ctx context.Context, sensorId int, healthStatus gen.SensorHealthStatus, healthReason string) {
	m.Called(ctx, sensorId, healthStatus, healthReason)
}

func (m *MockSensorService) ServiceSetEnabledSensorByName(ctx context.Context, name string, enabled bool) error {
	args := m.Called(ctx, name, enabled)
	return args.Error(0)
}

func (m *MockSensorService) ServiceGetSensorHealthHistoryByName(ctx context.Context, name string) ([]gen.SensorHealthHistory, error) {
	args := m.Called(ctx, name)
	return args.Get(0).([]gen.SensorHealthHistory), args.Error(1)
}

func (m *MockSensorService) ServiceGetTotalReadingsForEachSensor() gen.TotalReadingsSample {
	args := m.Called()
	return args.Get(0).(gen.TotalReadingsSample)
}

func (m *MockSensorService) ServiceGetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorService) ServiceApproveSensor(ctx context.Context, sensorId int) error {
	return m.Called(ctx, sensorId).Error(0)
}

func (m *MockSensorService) ServiceDismissSensor(ctx context.Context, sensorId int) error {
	return m.Called(ctx, sensorId).Error(0)
}
func (m *MockSensorService) ServiceProcessPushReadings(ctx context.Context, sensor gen.Sensor, readings []gen.Reading) error {
	return m.Called(ctx, sensor, readings).Error(0)
}
func (m *MockSensorService) ServiceGetMeasurementTypesForSensor(ctx context.Context, sensorId int) ([]gen.MeasurementType, error) {
	args := m.Called(ctx, sensorId)
	return args.Get(0).([]gen.MeasurementType), args.Error(1)
}
func (m *MockSensorService) ServiceGetAllMeasurementTypes(ctx context.Context) ([]gen.MeasurementType, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MeasurementType), args.Error(1)
}
func (m *MockSensorService) ServiceGetAllMeasurementTypesWithReadings(ctx context.Context) ([]gen.MeasurementType, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.MeasurementType), args.Error(1)
}
func (m *MockSensorService) ServiceGetSensorByExternalId(ctx context.Context, externalId string) (*gen.Sensor, error) {
	args := m.Called(ctx, externalId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}
func (m *MockSensorService) ServiceSensorExistsByExternalId(ctx context.Context, externalId string) (bool, error) {
	args := m.Called(ctx, externalId)
	return args.Bool(0), args.Error(1)
}

type MockCommandService struct {
	mock.Mock
}

func (m *MockCommandService) GetHistory(ctx context.Context, sensorID int) ([]gen.CommandHistoryEntry, error) {
	args := m.Called(ctx, sensorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.CommandHistoryEntry), args.Error(1)
}

func (m *MockCommandService) Send(ctx context.Context, sensorID int, actor *gen.User, property string, value string) (service.SentCommandResult, error) {
	args := m.Called(ctx, sensorID, actor, property, value)
	if args.Get(0) == nil {
		return service.SentCommandResult{}, args.Error(1)
	}
	return args.Get(0).(service.SentCommandResult), args.Error(1)
}

type MockPropertiesService struct {
	mock.Mock
}

func (m *MockPropertiesService) ServiceUpdateProperties(ctx context.Context, properties map[string]string) error {
	args := m.Called(ctx, properties)
	return args.Error(0)
}

func (m *MockPropertiesService) ServiceGetProperties(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// ============================================================================
// MockNotificationService
// ============================================================================

type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) CreateNotification(ctx context.Context, notif notifications.Notification, targetPermission string) (int, error) {
	args := m.Called(ctx, notif, targetPermission)
	return args.Int(0), args.Error(1)
}

func (m *MockNotificationService) GetNotificationsForUser(ctx context.Context, userID int, limit, offset int, includeDismissed bool) ([]notifications.UserNotification, error) {
	args := m.Called(ctx, userID, limit, offset, includeDismissed)
	return args.Get(0).([]notifications.UserNotification), args.Error(1)
}

func (m *MockNotificationService) GetUnreadCount(ctx context.Context, userID int) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockNotificationService) MarkAsRead(ctx context.Context, userID, notificationID int) error {
	args := m.Called(ctx, userID, notificationID)
	return args.Error(0)
}

func (m *MockNotificationService) Dismiss(ctx context.Context, userID, notificationID int) error {
	args := m.Called(ctx, userID, notificationID)
	return args.Error(0)
}

func (m *MockNotificationService) BulkMarkAsRead(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockNotificationService) BulkDismiss(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockNotificationService) GetChannelPreferences(ctx context.Context, userID int) ([]notifications.ChannelPreference, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]notifications.ChannelPreference), args.Error(1)
}

func (m *MockNotificationService) SetChannelPreference(ctx context.Context, userID int, pref notifications.ChannelPreference) error {
	args := m.Called(ctx, userID, pref)
	return args.Error(0)
}

func (m *MockNotificationService) ShouldNotifyChannel(ctx context.Context, userID int, category notifications.NotificationCategory, channel string) (bool, error) {
	args := m.Called(ctx, userID, category, channel)
	return args.Bool(0), args.Error(1)
}

// ============================================================================
// MockApiKeyService
// ============================================================================

type MockApiKeyService struct {
	mock.Mock
}

func (m *MockApiKeyService) CreateApiKey(ctx context.Context, name string, userId int, expiresAt *time.Time) (string, error) {
	args := m.Called(ctx, name, userId, expiresAt)
	return args.String(0), args.Error(1)
}

func (m *MockApiKeyService) ListApiKeysForUser(ctx context.Context, userId int) ([]db.ApiKey, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]db.ApiKey), args.Error(1)
}

func (m *MockApiKeyService) UpdateApiKeyExpiry(ctx context.Context, keyId int, userId int, expiresAt *time.Time) error {
	args := m.Called(ctx, keyId, userId, expiresAt)
	return args.Error(0)
}

func (m *MockApiKeyService) RevokeApiKey(ctx context.Context, keyId int, userId int) error {
	args := m.Called(ctx, keyId, userId)
	return args.Error(0)
}

func (m *MockApiKeyService) DeleteApiKey(ctx context.Context, keyId int, userId int) error {
	args := m.Called(ctx, keyId, userId)
	return args.Error(0)
}

func (m *MockApiKeyService) ValidateApiKey(ctx context.Context, rawKey string) (*gen.User, error) {
	args := m.Called(ctx, rawKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.User), args.Error(1)
}

// ============================================================================
// MockOAuthService
// ============================================================================

type MockOAuthService struct {
	mock.Mock
}

func (m *MockOAuthService) GetStatus(ctx context.Context) map[string]interface{} {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{})
}

func (m *MockOAuthService) GetAuthURL(ctx context.Context, state string) (string, error) {
	args := m.Called(ctx, state)
	return args.String(0), args.Error(1)
}

func (m *MockOAuthService) ExchangeCode(ctx context.Context, code string) error {
	args := m.Called(ctx, code)
	return args.Error(0)
}

func (m *MockOAuthService) IsReady(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *MockOAuthService) Reload(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
