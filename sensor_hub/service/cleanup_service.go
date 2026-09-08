package service

import (
	"context"
	"fmt"

	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"example/sensorHub/periodic"
	"example/sensorHub/telemetry"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

const reclaimChunkPages = 512

type sqliteInstruments struct {
	sizeBytes     metric.Int64Gauge
	freelistBytes metric.Int64Gauge
	freelistRatio metric.Float64Gauge
}

func newSQLiteInstruments() *sqliteInstruments {
	meter := telemetry.Meter("sqlite")

	sizeBytes, _ := meter.Int64Gauge("sqlite.database.size_bytes",
		metric.WithDescription("Total SQLite database size"),
		metric.WithUnit("By"))

	freelistBytes, _ := meter.Int64Gauge("sqlite.database.freelist_bytes",
		metric.WithDescription("Reclaimable space in SQLite freelist"),
		metric.WithUnit("By"))

	freelistRatio, _ := meter.Float64Gauge("sqlite.database.freelist_ratio",
		metric.WithDescription("Proportion of free pages in SQLite database"),
		metric.WithUnit("1"))

	return &sqliteInstruments{
		sizeBytes:     sizeBytes,
		freelistBytes: freelistBytes,
		freelistRatio: freelistRatio,
	}
}

type cleanupService struct {
	sensorRepo       database.SensorRepositoryInterface[gen.Sensor]
	readingsRepo     database.ReadingsRepository
	failedRepo       database.FailedLoginRepository
	notificationRepo database.NotificationRepository
	alertRepo        database.AlertRepository
	maintenanceRepo  database.MaintenanceRepository
	readingsSampler  ReadingsSamplerInterface
	logger           *slog.Logger
	metrics          *sqliteInstruments
}

func NewCleanupService(sensorRepo database.SensorRepositoryInterface[gen.Sensor], readingsRepo database.ReadingsRepository, failedRepo database.FailedLoginRepository, notificationRepo database.NotificationRepository, alertRepo database.AlertRepository, maintenanceRepo database.MaintenanceRepository, readingsSampler ReadingsSamplerInterface, logger *slog.Logger) CleanupServiceInterface {
	return &cleanupService{
		sensorRepo:       sensorRepo,
		readingsRepo:     readingsRepo,
		failedRepo:       failedRepo,
		notificationRepo: notificationRepo,
		alertRepo:        alertRepo,
		maintenanceRepo:  maintenanceRepo,
		readingsSampler:  readingsSampler,
		logger:           logger.With("component", "cleanup_service"),
		metrics:          newSQLiteInstruments(),
	}
}

func (cs *cleanupService) StartPeriodicCleanup(ctx context.Context) {
	periodic.RunTask(ctx, periodic.TaskConfig{
		Name: "data_cleanup",
		Interval: func() time.Duration {
			return time.Duration(appProps.AppConfig().DataCleanupIntervalHours) * time.Hour
		},
		Logger:         cs.logger,
		RunImmediately: true,
	}, func(ctx context.Context) error {
		// read at each run so retention changes apply without a restart
		cfg := appProps.AppConfig()
		return cs.performCleanup(ctx, cfg.HealthHistoryRetentionDays, cfg.SensorDataRetentionDays, cfg.FailedLoginRetentionDays, cfg.AlertHistoryRetentionDays)
	})
}

func (cs *cleanupService) performCleanup(ctx context.Context, healthHistoryRetentionDays int, sensorDataRetentionDays int, failedLoginRetentionDays int, alertHistoryRetentionDays int) error {
	if sensorDataRetentionDays > 0 {
		cs.logger.Debug("cleaning up old sensor readings", "retention_days", sensorDataRetentionDays)

		// Apply per-sensor retention first, collecting the IDs of sensors that have a custom value.
		customSensors, err := cs.sensorRepo.GetSensorsWithRetention(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch sensors with custom retention: %w", err)
		}

		customSensorIds := make([]int, 0, len(customSensors))
		for _, sensor := range customSensors {
			if sensor.RetentionHours == nil {
				continue
			}
			cutoff := time.Now().Add(-time.Duration(*sensor.RetentionHours) * time.Hour)
			if err := cs.readingsRepo.DeleteReadingsOlderThanForSensor(ctx, cutoff, sensor.Id); err != nil {
				return fmt.Errorf("failed per-sensor cleanup for sensor %d: %w", sensor.Id, err)
			}
			customSensorIds = append(customSensorIds, sensor.Id)
		}

		// Apply global retention to all remaining sensors (excluding those already handled above).
		globalCutoff := time.Now().AddDate(0, 0, -sensorDataRetentionDays)
		if err := cs.readingsRepo.DeleteReadingsOlderThanExcludingSensors(ctx, globalCutoff, customSensorIds); err != nil {
			return fmt.Errorf("failed global cleanup: %w", err)
		}
		cs.logger.Info("deleted old sensor readings", "retention_days", sensorDataRetentionDays, "custom_sensors", len(customSensorIds))
	}
	cs.logger.Info("sensor readings cleanup completed")

	if healthHistoryRetentionDays > 0 {
		cs.logger.Debug("cleaning up old health history", "retention_days", healthHistoryRetentionDays)
		err := cs.sensorRepo.DeleteHealthHistoryOlderThan(ctx, time.Now().AddDate(0, 0, -healthHistoryRetentionDays))
		if err != nil {
			return err
		}
		cs.logger.Info("deleted old health history records", "retention_days", healthHistoryRetentionDays)
	}
	cs.logger.Info("health history cleanup completed")

	if failedLoginRetentionDays > 0 {
		cs.logger.Debug("cleaning up old failed login attempts", "retention_days", failedLoginRetentionDays)
		threshold := time.Now().AddDate(0, 0, -failedLoginRetentionDays)
		if err := cs.failedRepo.DeleteAttemptsOlderThan(ctx, threshold); err != nil {
			return err
		}
		cs.logger.Info("deleted old failed login attempts", "retention_days", failedLoginRetentionDays)
	}

	// Clean up old notifications (90 days retention for dismissed notifications)
	if cs.notificationRepo != nil {
		notificationRetentionDays := 90
		cs.logger.Debug("cleaning up old notifications", "retention_days", notificationRetentionDays)
		threshold := time.Now().AddDate(0, 0, -notificationRetentionDays)
		deleted, err := cs.notificationRepo.DeleteOldNotifications(ctx, threshold)
		if err != nil {
			cs.logger.Warn("failed to cleanup old notifications", "error", err)
		} else if deleted > 0 {
			cs.logger.Info("deleted old notifications", "count", deleted, "retention_days", notificationRetentionDays)
		}
	}

	// Clean up old alert history
	if cs.alertRepo != nil && alertHistoryRetentionDays > 0 {
		cs.logger.Debug("cleaning up old alert history", "retention_days", alertHistoryRetentionDays)
		threshold := time.Now().AddDate(0, 0, -alertHistoryRetentionDays)
		deleted, err := cs.alertRepo.DeleteAlertHistoryOlderThan(ctx, threshold)
		if err != nil {
			cs.logger.Warn("failed to cleanup old alert history", "error", err)
		} else if deleted > 0 {
			cs.logger.Info("deleted old alert history", "count", deleted, "retention_days", alertHistoryRetentionDays)
		}
	}

	if cs.maintenanceRepo != nil {
		if err := cs.performDatabaseMaintenance(ctx); err != nil {
			cs.logger.Warn("database maintenance failed", "error", err)
		}
	}

	if cs.readingsSampler != nil {
		if err := cs.readingsSampler.Sample(ctx); err != nil {
			cs.logger.Warn("failed to sample readings row counts", "error", err)
		}
	}

	return nil
}

func (cs *cleanupService) performDatabaseMaintenance(ctx context.Context) error {
	statsBefore, err := cs.maintenanceRepo.DatabaseStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pre-maintenance stats: %w", err)
	}

	cs.logger.Debug("database stats before maintenance",
		"page_count", statsBefore.PageCount,
		"freelist_count", statsBefore.FreelistCount,
		"page_size", statsBefore.PageSize,
		"size_bytes", statsBefore.SizeBytes(),
		"freelist_bytes", statsBefore.FreelistBytes(),
	)

	started := time.Now()

	var pagesFreed int64
	for {
		freed, err := cs.maintenanceRepo.ReclaimFreePages(ctx, reclaimChunkPages)
		if err != nil {
			return fmt.Errorf("failed to reclaim free pages: %w", err)
		}
		if freed <= 0 {
			break
		}
		pagesFreed += freed
	}

	if err := cs.maintenanceRepo.Optimise(ctx); err != nil {
		cs.logger.Warn("PRAGMA optimize failed", "error", err)
	}

	if checkpoint, err := cs.maintenanceRepo.Checkpoint(ctx); err != nil {
		cs.logger.Warn("WAL checkpoint failed", "error", err)
	} else if checkpoint.Busy != 0 {
		cs.logger.Debug("WAL checkpoint blocked before the log could be truncated",
			"log_pages", checkpoint.LogPages,
			"checkpointed_pages", checkpoint.CheckpointedPages,
		)
	} else {
		cs.logger.Debug("WAL checkpoint truncated the log",
			"log_pages", checkpoint.LogPages,
			"checkpointed_pages", checkpoint.CheckpointedPages,
		)
	}

	statsAfter, err := cs.maintenanceRepo.DatabaseStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get post-maintenance stats: %w", err)
	}

	cs.logger.Info("database maintenance completed",
		"pages_freed", pagesFreed,
		"bytes_reclaimed", statsBefore.SizeBytes()-statsAfter.SizeBytes(),
		"duration_ms", time.Since(started).Milliseconds(),
		"page_count", statsAfter.PageCount,
		"freelist_count", statsAfter.FreelistCount,
		"size_bytes", statsAfter.SizeBytes(),
		"freelist_bytes", statsAfter.FreelistBytes(),
	)

	cs.metrics.sizeBytes.Record(ctx, statsAfter.SizeBytes())
	cs.metrics.freelistBytes.Record(ctx, statsAfter.FreelistBytes())
	cs.metrics.freelistRatio.Record(ctx, statsAfter.FreelistRatio())

	return nil
}
