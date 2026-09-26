package main

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDevice struct {
	reading string
	state   map[string]string
}

func readMock(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "mocks", name))
	require.NoError(t, err)
	return string(source)
}

func mockDevices(t *testing.T, source string) map[string]mockDevice {
	t.Helper()
	devices := make(map[string]mockDevice)
	entry := regexp.MustCompile(`Device\(\s*"([^"]+)",\s*"[^"]+",\s*\w+,\s*(\w+),\s*\{([^}]*)\}`)
	pair := regexp.MustCompile(`"(\w+)":\s*("?[\w.]+"?)`)
	for _, match := range entry.FindAllStringSubmatch(source, -1) {
		state := make(map[string]string)
		for _, field := range pair.FindAllStringSubmatch(match[3], -1) {
			state[field[1]] = field[2]
		}
		devices[match[1]] = mockDevice{reading: match[2], state: state}
	}
	require.NotEmpty(t, devices, "found no devices in mqtt_devices.py")
	return devices
}

func number(t *testing.T, text string) float64 {
	t.Helper()
	parsed, err := strconv.ParseFloat(text, 64)
	require.NoError(t, err, text)
	return parsed
}

func TestDevices_MirrorTheMockDevices(t *testing.T) {
	source := readMock(t, "mqtt_devices.py")
	devices := mockDevices(t, source)
	drifts := make(map[string][3]float64)
	for _, match := range regexp.MustCompile(`state\["(\w+)"\] = (?:round\()?drift\(state\["\w+"\], ([\d.]+), ([\d.]+), ([\d.]+)\)`).FindAllStringSubmatch(source, -1) {
		drifts[match[1]] = [3]float64{number(t, match[2]), number(t, match[3]), number(t, match[4])}
	}
	chances := make(map[string]float64)
	for _, match := range regexp.MustCompile(`random\.random\(\) < ([\d.]+):\s+state\["(\w+)"\]`).FindAllStringSubmatch(source, -1) {
		chances[match[2]] = number(t, match[1])
	}
	linkQuality := regexp.MustCompile(`"linkquality": random\.randint\((\d+), (\d+)\)`).FindStringSubmatch(source)
	require.NotNil(t, linkQuality)
	require.NotEmpty(t, drifts)
	require.NotEmpty(t, chances)

	for _, device := range mqttDevices {
		mock, ok := devices[device.Name]
		require.True(t, ok, "%s has no mock device", device.Name)
		for _, signal := range device.signals {
			switch signal := signal.(type) {
			case walk:
				drift, ok := drifts[signal.measurement]
				require.True(t, ok, "%s %s does not drift in the mock", device.Name, signal.measurement)
				assert.Equal(t, drift, [3]float64{signal.low, signal.high, signal.step}, "%s %s", device.Name, signal.measurement)
				assert.Equal(t, number(t, mock.state[signal.measurement]), signal.start, "%s %s", device.Name, signal.measurement)
			case toggle:
				assert.Equal(t, chances[signal.measurement], signal.chance, "%s %s", device.Name, signal.measurement)
				assert.Equal(t, mock.state[signal.measurement] == "True", signal.start, "%s %s", device.Name, signal.measurement)
			case steady:
				assert.Equal(t, number(t, mock.state[signal.measurement]), signal.value.number, "%s %s", device.Name, signal.measurement)
			case uniform:
				assert.Equal(t, "link_quality", signal.measurement, device.Name)
				assert.Equal(t, number(t, linkQuality[1]), signal.low, device.Name)
				assert.Equal(t, number(t, linkQuality[2]), signal.high, device.Name)
			case plugDraw:
				assert.Equal(t, "plug_reading", mock.reading, device.Name)
				assert.Equal(t, `"ON"`, mock.state["state"], device.Name)
				assert.Equal(t, number(t, mock.state["min_power"]), signal.minPower, device.Name)
				assert.Equal(t, number(t, mock.state["max_power"]), signal.maxPower, device.Name)
				assert.Equal(t, number(t, mock.state["energy"]), signal.startEnergy, device.Name)
			default:
				t.Fatalf("%s has a signal the test does not know: %T", device.Name, signal)
			}
		}
	}
}

func TestDevices_HTTPMockTemperatureMatchesTheMock(t *testing.T) {
	match := regexp.MustCompile(`random\.uniform\(([\d.]+), ([\d.]+)\)`).FindStringSubmatch(readMock(t, "http_sensor.py"))
	require.NotNil(t, match)
	signals := httpMock{}.device().signals
	require.Len(t, signals, 1)
	temperature, ok := signals[0].(uniform)
	require.True(t, ok)
	assert.Equal(t, "temperature", temperature.measurement)
	assert.Equal(t, number(t, match[1]), temperature.low)
	assert.Equal(t, number(t, match[2]), temperature.high)
}

func TestWalk_EndsOnTheStartWhenTheGapAllowsIt(t *testing.T) {
	temperature := walk{measurement: "temperature", low: 16, high: 28, step: 0.2, decimals: 2, start: 21}
	r := rand.New(rand.NewPCG(1, 2))

	values := temperature.generate(r, 40, map[string]value{"temperature": {number: 27}})[0].values

	assert.InDelta(t, 27, values[0].number, 0.2+0.005, "the first point is one step from the newest reading")
	assert.Equal(t, 21.0, values[len(values)-1].number)
	for i := 1; i < len(values); i++ {
		assert.LessOrEqual(t, values[i].number-values[i-1].number, 0.2+0.005)
		assert.GreaterOrEqual(t, values[i].number-values[i-1].number, -0.2-0.005)
	}
}

func TestWalk_EndsOnTheStartEvenWhenTheGapIsTooShort(t *testing.T) {
	temperature := walk{measurement: "temperature", low: 16, high: 28, step: 0.2, decimals: 2, start: 21}
	r := rand.New(rand.NewPCG(1, 2))

	values := temperature.generate(r, 3, map[string]value{"temperature": {number: 27}})[0].values

	assert.Equal(t, []value{{number: 26.8}, {number: 26.6}, {number: 21}}, values)
}
