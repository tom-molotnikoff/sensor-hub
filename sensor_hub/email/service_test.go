package email

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/testharness/smtpfake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func ptr[T any](v T) *T { return &v }

func newTestService(repo *fakeRepo, store *fakeSecrets, logs *bytes.Buffer) *Service {
	logger := slog.Default()
	if logs != nil {
		logger = slog.New(slog.NewTextHandler(logs, nil))
	}
	return NewService(repo, store, logger)
}

func validSettings() gen.EmailSettings {
	return gen.EmailSettings{Host: "smtp.example.com", Port: 587, Security: SecuritySTARTTLS, Username: "hub", FromAddress: "hub@example.com"}
}

func TestUpdate_RefusesInvalidSettings(t *testing.T) {
	cases := map[string]func(*gen.EmailSettings){
		"empty host":            func(s *gen.EmailSettings) { s.Host = "  " },
		"port 0":                func(s *gen.EmailSettings) { s.Port = 0 },
		"port 65536":            func(s *gen.EmailSettings) { s.Port = 65536 },
		"unknown security":      func(s *gen.EmailSettings) { s.Security = "ssl" },
		"empty from address":    func(s *gen.EmailSettings) { s.FromAddress = "" },
		"not an address":        func(s *gen.EmailSettings) { s.FromAddress = "hub" },
		"with a display name":   func(s *gen.EmailSettings) { s.FromAddress = "Hub <hub@example.com>" },
		"username on two lines": func(s *gen.EmailSettings) { s.Username = "hub\r\nRCPT TO:<x@example.com>" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			settings := validSettings()
			change(&settings)

			_, err := newTestService(repo, newFakeSecrets(), nil).Update(context.Background(), settings)

			assert.ErrorIs(t, err, ErrInvalidSettings)
			assert.Equal(t, database.EmailSettings{}, repo.settings, "nothing was saved")
		})
	}
}

func TestUpdate_SetsKeepsAndClearsThePassword(t *testing.T) {
	ctx := context.Background()
	store := newFakeSecrets()
	service := newTestService(&fakeRepo{}, store, nil)
	withPassword := func(password *string) gen.EmailSettings {
		s := validSettings()
		s.Password = password
		return s
	}

	got, err := service.Update(ctx, withPassword(ptr("first")))
	require.NoError(t, err)
	assert.Equal(t, gen.EmailSettingsPasswordStatus("set"), *got.PasswordStatus)
	assert.Nil(t, got.Password, "the password is never returned")

	for name, password := range map[string]*string{"omitted": nil, "placeholder": ptr("****")} {
		_, err := service.Update(ctx, withPassword(password))
		require.NoError(t, err, name)
		assert.Equal(t, "first", store.values["smtp/password"], "%s keeps the password", name)
	}

	_, err = service.Update(ctx, withPassword(ptr("second")))
	require.NoError(t, err)
	assert.Equal(t, "second", store.values["smtp/password"])

	got, err = service.Update(ctx, withPassword(ptr("")))
	require.NoError(t, err)
	assert.Equal(t, gen.EmailSettingsPasswordStatus("unset"), *got.PasswordStatus)
	assert.NotContains(t, store.values, "smtp/password")
}

func TestSendNotification_SendsNothingAndWarnsOnceWhenNotConfigured(t *testing.T) {
	cases := map[string]func(*fakeRepo, *fakeSecrets){
		"no host":                 func(r *fakeRepo, s *fakeSecrets) { s.values["smtp/password"] = "x" },
		"no password":             func(r *fakeRepo, s *fakeSecrets) { r.settings.Host = "smtp.example.com" },
		"password needs re-entry": func(r *fakeRepo, s *fakeSecrets) { r.settings.Host = "smtp.example.com"; s.needsReentry = true },
	}
	for name, setUp := range cases {
		t.Run(name, func(t *testing.T) {
			repo, store, logs := &fakeRepo{}, newFakeSecrets(), &bytes.Buffer{}
			setUp(repo, store)
			service := newTestService(repo, store, logs)

			for range 3 {
				assert.NoError(t, service.SendNotification("admin@example.com", "Hot", "It is hot", "threshold_alert"))
			}

			assert.Equal(t, 1, strings.Count(logs.String(), "level=WARN"), logs.String())
			assert.Nil(t, repo.settings.LastError, "nothing was tried")
			assert.Nil(t, repo.settings.LastSentAt)
		})
	}
}

// configuredService points a service at a fake SMTP server over plain SMTP.
func configuredService(t *testing.T, password string) (*Service, *fakeRepo, *smtpfake.Server) {
	t.Helper()
	fake := startServer(t, nil, false)
	repo := &fakeRepo{settings: database.EmailSettings{
		Host: fake.Host(), Port: fake.Port(), Security: SecurityNone, Username: testUser, FromAddress: "hub@example.com",
	}}
	store := newFakeSecrets()
	store.values["smtp/password"] = password
	return newTestService(repo, store, nil), repo, fake
}

func TestSendTest_NeedsSettings(t *testing.T) {
	service := newTestService(&fakeRepo{}, newFakeSecrets(), nil)

	assert.ErrorIs(t, service.SendTest(context.Background(), "admin@example.com"), ErrNotConfigured)
}

func TestSends_AreCountedByResult(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	good, _, _ := configuredService(t, testPassword)
	bad, _, _ := configuredService(t, "wrong-password")
	require.NoError(t, good.SendTest(context.Background(), "admin@example.com"))
	require.NoError(t, good.SendNotification("admin@example.com", "t", "m", "threshold_alert"))
	require.Error(t, bad.SendTest(context.Background(), "admin@example.com"))

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &collected))
	counts := map[string]int64{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "sensor_hub.email.send" {
				continue
			}
			for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
				result, _ := point.Attributes.Value("result")
				counts[result.AsString()] = point.Value
			}
		}
	}
	assert.Equal(t, map[string]int64{"sent": 2, "failed": 1}, counts)
}
