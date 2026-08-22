package appProps

import (
	"log/slog"
	"path/filepath"
	"sync/atomic"

	"example/sensorHub/telemetry"
)

type ApplicationConfiguration struct {
	SensorCollectionInterval   int    `prop:"sensor.collection.interval" default:"300" file:"application" validate:"positive" label:"Collection interval" desc:"How often every enabled sensor is polled." group:"sensors" unit:"seconds" apply:"next-cycle"`
	SensorDiscoverySkip        bool   `prop:"sensor.discovery.skip" default:"true" file:"application" label:"Skip sensor discovery" desc:"Skip automatic sensor discovery at startup." group:"sensors"`
	OpenAPILocation            string `prop:"openapi.yaml.location" default:"./docker_tests/openapi.yaml" file:"application" label:"OpenAPI YAML location" desc:"Path to the OpenAPI document used when discovering HTTP sensors." group:"advanced"`
	HealthHistoryRetentionDays int    `prop:"health.history.retention.days" default:"30" file:"application" validate:"non_negative" label:"Health history retention" desc:"How long sensor health check history is kept." group:"retention" unit:"days"`
	SensorDataRetentionDays    int    `prop:"sensor.data.retention.days" default:"90" file:"application" validate:"non_negative" label:"Sensor data retention" desc:"How long sensor readings are kept." group:"retention" unit:"days"`
	FailedLoginRetentionDays   int    `prop:"failed.login.retention.days" default:"2" file:"application" validate:"non_negative" label:"Failed login retention" desc:"How long failed login attempts are kept." group:"retention" unit:"days"`
	AlertHistoryRetentionDays  int    `prop:"alert.history.retention.days" default:"90" file:"application" validate:"non_negative" label:"Alert history retention" desc:"How long alert trigger history is kept." group:"retention" unit:"days"`
	DataCleanupIntervalHours   int    `prop:"data.cleanup.interval.hours" default:"1" file:"application" validate:"positive" label:"Cleanup interval" desc:"How often the retention cleanup task runs." group:"retention" unit:"hours" apply:"next-cycle"`

	SMTPUser string `prop:"smtp.user" default:"" file:"smtp" desc:"Email address alert and notification emails are sent from." group:"email"`

	DatabasePath string `prop:"database.path" default:"data/sensor_hub.db" file:"database" validate:"non_empty" label:"Database file" desc:"SQLite database file, set at install time. Not changeable at runtime." group:"advanced" readonly:"true"`

	AuthBcryptCost                int    `prop:"auth.bcrypt.cost" default:"12" file:"application" label:"Bcrypt cost" desc:"Work factor for password hashing; higher is slower and stronger." group:"security"`
	AuthSessionTTLMinutes         int    `prop:"auth.session.ttl.minutes" default:"43200" file:"application" label:"Session TTL" desc:"How long a login session stays valid." group:"security" unit:"minutes"`
	AuthSessionCookieName         string `prop:"auth.session.cookie.name" default:"sensor_hub_session" file:"application" label:"Session cookie name" desc:"Name of the browser cookie that carries the session." group:"security"`
	AuthLoginBackoffWindowMinutes int    `prop:"auth.login.backoff.window.minutes" default:"15" file:"application" label:"Login backoff window" desc:"Window over which failed logins are counted towards backoff." group:"security" unit:"minutes"`
	AuthLoginBackoffThreshold     int    `prop:"auth.login.backoff.threshold" default:"5" file:"application" label:"Login backoff threshold" desc:"Failed logins allowed in the window before backoff starts." group:"security"`
	AuthLoginBackoffBaseSeconds   int    `prop:"auth.login.backoff.base.seconds" default:"2" file:"application" label:"Login backoff base delay" desc:"Initial delay applied once login backoff starts." group:"security" unit:"seconds"`
	AuthLoginBackoffMaxSeconds    int    `prop:"auth.login.backoff.max.seconds" default:"300" file:"application" label:"Login backoff max delay" desc:"Upper limit on the login backoff delay." group:"security" unit:"seconds"`

	OAuthCredentialsFilePath         string `prop:"oauth.credentials.file.path" default:"credentials.json" file:"application" label:"OAuth credentials file" desc:"OAuth client credentials file used for sending mail." group:"email" apply:"action:oauth-reload"`
	OAuthTokenFilePath               string `prop:"oauth.token.file.path" default:"token.json" file:"application" label:"OAuth token file" desc:"File where the OAuth token is stored." group:"email" apply:"action:oauth-reload"`
	OAuthTokenRefreshIntervalMinutes int    `prop:"oauth.token.refresh.interval.minutes" default:"30" file:"application" label:"Token refresh interval" desc:"How often the OAuth token is refreshed in the background." group:"email" unit:"minutes" apply:"action:service-restart"`

	WeatherLatitude     string `prop:"weather.latitude" default:"53.383" file:"application" label:"Latitude" desc:"Latitude the weather forecast is fetched for." group:"weather"`
	WeatherLongitude    string `prop:"weather.longitude" default:"-1.4659" file:"application" label:"Longitude" desc:"Longitude the weather forecast is fetched for." group:"weather"`
	WeatherLocationName string `prop:"weather.location.name" default:"Sheffield" file:"application" label:"Location name" desc:"Display name for the forecast location." group:"weather"`

	LogLevel string `prop:"log.level" default:"info" file:"application" desc:"Minimum severity written to the log." group:"advanced" enum:"debug,info,warn,error"`

	MQTTBrokerEnabled bool `prop:"mqtt.broker.enabled" default:"true" file:"application" label:"Broker enabled" desc:"Whether the embedded MQTT broker is started." group:"mqtt" apply:"action:service-restart"`
	MQTTBrokerPort    int  `prop:"mqtt.broker.port" default:"1883" file:"application" validate:"positive" label:"Broker port" desc:"TCP port the embedded MQTT broker listens on." group:"mqtt" apply:"action:service-restart"`

	ActuatorCommandTimeoutSeconds int `prop:"actuator.command.timeout_seconds" default:"10" file:"application" validate:"positive" label:"Actuator command timeout" desc:"How long to wait for a device to acknowledge a command." group:"advanced" unit:"seconds"`

	ReadingsAggregationEnabled bool   `prop:"readings.aggregation.enabled" default:"true" file:"application" label:"Readings aggregation" desc:"Whether readings are downsampled into aggregation tiers." group:"advanced" apply:"action:service-restart"`
	ReadingsAggregationTiers   string `prop:"readings.aggregation.tiers" default:"PT15M:raw,PT1H:PT10S,PT6H:PT1M,P1D:PT5M,P7D:PT15M,P30D:PT1H" file:"application" label:"Aggregation tiers" desc:"Aggregation tier definitions as window:resolution pairs." group:"advanced" apply:"action:service-restart"`
}

