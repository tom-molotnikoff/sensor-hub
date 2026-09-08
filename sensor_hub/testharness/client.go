//go:build integration

package testharness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	gen "example/sensorHub/gen"

	"github.com/gorilla/websocket"
)

// Client is the integration-test HTTP client for the sensor-hub API. It is a
// thin wrapper around the oapi-codegen generated `gen.Client` that owns the
// session cookie jar and the CSRF token issued at login. Every helper method
// here delegates to a typed `gen.Client` operation so the tests exercise the
// same wire contract used by the production CLI and UI.
type Client struct {
	t         *testing.T // nil when used from TestMain
	baseURL   string
	http      *http.Client
	gen       *gen.Client
	csrfToken string
}

// NewClient creates an unauthenticated client pointed at the test server.
func NewClient(t *testing.T, baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{Jar: jar}

	c := &Client{
		t:       t,
		baseURL: baseURL,
		http:    httpClient,
	}

	g, err := gen.NewClient(
		strings.TrimRight(baseURL, "/")+"/api",
		gen.WithHTTPClient(httpClient),
		gen.WithRequestEditorFn(c.injectCSRF),
	)
	if err != nil {
		c.fatalf("failed to build generated client: %v", err)
	}
	c.gen = g
	return c
}

func (c *Client) injectCSRF(_ context.Context, req *http.Request) error {
	if c.csrfToken == "" {
		return nil
	}
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		return nil
	}
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	return nil
}

func (c *Client) fatalf(format string, args ...any) {
	if c.t != nil {
		c.t.Helper()
		c.t.Fatalf(format, args...)
	} else {
		panic(fmt.Sprintf(format, args...))
	}
}

// consume reads the body and returns it together with the HTTP status code.
// Network/transport errors are treated as fatal because every integration test
// would be invalid in the face of a connection failure.
func (c *Client) consume(resp *http.Response, err error) (json.RawMessage, int) {
	if err != nil {
		c.fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		c.fatalf("failed to read response body: %v", readErr)
	}
	return json.RawMessage(body), resp.StatusCode
}

// statusOnly is used for endpoints whose body is irrelevant to tests.
func (c *Client) statusOnly(resp *http.Response, err error) int {
	_, status := c.consume(resp, err)
	return status
}

// decodeInto reads + decodes a successful response body into v. On non-2xx,
// v is left untouched and the status is returned.
func (c *Client) decodeInto(resp *http.Response, err error, v any) int {
	body, status := c.consume(resp, err)
	if status >= 200 && status < 300 && len(body) > 0 {
		if decErr := json.Unmarshal(body, v); decErr != nil {
			c.fatalf("failed to decode response: %v\nbody: %s", decErr, string(body))
		}
	}
	return status
}

func (c *Client) ctx() context.Context { return context.Background() }

func (c *Client) GetRaw(path string, headers http.Header) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(c.ctx(), http.MethodGet, strings.TrimRight(c.baseURL, "/")+path, nil)
	if err != nil {
		return nil, nil, err
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if err := c.injectCSRF(c.ctx(), req); err != nil {
		return nil, nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
}

func (c *Client) DialWebSocket(path string) (*websocket.Conn, *http.Response, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, nil, err
	}

	scheme := "ws"
	if base.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := &url.URL{
		Scheme: scheme,
		Host:   base.Host,
		Path:   path,
	}

	headers := http.Header{}
	for _, cookie := range c.http.Jar.Cookies(base) {
		headers.Add("Cookie", cookie.String())
	}

	return websocket.DefaultDialer.Dial(wsURL.String(), headers)
}

// --- Auth ---

// Login authenticates and stores the session cookie + CSRF token.
func (c *Client) Login(username, password string) int {
	body, status := c.consume(c.gen.Login(c.ctx(), gen.LoginJSONRequestBody{
		Username: username,
		Password: password,
	}))
	if status == http.StatusOK {
		var loginResp struct {
			CSRFToken string `json:"csrf_token"`
		}
		if err := json.Unmarshal(body, &loginResp); err == nil {
			c.csrfToken = loginResp.CSRFToken
		}
	}
	return status
}

// LoginAdmin logs in with the default admin credentials.
func (c *Client) LoginAdmin(env *Env) {
	if status := c.Login(env.AdminUser, env.AdminPass); status != http.StatusOK {
		c.fatalf("admin login failed with status %d", status)
	}
}

func (c *Client) GetMe() (json.RawMessage, int) {
	return c.consume(c.gen.GetCurrentUser(c.ctx()))
}

