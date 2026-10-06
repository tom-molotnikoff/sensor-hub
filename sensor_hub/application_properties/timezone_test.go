package appProps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigurationFromMaps_RejectsAZoneThatDoesNotLoad(t *testing.T) {
	for _, zone := range []string{"Europe/Londn", "", "Local"} {
		appProps := validAppPropsMap()
		appProps["hub.timezone"] = zone

		_, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

		var vErr *ValidationError
		require.ErrorAs(t, err, &vErr, "zone %q", zone)
		assert.Equal(t, "hub.timezone", vErr.Key)
	}
}

func TestLoadConfigurationFromMaps_AcceptsAnIANAZone(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["hub.timezone"] = "Europe/London"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	require.NoError(t, err)
	assert.Equal(t, "Europe/London", cfg.HubLocation().String())
}

func TestServerTimezone_UsesTheTZVariable(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	assert.Equal(t, "America/New_York", serverTimezone())
}

func TestZoneFromLocaltimeLink(t *testing.T) {
	assert.Equal(t, "Europe/London", zoneFromLocaltimeLink("../usr/share/zoneinfo/Europe/London"))
	assert.Equal(t, "", zoneFromLocaltimeLink("/etc/localtime.custom"))
}

func TestOnReload_SeesTheNewConfiguration(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["hub.timezone"] = "Asia/Tokyo"

	var seen string
	remove := OnReload(func(cfg *ApplicationConfiguration) { seen = cfg.HubTimezone })
	defer remove()

	require.NoError(t, ReloadConfig(appProps, validSmtpPropsMap(), validDbPropsMap()))
	assert.Equal(t, "Asia/Tokyo", seen)
}