var appConfigPtr atomic.Pointer[ApplicationConfiguration]

// AppConfig returns the current configuration snapshot. Nil until
// InitialiseConfig or SetAppConfig has run. Callers that read several fields
// should hold the returned pointer so all reads come from one snapshot.
func AppConfig() *ApplicationConfiguration {
	return appConfigPtr.Load()
}

// SetAppConfig atomically replaces the configuration. Production code goes
// through ReloadConfig; tests set snapshots directly.
func SetAppConfig(cfg *ApplicationConfiguration) {
	appConfigPtr.Store(cfg)
}

func ConvertConfigurationToMaps(cfg *ApplicationConfiguration) (map[string]string, map[string]string, map[string]string) {
	return ConvertToMaps(cfg)
}

func LoadConfigurationFromMaps(appProps, smtpProps, dbProps map[string]string) (*ApplicationConfiguration, error) {
	cfg, err := LoadFromMaps(appProps, smtpProps, dbProps)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func InitialiseConfig(dir string) error {
	setConfigPaths(dir)

	appProps, err := ReadApplicationPropertiesFile()
	if err != nil {
		return err
	}

	smtpProps, err := ReadSMTPPropertiesFile()
	if err != nil {
		return err
	}

	dbProps, err := ReadDatabasePropertiesFile()
	if err != nil {
		return err
	}

	return ReloadConfig(appProps, smtpProps, dbProps)
}

// ReloadConfig replaces the global AppConfig from the supplied raw property
// maps, returning an error and leaving the config untouched when the maps do
// not parse. Relative OAuth file paths are stored as-is on the returned
// struct; callers obtain a config-dir-resolved absolute path via
// [ApplicationConfiguration.ResolvedOAuthCredentialsPath] /
// [ApplicationConfiguration.ResolvedOAuthTokenPath]. Resolving on demand
// (rather than mutating the struct on load) keeps reloads idempotent — see
// issue #44.
func ReloadConfig(appProps, smtpProps, dbProps map[string]string) error {
	cfg, err := LoadConfigurationFromMaps(appProps, smtpProps, dbProps)
	if err != nil {
		slog.Error("failed to reload configuration", "error", err)
		return err
	}

	SetAppConfig(cfg)

	telemetry.SetLogLevel(cfg.LogLevel)

	LogConfig(cfg)

	return nil
}

// ResolvedOAuthCredentialsPath returns the OAuth credentials file path
// resolved against the configuration directory when the stored value is
// relative. Absolute and empty values pass through unchanged.
func (cfg *ApplicationConfiguration) ResolvedOAuthCredentialsPath() string {
	return resolveAgainstConfigDir(cfg.OAuthCredentialsFilePath)
}

// ResolvedOAuthTokenPath returns the OAuth token file path resolved against
// the configuration directory when the stored value is relative. Absolute and
// empty values pass through unchanged.
func (cfg *ApplicationConfiguration) ResolvedOAuthTokenPath() string {
	return resolveAgainstConfigDir(cfg.OAuthTokenFilePath)
}

func resolveAgainstConfigDir(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(configDir, p)
}