func (c *Client) Logout() int {
	return c.statusOnly(c.gen.Logout(c.ctx()))
}

// ChangePassword changes the current user's password.
func (c *Client) ChangePassword(newPassword string) int {
	return c.statusOnly(c.gen.ChangePassword(c.ctx(), gen.ChangePasswordJSONRequestBody{
		NewPassword: newPassword,
	}))
}

// --- Sensors ---

func (c *Client) AddSensor(sensor gen.Sensor) (json.RawMessage, int) {
	return c.consume(c.gen.AddSensor(c.ctx(), sensor))
}

func (c *Client) GetAllSensors() ([]gen.Sensor, int) {
	var result []gen.Sensor
	resp, err := c.gen.GetAllSensors(c.ctx())
	status := c.decodeInto(resp, err, &result)
	return result, status
}

func (c *Client) GetSensorByName(name string) (gen.Sensor, int) {
	var result gen.Sensor
	resp, err := c.gen.GetSensorByName(c.ctx(), name)
	status := c.decodeInto(resp, err, &result)
	return result, status
}

func (c *Client) DeleteSensor(name string) int {
	return c.statusOnly(c.gen.DeleteSensorByName(c.ctx(), name))
}

// UpdateSensorRetentionHours sets or clears the per-sensor retention override.
// retentionHours nil marshals as JSON null, which clears the override.
func (c *Client) UpdateSensorRetentionHours(id int, retentionHours *int) int {
	body := strings.NewReader(mustMarshal(map[string]any{"retention_hours": retentionHours}))
	return c.statusOnly(c.gen.UpdateSensorByIdWithBody(c.ctx(), id, "application/json", body))
}

func (c *Client) EnableSensor(name string) int {
	return c.statusOnly(c.gen.EnableSensor(c.ctx(), name))
}

func (c *Client) DisableSensor(name string) int {
	return c.statusOnly(c.gen.DisableSensor(c.ctx(), name))
}

func (c *Client) CollectAll() (json.RawMessage, int) {
	return c.consume(c.gen.CollectAllSensorReadings(c.ctx()))
}

func (c *Client) CollectByName(name string) (json.RawMessage, int) {
	return c.consume(c.gen.CollectFromSensor(c.ctx(), name))
}

// --- Readings ---

func (c *Client) GetReadingsBetween(from, to, sensor string) ([]gen.Reading, int) {
	resp, status := c.GetReadingsBetweenAggregated(from, to, sensor, "", "", "")
	return resp.Readings, status
}

func (c *Client) GetReadingsBetweenAggregated(from, to, sensor, measurementType, aggregation, aggFunction string) (gen.AggregatedReadingsResponse, int) {
	params := &gen.GetReadingsBetweenDatesParams{
		Start: from,
		End:   to,
	}
	if sensor != "" {
		params.Sensor = &sensor
	}
	if measurementType != "" {
		params.Type = &measurementType
	}
	if aggregation != "" {
		agg := gen.GetReadingsBetweenDatesParamsAggregation(aggregation)
		params.Aggregation = &agg
	}
	if aggFunction != "" {
		fn := gen.GetReadingsBetweenDatesParamsAggregationFunction(aggFunction)
		params.AggregationFunction = &fn
	}

	var result gen.AggregatedReadingsResponse
	resp, err := c.gen.GetReadingsBetweenDates(c.ctx(), params)
	status := c.decodeInto(resp, err, &result)
	return result, status
}

// --- Measurement Types ---

func (c *Client) GetAllMeasurementTypes() (json.RawMessage, int) {
	return c.consume(c.gen.GetAllMeasurementTypes(c.ctx(), &gen.GetAllMeasurementTypesParams{}))
}

func (c *Client) GetMeasurementTypesWithReadings() (json.RawMessage, int) {
	hasReadings := true
	return c.consume(c.gen.GetAllMeasurementTypes(c.ctx(), &gen.GetAllMeasurementTypesParams{
		HasReadings: &hasReadings,
	}))
}

func (c *Client) GetMeasurementTypesForSensor(sensorID int) (json.RawMessage, int) {
	return c.consume(c.gen.GetSensorMeasurementTypes(c.ctx(), sensorID))
}

func (c *Client) GetSensorCapabilities(sensorID int) ([]gen.Capability, int) {
	var result []gen.Capability
	resp, err := c.gen.GetSensorCapabilities(c.ctx(), sensorID)
	status := c.decodeInto(resp, err, &result)
	return result, status
}

