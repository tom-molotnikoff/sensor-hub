package api

import (
	"context"
	gen "example/sensorHub/gen"
	"example/sensorHub/service"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockReadingsService struct {
	ServiceGetBetweenDatesFunc func(context.Context, string, string, string, string, string, string) (*gen.AggregatedReadingsResponse, error)
	ServiceGetLatestFunc       func(context.Context) ([]gen.Reading, error)
}

func (m *mockReadingsService) ServiceGetBetweenDates(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
	return m.ServiceGetBetweenDatesFunc(ctx, startDate, endDate, sensorName, measurementType, overrideInterval, overrideFunction)
}
func (m *mockReadingsService) ServiceGetLatest(ctx context.Context) ([]gen.Reading, error) {
	return m.ServiceGetLatestFunc(ctx)
}

// setupReadingsBetweenRoute builds a router that pre-constructs params and calls GetReadingsBetweenDates,
// mirroring what readings_routes.go does via its closures.
func setupReadingsBetweenRoute(s *Server, params gen.GetReadingsBetweenDatesParams) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/readings/between", func(c *gin.Context) {
		s.GetReadingsBetweenDates(c, params)
	})
	return router
}

func mockGetLatestReadingsSuccessful() ([]gen.Reading, error) {
	val := 21.5
	return []gen.Reading{
		{SensorName: "Test", MeasurementType: "temperature", Unit: "°C", NumericValue: &val, Time: "2025-08-31T10:00:00Z"},
	}, nil
}

func mockGetReadingsBetweenDatesSuccessful(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
	v1 := 22.5
	v2 := 24.5
	v3 := 23.5
	readings := []gen.Reading{
		{SensorName: "sensor1", MeasurementType: "temperature", Unit: "°C", NumericValue: &v1, Time: "2024-01-01T10:00:00Z"},
		{SensorName: "sensor2", MeasurementType: "temperature", Unit: "°C", NumericValue: &v1, Time: "2024-01-02T10:00:00Z"},
		{SensorName: "sensor2", MeasurementType: "temperature", Unit: "°C", NumericValue: &v2, Time: "2024-01-03T10:00:00Z"},
		{SensorName: "sensor2", MeasurementType: "temperature", Unit: "°C", NumericValue: &v3, Time: "2024-01-04T10:00:00Z"},
	}
	return &gen.AggregatedReadingsResponse{
		AggregationInterval: "PT1H",
		AggregationFunction: "avg",
		Readings:            readings,
	}, nil
}

func mockGetReadingsBetweenDatesError(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
	return nil, fmt.Errorf("failed to fetch readings")
}

func TestGetReadingsBetweenDates_Success(t *testing.T) {
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: mockGetReadingsBetweenDatesSuccessful,
	}}

	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: "2024-01-04"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "sensor1")
	assert.Contains(t, w.Body.String(), "22.5")
	assert.Contains(t, w.Body.String(), "aggregation_interval")
	assert.Contains(t, w.Body.String(), "aggregation_function")
}

func TestGetReadingsBetweenDates_MissingStart(t *testing.T) {
	s := new(Server)
	params := gen.GetReadingsBetweenDatesParams{Start: "", End: "2024-01-04"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Start and end dates are required")
}

func TestGetReadingsBetweenDates_MissingEnd(t *testing.T) {
	s := new(Server)
	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: ""}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Start and end dates are required")
}

func TestGetReadingsBetweenDates_InvalidStartFormat(t *testing.T) {
	s := new(Server)
	params := gen.GetReadingsBetweenDatesParams{Start: "invalid-date", End: "2024-01-04"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid start parameter")
}

func TestGetReadingsBetweenDates_InvalidEndFormat(t *testing.T) {
	s := new(Server)
	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: "invalid-date"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid end parameter")
}

func TestGetReadingsBetweenDates_ServiceError(t *testing.T) {
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: mockGetReadingsBetweenDatesError,
	}}

	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: "2024-01-04"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to fetch readings")
}

