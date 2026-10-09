package appProps

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"example/sensorHub/telemetry"
)

type ApplicationConfiguration struct {
	SensorCollectionInterval       int `prop:"sensor.collection.interval" default:"300" file:"application" validate:"positive" label:"Collection interval" desc:"How often every enabled sensor is polled." group:"sensors" unit:"seconds" apply:"next-cycle"`
	HealthHistoryRetentionDays     int `prop:"health.history.retention.days" default:"30" file:"application" validate:"non_negative" label:"Health history retention" desc:"How long sensor health check history is kept." group:"retention" unit:"days"`
	SensorDataRetentionDays        int `prop:"sensor.data.retention.days" default:"90" file:"application" validate:"non_negative" label:"Sensor data retention" desc:"How long sensor readings are kept." group:"retention" unit:"days"`
	FailedLoginRetentionDays       int `prop:"failed.login.retention.days" default:"2" file:"application" validate:"non_negative" label:"Failed login retention" desc:"How long failed login attempts are kept." group:"retention" unit:"days"`
	AlertHistoryRetentionDays      int `prop:"alert.history.retention.days" default:"90" file:"application" validate:"non_negative" label:"Alert history retention" desc:"How long alert trigger history is kept." group:"retention" unit:"days"`
	AutomationHistoryRetentionDays int `prop:"automation.history.retention.days" default:"30" file:"application" validate:"non_negative" label:"Automation run history retention" desc:"How long finished automation runs and their step outcomes are kept. Running and waiting runs are always kept." group:"retention" unit:"days"`
	CommandHistoryRetentionDays    int `prop:"command.history.retention.days" default:"90" file:"application" validate:"non_negative" label:"Command history retention" desc:"How long the history of commands sent to devices is kept, whether a person or an automation sent them." group:"retention" unit:"days"`
	DataCleanupIntervalHours       int `prop:"data.cleanup.interval.hours" default:"1" file:"application" validate:"positive" label:"Cleanup interval" desc:"How often the retention cleanup task runs." group:"retention" unit:"hours" apply:"next-cycle"`

	DatabasePath              string `prop:"database.path" default:"data/sensor_hub.db" file:"database" validate:"non_empty" label:"Database file" desc:"SQLite database file, set at install time. Not changeable at runtime." group:"advanced" readonly:"true"`
	DatabaseReaderConnections int    `prop:"database.reader.connections" default:"4" file:"database" validate:"positive" label:"Reader connections" desc:"Connections in the read-only database pool; reads run in parallel up to this many." group:"advanced" apply:"action:service-restart"`

	AuthBcryptCost                int    `prop:"auth.bcrypt.cost" default:"12" file:"application" validate:"min:10,max:31" label:"Bcrypt cost" desc:"Work factor for password hashing, from 10 to 31; higher is slower and stronger." group:"security"`
	AuthSessionTTLMinutes         int    `prop:"auth.session.ttl.minutes" default:"43200" file:"application" label:"Session TTL" desc:"How long a login session stays valid." group:"security" unit:"minutes"`
	AuthSessionCookieName         string `prop:"auth.session.cookie.name" default:"sensor_hub_session" file:"application" label:"Session cookie name" desc:"Name of the browser cookie that carries the session." group:"security"`
	AuthLoginBackoffWindowMinutes int    `prop:"auth.login.backoff.window.minutes" default:"15" file:"application" label:"Login backoff window" desc:"Window over which failed logins are counted towards backoff." group:"security" unit:"minutes"`
	AuthLoginBackoffThreshold     int    `prop:"auth.login.backoff.threshold" default:"5" file:"application" label:"Login backoff threshold" desc:"Failed logins allowed in the window before backoff starts." group:"security"`
	AuthLoginBackoffBaseSeconds   int    `prop:"auth.login.backoff.base.seconds" default:"2" file:"application" label:"Login backoff base delay" desc:"Initial delay applied once login backoff starts." group:"security" unit:"seconds"`
	AuthLoginBackoffMaxSeconds    int    `prop:"auth.login.backoff.max.seconds" default:"300" file:"application" label:"Login backoff max delay" desc:"Upper limit on the login backoff delay." group:"security" unit:"seconds"`

	WeatherLatitude     string `prop:"weather.latitude" default:"53.383" file:"application" label:"Latitude" desc:"Latitude the weather forecast is fetched for." group:"weather"`
	WeatherLongitude    string `prop:"weather.longitude" default:"-1.4659" file:"application" label:"Longitude" desc:"Longitude the weather forecast is fetched for." group:"weather"`
	WeatherLocationName string `prop:"weather.location.name" default:"Sheffield" file:"application" label:"Location name" desc:"Display name for the forecast location." group:"weather"`

	HTTPListenAddress    string `prop:"http.listen.address" default:"127.0.0.1:8080" file:"application" validate:"listen_address" label:"HTTP listen address" desc:"Host and port the HTTP API and web UI listen on. The default takes connections from this machine only, such as from nginx." group:"advanced" apply:"action:service-restart"`
	HTTPTrustedProxies   string `prop:"http.trusted.proxies" default:"127.0.0.1,::1" file:"application" validate:"ip_list" label:"Trusted proxies" desc:"Comma-separated IPs or CIDRs of reverse proxies whose X-Forwarded-For and X-Real-IP headers are believed. The default trusts this machine only, such as nginx running on it. Empty trusts none, so the client address is the connecting peer." group:"security" apply:"action:service-restart"`
	MetricsListenAddress string `prop:"metrics.listen.address" default:"127.0.0.1:9464" file:"application" validate:"listen_address_or_empty" label:"Metrics listen address" desc:"Host and port the Prometheus /metrics endpoint listens on, apart from the API. Empty turns the endpoint off." group:"advanced" apply:"action:service-restart"`

	LogLevel string `prop:"log.level" default:"info" file:"application" desc:"Minimum severity written to the log." group:"advanced" enum:"debug,info,warn,error"`

	MQTTBrokerEnabled          bool   `prop:"mqtt.broker.enabled" default:"true" file:"application" label:"Broker enabled" desc:"Whether the embedded MQTT broker is started." group:"mqtt" apply:"action:service-restart"`
	MQTTBrokerPort             int    `prop:"mqtt.broker.port" default:"1883" file:"application" validate:"positive" label:"Broker port" desc:"TCP port the embedded MQTT broker listens on." group:"mqtt" apply:"action:service-restart"`
	MQTTBrokerListenAddress    string `prop:"mqtt.broker.listen.address" default:"127.0.0.1" file:"application" validate:"listen_host" label:"Broker listen address" desc:"Host or IP address the embedded MQTT broker listens on, without the port. The default takes connections from this machine only, such as through a tunnel or nginx; 0.0.0.0 takes them from the network." group:"mqtt" apply:"action:service-restart"`
	MQTTBrokerConnectRateLimit int    `prop:"mqtt.broker.connect.rate.limit" default:"20" file:"application" validate:"non_negative" label:"Broker CONNECT rate limit" desc:"Most CONNECTs the embedded MQTT broker accepts in one second, across all clients. The rest are refused before their credentials are checked. 0 turns the limit off." group:"mqtt" unit:"per second" apply:"action:service-restart"`

	HubTimezone                  string `prop:"hub.timezone" default:"" file:"application" validate:"timezone" label:"Hub timezone" desc:"IANA zone name, such as Europe/London, that automation schedules run in. Defaults to the server's zone." group:"automations"`
	AutomationMissedGraceMinutes int    `prop:"automation.missed.grace.minutes" default:"10" file:"application" validate:"non_negative" label:"Missed trigger grace window" desc:"How late a trigger that came due while the hub was down can be and still run on startup. Later ones are recorded as missed." group:"automations" unit:"minutes" apply:"action:service-restart"`
	AutomationLoopMaxChain       int    `prop:"automation.loop.max.chain" default:"5" file:"application" validate:"positive" label:"Loop guard chain limit" desc:"The longest chain of automation runs that can start, where a reading acknowledging each run's command started the next. The run that would make it longer is refused and recorded as failed." group:"automations"`

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

func ConvertConfigurationToMaps(cfg *ApplicationConfiguration) (map[string]string, map[string]string) {
	return ConvertToMaps(cfg)
}

func LoadConfigurationFromMaps(appProps, dbProps map[string]string) (*ApplicationConfiguration, error) {
	cfg, err := LoadFromMaps(appProps, dbProps)
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

	dbProps, err := ReadDatabasePropertiesFile()
	if err != nil {
		return err
	}

	return ReloadConfig(appProps, dbProps)
}

// ReloadConfig replaces the global AppConfig from the supplied raw property
// maps, returning an error and leaving the config untouched when the maps do
// not parse. Keys no property is registered for, such as the oauth.* keys a
// 1.5.x install saved, are ignored, and the next save drops them.
func ReloadConfig(appProps, dbProps map[string]string) error {
	cfg, err := LoadConfigurationFromMaps(appProps, dbProps)
	if err != nil {
		slog.Error("failed to reload configuration", "error", err)
		return err
	}

	SetAppConfig(cfg)

	telemetry.SetLogLevel(cfg.LogLevel)

	LogConfig(cfg)

	reloadListenersMu.Lock()
	listeners := make([]func(*ApplicationConfiguration), 0, len(reloadListeners))
	for _, listener := range reloadListeners {
		listeners = append(listeners, listener)
	}
	reloadListenersMu.Unlock()
	for _, listener := range listeners {
		listener(cfg)
	}

	return nil
}

var (
	reloadListenersMu  sync.Mutex
	reloadListeners    = make(map[int]func(*ApplicationConfiguration))
	nextReloadListener int
)

// OnReload listeners run after every successful reload, from a PATCH or from
// an edit to the files on disk.
func OnReload(listener func(cfg *ApplicationConfiguration)) (remove func()) {
	reloadListenersMu.Lock()
	defer reloadListenersMu.Unlock()
	id := nextReloadListener
	nextReloadListener++
	reloadListeners[id] = listener
	return func() {
		reloadListenersMu.Lock()
		defer reloadListenersMu.Unlock()
		delete(reloadListeners, id)
	}
}
