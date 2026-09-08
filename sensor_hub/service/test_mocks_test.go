package service

import (
	"context"
	"time"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"

	"github.com/stretchr/testify/mock"
)

// ============================================================================
// MockUserRepository
// ============================================================================

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUserByUsername(ctx context.Context, username string) (*gen.User, string, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*gen.User), args.String(1), args.Error(2)
}

func (m *MockUserRepository) GetUserById(ctx context.Context, id int) (*gen.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.User), args.Error(1)
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user gen.User, passwordHash string) (int, error) {
	args := m.Called(ctx, user, passwordHash)
	return args.Int(0), args.Error(1)
}

func (m *MockUserRepository) ListUsers(ctx context.Context) ([]gen.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.User), args.Error(1)
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, userId int, passwordHash string, mustChange bool) error {
	args := m.Called(ctx, userId, passwordHash, mustChange)
	return args.Error(0)
}

func (m *MockUserRepository) SetDisabled(ctx context.Context, userId int, disabled bool) error {
	args := m.Called(ctx, userId, disabled)
	return args.Error(0)
}

func (m *MockUserRepository) AssignRoleToUser(ctx context.Context, userId int, roleName string) error {
	args := m.Called(ctx, userId, roleName)
	return args.Error(0)
}

func (m *MockUserRepository) GetRolesForUser(ctx context.Context, userId int) ([]string, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserRepository) DeleteSessionsForUser(ctx context.Context, userId int) error {
	args := m.Called(ctx, userId)
	return args.Error(0)
}

func (m *MockUserRepository) DeleteSessionsForUserExcept(ctx context.Context, userId int, keepToken string) error {
	args := m.Called(ctx, userId, keepToken)
	return args.Error(0)
}

func (m *MockUserRepository) DeleteUserById(ctx context.Context, userId int) error {
	args := m.Called(ctx, userId)
	return args.Error(0)
}

func (m *MockUserRepository) SetMustChangeFlag(ctx context.Context, userId int, mustChange bool) error {
	args := m.Called(ctx, userId, mustChange)
	return args.Error(0)
}

func (m *MockUserRepository) SetRolesForUser(ctx context.Context, userId int, roles []string) error {
	args := m.Called(ctx, userId, roles)
	return args.Error(0)
}

// ============================================================================
// MockSessionRepository
// ============================================================================

type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) CreateSession(ctx context.Context, userId int, rawToken string, expiresAt time.Time, ip string, userAgent string) (string, error) {
	args := m.Called(ctx, userId, rawToken, expiresAt, ip, userAgent)
	return args.String(0), args.Error(1)
}

func (m *MockSessionRepository) GetAuthenticatedUserByToken(ctx context.Context, rawToken string) (*gen.User, time.Time, error) {
	args := m.Called(ctx, rawToken)
	var user *gen.User
	if args.Get(0) != nil {
		user = args.Get(0).(*gen.User)
	}
	return user, args.Get(1).(time.Time), args.Error(2)
}

func (m *MockSessionRepository) TouchSession(ctx context.Context, rawToken string) error {
	args := m.Called(ctx, rawToken)
	return args.Error(0)
}

func (m *MockSessionRepository) DeleteSessionByToken(ctx context.Context, rawToken string) error {
	args := m.Called(ctx, rawToken)
	return args.Error(0)
}

func (m *MockSessionRepository) DeleteExpiredSessions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSessionRepository) ListSessionsForUser(ctx context.Context, userId int) ([]database.SessionInfo, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]database.SessionInfo), args.Error(1)
}

func (m *MockSessionRepository) RevokeSessionById(ctx context.Context, sessionId int64) error {
	args := m.Called(ctx, sessionId)
	return args.Error(0)
}

func (m *MockSessionRepository) InsertSessionAudit(ctx context.Context, sessionId int64, revokedByUserId *int, action string, reason *string) error {
	args := m.Called(ctx, sessionId, revokedByUserId, action, reason)
	return args.Error(0)
}

func (m *MockSessionRepository) GetCSRFForToken(ctx context.Context, rawToken string) (string, error) {
	args := m.Called(ctx, rawToken)
	return args.String(0), args.Error(1)
}

func (m *MockSessionRepository) GetSessionIdByToken(ctx context.Context, rawToken string) (int64, error) {
	args := m.Called(ctx, rawToken)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) DeleteSessionsForUser(ctx context.Context, userId int) error {
	args := m.Called(ctx, userId)
	return args.Error(0)
}

