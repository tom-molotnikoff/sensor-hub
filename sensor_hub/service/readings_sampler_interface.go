package service

import (
	"context"

	gen "example/sensorHub/gen"
)

type ReadingsSamplerInterface interface {
	Sample(ctx context.Context) error
	LatestSample() gen.TotalReadingsSample
}
