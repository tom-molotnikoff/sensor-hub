package periodic

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime/debug"
	"time"
)

// TaskConfig configures a supervised periodic task.
type TaskConfig struct {
	Name           string
	Interval       func() time.Duration // read before each wait, so changes apply from the next cycle
	Logger         *slog.Logger
	RunImmediately bool // if true, run the task once before waiting for the first tick
}

const (
	initialBackoff = 5 * time.Second
	maxBackoff     = 5 * time.Minute

	// fallbackInterval is used when the interval function returns a
	// non-positive duration, which would otherwise spin the loop.
	fallbackInterval = time.Minute
)

// RunTask launches a supervised goroutine that executes task on every tick.
// Ticks are fixed-delay: the next wait is armed after the task returns, so the
// effective period is the interval plus the task's run time, and a long run
// never causes back-to-back catch-up executions.
// On panic the goroutine logs the stack trace, backs off exponentially, and restarts.
// The goroutine exits cleanly when ctx is cancelled.
func RunTask(ctx context.Context, cfg TaskConfig, task func(ctx context.Context) error) {
	Supervise(ctx, cfg.Name, cfg.Logger, func(ctx context.Context, healthy func()) {
		runLoop(ctx, cfg, task, healthy)
	})
}

// Supervise reruns loop after a backoff whenever it panics. loop should call
// healthy after useful work so a later panic backs off from the start again.
func Supervise(ctx context.Context, name string, logger *slog.Logger, loop func(ctx context.Context, healthy func())) {
	go func() {
		consecutivePanics := 0
		healthy := func() { consecutivePanics = 0 }
		for {
			if !runRecovered(ctx, name, logger, loop, healthy) {
				return
			}
			consecutivePanics++

			backoff := time.Duration(float64(initialBackoff) * math.Pow(2, float64(consecutivePanics-1)))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			logger.Error("restarting supervised task after backoff",
				"task", name,
				"backoff", backoff.String(),
				"consecutive_panics", consecutivePanics,
			)

			select {
			case <-ctx.Done():
				logger.Info("supervised task stopping during backoff", "task", name, "reason", ctx.Err())
				return
			case <-time.After(backoff):
			}
		}
	}()
}

func runRecovered(ctx context.Context, name string, logger *slog.Logger, loop func(ctx context.Context, healthy func()), healthy func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("supervised task panicked",
				"task", name,
				"panic", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
			)
			panicked = true
		}
	}()
	loop(ctx, healthy)
	return false
}

func runLoop(ctx context.Context, cfg TaskConfig, task func(ctx context.Context) error, healthy func()) {
	cfg.Logger.Info("periodic task started", "task", cfg.Name, "interval", clamped(cfg.Interval()).String())

	if cfg.RunImmediately {
		executeTask(ctx, cfg, task, healthy)
	}

	for {
		timer := time.NewTimer(interval(cfg))
		select {
		case <-ctx.Done():
			timer.Stop()
			cfg.Logger.Info("periodic task stopping", "task", cfg.Name, "reason", ctx.Err())
			return
		case <-timer.C:
			executeTask(ctx, cfg, task, healthy)
		}
	}
}

// interval reads the configured interval, clamping a non-positive value to
// fallbackInterval so a bad runtime read cannot spin or stall the loop.
func interval(cfg TaskConfig) time.Duration {
	d := cfg.Interval()
	if d <= 0 {
		cfg.Logger.Error("periodic task interval is not positive, using fallback",
			"task", cfg.Name,
			"interval", d.String(),
			"fallback", fallbackInterval.String(),
		)
	}
	return clamped(d)
}

// clamped is the clamp without the error log, for log lines that report the
// wait without owning it.
func clamped(d time.Duration) time.Duration {
	if d <= 0 {
		return fallbackInterval
	}
	return d
}

func executeTask(ctx context.Context, cfg TaskConfig, task func(ctx context.Context) error, healthy func()) {
	err := task(ctx)
	if err != nil {
		cfg.Logger.Error("periodic task error", "task", cfg.Name, "error", err)
	} else {
		healthy()
		cfg.Logger.Info("periodic task completed", "task", cfg.Name, "next_in", clamped(cfg.Interval()).String())
	}
}