func TestGetReadingsBetweenDates_InvalidAggregationFunction(t *testing.T) {
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			return nil, &service.ErrUnsupportedAggregationFunction{Function: overrideFunction}
		},
	}}

	fn := gen.GetReadingsBetweenDatesParamsAggregationFunction("invalid_fn")
	params := gen.GetReadingsBetweenDatesParams{
		Start:               "2024-01-01",
		End:                 "2024-01-04",
		AggregationFunction: &fn,
	}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetReadingsBetweenDates_WithSensorFilter(t *testing.T) {
	var capturedSensor string
	val := 21.0
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			capturedSensor = sensorName
			return &gen.AggregatedReadingsResponse{
				AggregationInterval: "raw",
				AggregationFunction: "none",
				Readings: []gen.Reading{
					{SensorName: "Office", MeasurementType: "temperature", Unit: "°C", NumericValue: &val, Time: "2024-01-01T10:00:00Z"},
				},
			}, nil
		},
	}}

	sensor := "Office"
	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: "2024-01-04", Sensor: &sensor}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Office", capturedSensor)
	assert.Contains(t, w.Body.String(), "Office")
}

func TestGetReadingsBetweenDates_NoSensorFilter(t *testing.T) {
	var capturedSensor string
	val := 22.5
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			capturedSensor = sensorName
			return &gen.AggregatedReadingsResponse{
				AggregationInterval: "raw",
				AggregationFunction: "none",
				Readings: []gen.Reading{
					{SensorName: "sensor1", MeasurementType: "temperature", Unit: "°C", NumericValue: &val, Time: "2024-01-01T10:00:00Z"},
				},
			}, nil
		},
	}}

	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: "2024-01-04"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", capturedSensor)
}

func TestGetReadingsBetweenDates_ISODatetime(t *testing.T) {
	var capturedStart, capturedEnd string
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			capturedStart = startDate
			capturedEnd = endDate
			return &gen.AggregatedReadingsResponse{AggregationInterval: "raw", AggregationFunction: "none", Readings: []gen.Reading{}}, nil
		},
	}}

	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01T10:00:00Z", End: "2024-01-01T16:00:00Z"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2024-01-01 10:00:00", capturedStart)
	assert.Equal(t, "2024-01-01 16:00:00", capturedEnd)
}

func TestGetReadingsBetweenDates_ISODatetimeWithOffset(t *testing.T) {
	var capturedStart, capturedEnd string
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			capturedStart = startDate
			capturedEnd = endDate
			return &gen.AggregatedReadingsResponse{AggregationInterval: "raw", AggregationFunction: "none", Readings: []gen.Reading{}}, nil
		},
	}}

	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01T11:00:00+01:00", End: "2024-01-01T17:00:00+01:00"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2024-01-01 10:00:00", capturedStart)
	assert.Equal(t, "2024-01-01 16:00:00", capturedEnd)
}

func TestGetReadingsBetweenDates_DateOnlyExpandsToFullDay(t *testing.T) {
	var capturedStart, capturedEnd string
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			capturedStart = startDate
			capturedEnd = endDate
			return &gen.AggregatedReadingsResponse{AggregationInterval: "raw", AggregationFunction: "none", Readings: []gen.Reading{}}, nil
		},
	}}

	params := gen.GetReadingsBetweenDatesParams{Start: "2024-01-01", End: "2024-01-01"}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2024-01-01 00:00:00", capturedStart)
	assert.Equal(t, "2024-01-01 23:59:59", capturedEnd)
}

func TestGetReadingsBetweenDates_TypedAggregationParams(t *testing.T) {
	var capturedInterval, capturedFunction string
	s := &Server{readingsService: &mockReadingsService{
		ServiceGetBetweenDatesFunc: func(ctx context.Context, startDate, endDate, sensorName, measurementType string, overrideInterval, overrideFunction string) (*gen.AggregatedReadingsResponse, error) {
			capturedInterval = overrideInterval
			capturedFunction = overrideFunction
			return &gen.AggregatedReadingsResponse{
				AggregationInterval: gen.AggregatedReadingsResponseAggregationInterval(overrideInterval),
				AggregationFunction: gen.AggregatedReadingsResponseAggregationFunction(overrideFunction),
				Readings:            []gen.Reading{},
			}, nil
		},
	}}

	agg := gen.GetReadingsBetweenDatesParamsAggregation("PT1H")
	fn := gen.GetReadingsBetweenDatesParamsAggregationFunction("count")
	params := gen.GetReadingsBetweenDatesParams{
		Start:               "2024-01-01",
		End:                 "2024-01-04",
		Aggregation:         &agg,
		AggregationFunction: &fn,
	}
	router := setupReadingsBetweenRoute(s, params)

	req := httptest.NewRequest("GET", "/api/readings/between", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "PT1H", capturedInterval)
	assert.Equal(t, "count", capturedFunction)
}
