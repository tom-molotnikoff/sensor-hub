package appProps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigurationFromMaps_RejectsAnInvalidListenOrProxySetting(t *testing.T) {
	cases := map[string][]string{
		"http.listen.address":        {"", "8080", "127.0.0.1", "127.0.0.1:http", "127.0.0.1:0", "127.0.0.1:70000"},
		"metrics.listen.address":     {"9464", "localhost:"},
		"http.trusted.proxies":       {"nginx", "127.0.0.1;::1", "10.0.0.0/33"},
		"mqtt.broker.listen.address": {"", "0.0.0.0:1883", "[::1]", "local host", "-lan.example", "a..b"},
	}
	for key, values := range cases {
		for _, value := range values {
			appProps := validAppPropsMap()
			appProps[key] = value

			_, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

			var vErr *ValidationError
			require.ErrorAs(t, err, &vErr, "%s=%q", key, value)
			assert.Equal(t, key, vErr.Key)
		}
	}
}

func TestLoadConfigurationFromMaps_AcceptsListenAndProxySettings(t *testing.T) {
	appProps := validAppPropsMap()
	appProps["http.listen.address"] = "0.0.0.0:8080"
	appProps["metrics.listen.address"] = ""
	appProps["http.trusted.proxies"] = "127.0.0.1, ::1,10.0.0.0/8"
	appProps["mqtt.broker.listen.address"] = "::"

	cfg, err := LoadConfigurationFromMaps(appProps, validSmtpPropsMap(), validDbPropsMap())

	require.NoError(t, err)
	assert.Equal(t, []string{"127.0.0.1", "::1", "10.0.0.0/8"}, cfg.TrustedProxies())
	assert.Equal(t, "[::]:1883", cfg.MQTTBrokerAddress())
}