func (c *Client) GetSensorCommandHistory(sensorID int) ([]gen.CommandHistoryEntry, int) {
	var result []gen.CommandHistoryEntry
	resp, err := c.gen.GetSensorCommandHistory(c.ctx(), sensorID)
	status := c.decodeInto(resp, err, &result)
	return result, status
}

func (c *Client) SendSensorCommand(sensorID int, property, value string) (gen.SensorCommandAccepted, int) {
	var result gen.SensorCommandAccepted
	resp, err := c.gen.SendSensorCommand(c.ctx(), sensorID, gen.SendSensorCommandJSONRequestBody{
		Property: property,
		Value:    value,
	})
	status := c.decodeInto(resp, err, &result)
	return result, status
}

// --- Alerts ---

func (c *Client) CreateAlertRule(rule gen.AlertRule) (json.RawMessage, int) {
	return c.consume(c.gen.CreateAlertRule(c.ctx(), rule))
}

// UpdateAlertRuleWithBody allows sending an arbitrary JSON body to PUT /alerts/{id},
// e.g. to test that the server accepts a body containing only mutable fields.
func (c *Client) UpdateAlertRuleWithBody(id int, body any) (json.RawMessage, int) {
	r := strings.NewReader(mustMarshal(body))
	return c.consume(c.gen.UpdateAlertRuleWithBody(c.ctx(), id, "application/json", r))
}

func (c *Client) GetAlertRulesBySensorID(sensorID int) (json.RawMessage, int) {
	return c.consume(c.gen.GetAlertRulesBySensorId(c.ctx(), sensorID))
}

func (c *Client) GetAlertHistory(sensorID int) (json.RawMessage, int) {
	return c.consume(c.gen.GetAlertHistory(c.ctx(), sensorID, &gen.GetAlertHistoryParams{}))
}

// --- Notifications ---

func (c *Client) SetChannelPreference(pref gen.ChannelPreference) int {
	return c.statusOnly(c.gen.SetChannelPreference(c.ctx(), gen.SetChannelPreferenceJSONRequestBody(pref)))
}

// --- Users ---

func (c *Client) CreateUser(user gen.CreateUserRequest) (json.RawMessage, int) {
	return c.consume(c.gen.CreateUser(c.ctx(), user))
}

func (c *Client) ListUsers() (json.RawMessage, int) {
	return c.consume(c.gen.ListUsers(c.ctx()))
}

func (c *Client) DeleteUser(id int) int {
	return c.statusOnly(c.gen.DeleteUser(c.ctx(), id))
}

// --- Notifications ---

func (c *Client) GetNotifications(limit, offset int) (json.RawMessage, int) {
	return c.consume(c.gen.ListNotifications(c.ctx(), &gen.ListNotificationsParams{
		Limit:  &limit,
		Offset: &offset,
	}))
}

func (c *Client) GetUnreadCount() (json.RawMessage, int) {
	return c.consume(c.gen.GetUnreadCount(c.ctx()))
}

func (c *Client) BulkMarkAsRead() int {
	return c.statusOnly(c.gen.BulkMarkAsRead(c.ctx()))
}

func (c *Client) BulkDismiss() int {
	return c.statusOnly(c.gen.BulkDismiss(c.ctx()))
}

// --- Properties ---

func (c *Client) GetProperties() (json.RawMessage, int) {
	return c.consume(c.gen.GetProperties(c.ctx()))
}

func (c *Client) SetProperty(key, value string) int {
	return c.statusOnly(c.gen.UpdateProperties(c.ctx(), gen.UpdatePropertiesJSONRequestBody{key: value}))
}

func (c *Client) UpdateProperties(props gen.UpdatePropertiesJSONRequestBody) (json.RawMessage, int) {
	return c.consume(c.gen.UpdateProperties(c.ctx(), props))
}

func (c *Client) GetPropertyDefinitions() (gen.PropertyDefinitionsResponse, int) {
	var out gen.PropertyDefinitionsResponse
	resp, err := c.gen.GetPropertyDefinitions(c.ctx())
	status := c.decodeInto(resp, err, &out)
	return out, status
}

// --- API Keys ---

func (c *Client) CreateApiKey(name string) (json.RawMessage, int) {
	return c.consume(c.gen.CreateApiKey(c.ctx(), gen.CreateApiKeyJSONRequestBody{Name: name}))
}

// --- Roles ---

func (c *Client) ListRoles() (json.RawMessage, int) {
	return c.consume(c.gen.ListRoles(c.ctx()))
}

// --- Dashboards ---

