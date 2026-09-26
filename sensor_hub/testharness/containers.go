//go:build integration

package testharness

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/moby/go-archive"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MockSensor represents a running mock sensor container with its accessible URL.
type MockSensor struct {
	Container testcontainers.Container
	URL       string // e.g. "http://localhost:55001/temperature"
}

// StartMockSensors builds the mock sensor Docker image and starts n containers.
// Containers are terminated when the test completes via t.Cleanup.
func StartMockSensors(t *testing.T, n int) []MockSensor {
	t.Helper()
	ctx := context.Background()

	sensors, cleanup, err := startMockSensors(ctx, n)
	if err != nil {
		t.Fatalf("failed to start mock sensors: %v", err)
	}
	t.Cleanup(cleanup)

	return sensors
}

// StartMockSensorsForMain is like StartMockSensors but for use in TestMain
// where *testing.T is not available. Returns a cleanup function.
func StartMockSensorsForMain(ctx context.Context, n int) ([]MockSensor, func(), error) {
	return startMockSensors(ctx, n)
}

func startMockSensors(ctx context.Context, n int) ([]MockSensor, func(), error) {
	buildContext, err := mocksBuildContext()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to archive mocks build context: %w", err)
	}

	sensors := make([]MockSensor, 0, n)
	var containers []testcontainers.Container

	cleanup := func() {
		for _, c := range containers {
			_ = c.Terminate(ctx)
		}
	}

	for i := 0; i < n; i++ {
		contextArchive := bytes.NewReader(buildContext)
		req := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				ContextArchive: contextArchive,
				Dockerfile:     "dockerfiles/mocks.dockerfile",
				BuildOptionsModifier: func(opts *client.ImageBuildOptions) {
					opts.Version = build.BuilderBuildKit
					contextArchive.Reset(buildContext)
				},
			},
			Cmd:          []string{"python", "http_sensor.py"},
			ExposedPorts: []string{"5000/tcp"},
			WaitingFor:   wait.ForHTTP("/temperature").WithPort("5000/tcp").WithStartupTimeout(60 * time.Second),
		}
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to start mock sensor container %d: %w", i, err)
		}
		containers = append(containers, container)

		host, err := container.Host(ctx)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to get container host: %w", err)
		}
		port, err := container.MappedPort(ctx, "5000/tcp")
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to get mapped port: %w", err)
		}

		url := fmt.Sprintf("http://%s:%s", host, port.Port())
		sensors = append(sensors, MockSensor{Container: container, URL: url})
	}

	return sensors, cleanup, nil
}

func mocksBuildContext() ([]byte, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	devstack := filepath.Join(filepath.Dir(thisFile), "..", "devstack")
	tarball, err := archive.TarWithOptions(devstack, &archive.TarOptions{})
	if err != nil {
		return nil, err
	}
	defer tarball.Close()
	return io.ReadAll(tarball)
}