// ============================================================================
// MockFailedLoginRepository
// ============================================================================

type MockFailedLoginRepository struct {
	mock.Mock
}

func (m *MockFailedLoginRepository) RecordFailedAttempt(ctx context.Context, username string, userId *int, ip string, reason string) error {
	args := m.Called(ctx, username, userId, ip, reason)
	return args.Error(0)
}

func (m *MockFailedLoginRepository) CountRecentFailedAttemptsByUsername(ctx context.Context, username string, window time.Duration) (int, error) {
	args := m.Called(ctx, username, window)
	return args.Int(0), args.Error(1)
}

func (m *MockFailedLoginRepository) CountRecentFailedAttemptsByIP(ctx context.Context, ip string, window time.Duration) (int, error) {
	args := m.Called(ctx, ip, window)
	return args.Int(0), args.Error(1)
}

func (m *MockFailedLoginRepository) DeleteRecentFailedAttemptsByIP(ctx context.Context, ip string, window time.Duration) error {
	args := m.Called(ctx, ip, window)
	return args.Error(0)
}

func (m *MockFailedLoginRepository) DeleteAttemptsOlderThan(ctx context.Context, threshold time.Time) error {
	args := m.Called(ctx, threshold)
	return args.Error(0)
}

// ============================================================================
// MockRoleRepository
// ============================================================================

type MockRoleRepository struct {
	mock.Mock
}

func (m *MockRoleRepository) GetPermissionsForUser(ctx context.Context, userId int) ([]string, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRoleRepository) GetAllRoles(ctx context.Context) ([]database.RoleInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]database.RoleInfo), args.Error(1)
}

func (m *MockRoleRepository) GetAllPermissions(ctx context.Context) ([]database.PermissionInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]database.PermissionInfo), args.Error(1)
}

func (m *MockRoleRepository) GetPermissionsForRole(ctx context.Context, roleId int) ([]database.PermissionInfo, error) {
	args := m.Called(ctx, roleId)
	return args.Get(0).([]database.PermissionInfo), args.Error(1)
}

func (m *MockRoleRepository) AssignPermissionToRole(ctx context.Context, roleId int, permissionId int) error {
	args := m.Called(ctx, roleId, permissionId)
	return args.Error(0)
}

func (m *MockRoleRepository) RemovePermissionFromRole(ctx context.Context, roleId int, permissionId int) error {
	args := m.Called(ctx, roleId, permissionId)
	return args.Error(0)
}

// ============================================================================
// MockSensorRepository
// ============================================================================

type MockSensorRepository struct {
	mock.Mock
}

func (m *MockSensorRepository) AddSensor(ctx context.Context, sensor gen.Sensor) error {
	args := m.Called(ctx, sensor)
	return args.Error(0)
}

func (m *MockSensorRepository) GetAllSensors(ctx context.Context) ([]gen.Sensor, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) GetSensorByName(ctx context.Context, name string) (*gen.Sensor, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) GetSensorById(ctx context.Context, id int) (*gen.Sensor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) GetSensorsByDriver(ctx context.Context, sensorDriver string) ([]gen.Sensor, error) {
	args := m.Called(ctx, sensorDriver)
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) GetSensorIdByName(ctx context.Context, name string) (int, error) {
	args := m.Called(ctx, name)
	return args.Int(0), args.Error(1)
}

func (m *MockSensorRepository) SensorExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockSensorRepository) UpdateSensorById(ctx context.Context, sensor gen.Sensor, retentionHoursPresent bool) error {
	args := m.Called(ctx, sensor, retentionHoursPresent)
	return args.Error(0)
}

func (m *MockSensorRepository) DeleteSensorByName(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockSensorRepository) UpdateSensorHealthById(ctx context.Context, sensorId int, health gen.SensorHealthStatus, reason string) error {
	args := m.Called(ctx, sensorId, health, reason)
	return args.Error(0)
}

func (m *MockSensorRepository) GetSensorHealthHistoryById(ctx context.Context, sensorId int, since time.Time) ([]gen.SensorHealthHistory, error) {
	args := m.Called(ctx, sensorId, since)
	return args.Get(0).([]gen.SensorHealthHistory), args.Error(1)
}

func (m *MockSensorRepository) SetEnabledSensorByName(ctx context.Context, name string, enabled bool) error {
	args := m.Called(ctx, name, enabled)
	return args.Error(0)
}

func (m *MockSensorRepository) DeleteHealthHistoryOlderThan(ctx context.Context, threshold time.Time) error {
	args := m.Called(ctx, threshold)
	return args.Error(0)
}