// CreateDashboard creates a dashboard with the given name. The body matches the
// historical wire format used by the previous hand-written client (a partial
// object with only `name`); we use the *WithBody variant because the typed
// `gen.CreateDashboardRequest` requires a non-nil Config.
func (c *Client) CreateDashboard(name string) (json.RawMessage, int) {
	body := strings.NewReader(mustMarshal(map[string]string{"name": name}))
	return c.consume(c.gen.CreateDashboardWithBody(c.ctx(), "application/json", body))
}

func (c *Client) ListDashboards() (json.RawMessage, int) {
	return c.consume(c.gen.ListDashboards(c.ctx()))
}

func (c *Client) GetDashboard(id int) (json.RawMessage, int) {
	return c.consume(c.gen.GetDashboard(c.ctx(), id))
}

func (c *Client) UpdateDashboard(id int, req gen.UpdateDashboardRequest) (json.RawMessage, int) {
	return c.consume(c.gen.UpdateDashboard(c.ctx(), id, req))
}

func (c *Client) DeleteDashboard(id int) int {
	return c.statusOnly(c.gen.DeleteDashboard(c.ctx(), id))
}

func (c *Client) ShareDashboard(id, targetUserId int) (json.RawMessage, int) {
	return c.consume(c.gen.ShareDashboard(c.ctx(), id, gen.ShareDashboardJSONRequestBody{
		TargetUserId: targetUserId,
	}))
}

func (c *Client) SetDefaultDashboard(id int) (json.RawMessage, int) {
	return c.consume(c.gen.SetDefaultDashboard(c.ctx(), id))
}

// --- MQTT Brokers ---

func (c *Client) ListMQTTBrokers() (json.RawMessage, int) {
	return c.consume(c.gen.ListMqttBrokers(c.ctx()))
}

func (c *Client) CreateMQTTBroker(broker gen.MQTTBroker) (json.RawMessage, int) {
	return c.consume(c.gen.CreateMqttBroker(c.ctx(), broker))
}

func (c *Client) GetMQTTBroker(id int) (json.RawMessage, int) {
	return c.consume(c.gen.GetMqttBroker(c.ctx(), id))
}

func (c *Client) UpdateMQTTBroker(id int, broker gen.MQTTBroker) (json.RawMessage, int) {
	return c.consume(c.gen.UpdateMqttBroker(c.ctx(), id, broker))
}

func (c *Client) DeleteMQTTBroker(id int) int {
	return c.statusOnly(c.gen.DeleteMqttBroker(c.ctx(), id))
}

// --- MQTT Subscriptions ---

func (c *Client) ListMQTTSubscriptions() (json.RawMessage, int) {
	return c.consume(c.gen.ListMqttSubscriptions(c.ctx(), &gen.ListMqttSubscriptionsParams{}))
}

func (c *Client) CreateMQTTSubscription(sub gen.MQTTSubscription) (json.RawMessage, int) {
	return c.consume(c.gen.CreateMqttSubscription(c.ctx(), sub))
}

func (c *Client) GetMQTTSubscription(id int) (json.RawMessage, int) {
	return c.consume(c.gen.GetMqttSubscription(c.ctx(), id))
}

func (c *Client) UpdateMQTTSubscription(id int, sub gen.MQTTSubscription) (json.RawMessage, int) {
	return c.consume(c.gen.UpdateMqttSubscription(c.ctx(), id, sub))
}

func (c *Client) DeleteMQTTSubscription(id int) int {
	return c.statusOnly(c.gen.DeleteMqttSubscription(c.ctx(), id))
}

// --- Sensor Status ---

func (c *Client) GetSensorsByStatus(status string) (json.RawMessage, int) {
	return c.consume(c.gen.GetSensorsByStatus(c.ctx(), gen.GetSensorsByStatusParamsStatus(status)))
}

func (c *Client) ApproveSensor(id int) (json.RawMessage, int) {
	return c.consume(c.gen.ApproveSensor(c.ctx(), id))
}

func (c *Client) DismissSensor(id int) (json.RawMessage, int) {
	return c.consume(c.gen.DismissSensor(c.ctx(), id))
}

// --- Health & Drivers (formerly served via raw GetJSON) ---

func (c *Client) GetHealth() (json.RawMessage, int) {
	return c.consume(c.gen.GetHealth(c.ctx()))
}

func (c *Client) ListDrivers() (json.RawMessage, int) {
	return c.consume(c.gen.ListDrivers(c.ctx(), &gen.ListDriversParams{}))
}

// --- helpers ---

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("testharness: failed to marshal request body: %v", err))
	}
	return string(b)
}
