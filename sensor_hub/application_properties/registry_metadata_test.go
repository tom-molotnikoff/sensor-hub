package appProps

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func defByKey(t *testing.T, defs []PropertyDef, key string) PropertyDef {
	t.Helper()
	for _, d := range defs {
		if d.Key == key {
			return d
		}
	}
	t.Fatalf("no definition for key %q", key)
	return PropertyDef{}
}

func TestBuildRegistry_ReadsMetadataTags(t *testing.T) {
	type taggedConfig struct {
		Interval int    `prop:"test.interval" default:"300" file:"application" validate:"positive" desc:"How often sensors are polled." group:"sensors" unit:"seconds"`
		LogLevel string `prop:"test.log.level" default:"info" file:"application" desc:"Minimum severity written to the log." group:"advanced" enum:"debug,info,warn,error"`
	}

	defs := buildRegistryFrom(reflect.TypeOf(taggedConfig{}))

	interval := defByKey(t, defs, "test.interval")
	assert.Equal(t, "How often sensors are polled.", interval.Description)
	assert.Equal(t, "sensors", interval.Group)
	assert.Equal(t, "seconds", interval.Unit)
	assert.Empty(t, interval.Enum)

	level := defByKey(t, defs, "test.log.level")
	assert.Equal(t, []string{"debug", "info", "warn", "error"}, level.Enum)
	assert.Empty(t, level.Unit)
}

func TestBuildRegistry_ApplyStates(t *testing.T) {
	type taggedConfig struct {
		Interval   int    `prop:"test.interval" default:"300" file:"application" apply:"next-cycle"`
		BrokerPort int    `prop:"test.broker.port" default:"1883" file:"application" apply:"action:service-restart"`
		Untagged   string `prop:"test.untagged" default:"" file:"application"`
		DBPath     string `prop:"test.db.path" default:"data/db" file:"database" readonly:"true"`
	}

	defs := buildRegistryFrom(reflect.TypeOf(taggedConfig{}))

	assert.Equal(t, ApplyNextCycle, defByKey(t, defs, "test.interval").Apply)
	assert.Equal(t, ApplyState("action:service-restart"), defByKey(t, defs, "test.broker.port").Apply)
	assert.Equal(t, ApplyLive, defByKey(t, defs, "test.untagged").Apply)

	dbPath := defByKey(t, defs, "test.db.path")
	assert.True(t, dbPath.ReadOnly)
	assert.Equal(t, ApplyReadOnly, dbPath.Apply)
}

func TestBuildRegistry_Labels(t *testing.T) {
	type taggedConfig struct {
		SensorCollectionInterval int    `prop:"test.interval" default:"300" file:"application" label:"Collection interval"`
		DataCleanupIntervalHours int    `prop:"test.cleanup" default:"1" file:"application"`
		SMTPUser                 string `prop:"test.smtp.user" default:"" file:"smtp"`
	}

	defs := buildRegistryFrom(reflect.TypeOf(taggedConfig{}))

	assert.Equal(t, "Collection interval", defByKey(t, defs, "test.interval").Label)
	assert.Equal(t, "Data cleanup interval hours", defByKey(t, defs, "test.cleanup").Label)
	assert.Equal(t, "SMTP user", defByKey(t, defs, "test.smtp.user").Label)
}

// TestRegistry_MetadataComplete keeps the metadata honest as properties are
// added. Once the configuration.md table is deleted this test is the only
// enforcement that a new property ships with a description, a group and a
// truthful apply state.
func TestRegistry_MetadataComplete(t *testing.T) {
	defs := Definitions()
	assert.Len(t, defs, 36)

	knownGroups := make(map[string]bool)
	for _, g := range PropertyGroups() {
		assert.NotEmpty(t, g.ID)
		assert.NotEmpty(t, g.Label)
		assert.NotEmpty(t, g.Description)
		knownGroups[g.ID] = true
	}

	applyCounts := make(map[string]int)
	groupCounts := make(map[string]int)

	for _, def := range defs {
		assert.NotEmpty(t, def.Label, "property %q has no label", def.Key)
		assert.NotEmpty(t, def.Description, "property %q has no description", def.Key)
		assert.True(t, knownGroups[def.Group], "property %q has unknown group %q", def.Key, def.Group)
		assert.True(t, def.Apply.Valid(), "property %q has invalid apply state %q", def.Key, def.Apply)

		if len(def.Enum) > 0 {
			assert.Contains(t, def.Enum, def.Default, "property %q default %q is outside its enum", def.Key, def.Default)
		}

		category := string(def.Apply)
		if strings.HasPrefix(category, "action:") {
			category = "action"
		}
		applyCounts[category]++
		groupCounts[def.Group]++
	}

	assert.Equal(t, map[string]int{"live": 21, "next-cycle": 2, "action": 12, "readonly": 1}, applyCounts)
	assert.Equal(t, map[string]int{"sensors": 1, "automations": 3, "retention": 7, "security": 8, "mqtt": 2, "email": 4, "weather": 3, "advanced": 8}, groupCounts)
}

func TestRegistry_SpecAssignments(t *testing.T) {
	defs := Definitions()

	logLevel := defByKey(t, defs, "log.level")
	assert.Equal(t, ApplyLive, logLevel.Apply)
	assert.Equal(t, []string{"debug", "info", "warn", "error"}, logLevel.Enum)

	assert.Equal(t, ApplyNextCycle, defByKey(t, defs, "sensor.collection.interval").Apply)
	assert.Equal(t, ApplyNextCycle, defByKey(t, defs, "data.cleanup.interval.hours").Apply)
	assert.Equal(t, ApplyState("action:service-restart"), defByKey(t, defs, "mqtt.broker.port").Apply)
	assert.Equal(t, ApplyState("action:oauth-reload"), defByKey(t, defs, "oauth.token.file.path").Apply)

	for key, group := range map[string]string{
		"http.listen.address":    "advanced",
		"http.trusted.proxies":   "security",
		"metrics.listen.address": "advanced",
	} {
		def := defByKey(t, defs, key)
		assert.Equal(t, ApplyState("action:service-restart"), def.Apply, key)
		assert.Equal(t, group, def.Group, key)
	}
	assert.Equal(t, "127.0.0.1:8080", defByKey(t, defs, "http.listen.address").Default)
	assert.Equal(t, "", defByKey(t, defs, "http.trusted.proxies").Default)
	assert.Equal(t, "127.0.0.1:9464", defByKey(t, defs, "metrics.listen.address").Default)

	dbPath := defByKey(t, defs, "database.path")
	assert.True(t, dbPath.ReadOnly)
	assert.Equal(t, ApplyReadOnly, dbPath.Apply)
}

func TestPropertyGroups_Ordered(t *testing.T) {
	groups := PropertyGroups()
	assert.Len(t, groups, 8)
	for i, g := range groups {
		assert.Equal(t, i+1, g.Order, "group %q out of order", g.ID)
	}
}

func TestApplyState_Valid(t *testing.T) {
	valid := []ApplyState{"live", "next-cycle", "readonly", "action:service-restart", "action:oauth-reload"}
	for _, s := range valid {
		assert.True(t, s.Valid(), "expected %q to be valid", s)
	}

	invalid := []ApplyState{"", "restart", "action:", "action:unknown", "Live"}
	for _, s := range invalid {
		assert.False(t, s.Valid(), "expected %q to be invalid", s)
	}
}