func (m *MockSensorRepository) GetSensorsByStatus(ctx context.Context, status string) ([]gen.Sensor, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) UpdateSensorStatus(ctx context.Context, sensorId int, status string) error {
	return m.Called(ctx, sensorId, status).Error(0)
}

func (m *MockSensorRepository) GetSensorsWithRetention(ctx context.Context) ([]gen.Sensor, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) GetSensorByExternalId(ctx context.Context, externalId string) (*gen.Sensor, error) {
	args := m.Called(ctx, externalId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.Sensor), args.Error(1)
}

func (m *MockSensorRepository) SensorExistsByExternalId(ctx context.Context, externalId string) (bool, error) {
	args := m.Called(ctx, externalId)
	return args.Bool(0), args.Error(1)
}

// ============================================================================
// MockReadingsRepository
// ============================================================================

type MockReadingsRepository struct {
	mock.Mock
}

func (m *MockReadingsRepository) Ingest(ctx context.Context, batch database.ReadingBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockReadingsRepository) GetBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, interval database.AggregationInterval, aggFunc database.AggregationFunction) ([]gen.Reading, error) {
	args := m.Called(ctx, startDate, endDate, sensorName, measurementType, interval, aggFunc)
	return args.Get(0).([]gen.Reading), args.Error(1)
}

func (m *MockReadingsRepository) GetLatest(ctx context.Context) ([]gen.Reading, error) {
	args := m.Called(ctx)
	return args.Get(0).([]gen.Reading), args.Error(1)
}

func (m *MockReadingsRepository) CountReadingsPerActiveSensor(ctx context.Context) (map[string]int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockReadingsRepository) DeleteReadingsOlderThan(ctx context.Context, cutoffDate time.Time) error {
	args := m.Called(ctx, cutoffDate)
	return args.Error(0)
}

func (m *MockReadingsRepository) DeleteReadingsOlderThanForSensor(ctx context.Context, cutoffDate time.Time, sensorId int) error {
	args := m.Called(ctx, cutoffDate, sensorId)
	return args.Error(0)
}

func (m *MockReadingsRepository) DeleteReadingsOlderThanExcludingSensors(ctx context.Context, cutoffDate time.Time, excludedSensorIds []int) error {
	args := m.Called(ctx, cutoffDate, excludedSensorIds)
	return args.Error(0)
}

// ============================================================================
// MockApiKeyRepository
// ============================================================================

type MockApiKeyRepository struct {
	mock.Mock
}

func (m *MockApiKeyRepository) CreateApiKey(ctx context.Context, name string, keyPrefix string, keyHash string, userId int, expiresAt *time.Time) (int64, error) {
	args := m.Called(ctx, name, keyPrefix, keyHash, userId, expiresAt)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockApiKeyRepository) GetApiKeyByHash(ctx context.Context, keyHash string) (*database.ApiKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.ApiKey), args.Error(1)
}

func (m *MockApiKeyRepository) ListApiKeysForUser(ctx context.Context, userId int) ([]database.ApiKey, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]database.ApiKey), args.Error(1)
}

func (m *MockApiKeyRepository) UpdateApiKeyExpiry(ctx context.Context, id int, expiresAt *time.Time) error {
	args := m.Called(ctx, id, expiresAt)
	return args.Error(0)
}

func (m *MockApiKeyRepository) RevokeApiKey(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockApiKeyRepository) DeleteApiKey(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockApiKeyRepository) UpdateLastUsed(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockReadingsSampler struct {
	mock.Mock
}

func (m *MockReadingsSampler) Sample(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockReadingsSampler) LatestSample() gen.TotalReadingsSample {
	args := m.Called()
	return args.Get(0).(gen.TotalReadingsSample)
}

// ============================================================================
// MockMaintenanceRepository
// ============================================================================

type MockMaintenanceRepository struct {
	mock.Mock
}

func (m *MockMaintenanceRepository) ReclaimFreePages(ctx context.Context, chunkPages int) (int64, error) {
	args := m.Called(ctx, chunkPages)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMaintenanceRepository) Checkpoint(ctx context.Context) (*database.CheckpointResult, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.CheckpointResult), args.Error(1)
}

func (m *MockMaintenanceRepository) Optimise(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMaintenanceRepository) DatabaseStats(ctx context.Context) (*database.DatabaseStatsResult, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.DatabaseStatsResult), args.Error(1)
}
