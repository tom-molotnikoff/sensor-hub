package service

import (
	"context"

	"example/sensorHub/alerting"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"log/slog"
)

type AlertRuleCache interface {
	InvalidateRules()
}

type AlertManagementService struct {
	alertRepo database.AlertRepository
	ruleCache AlertRuleCache
	logger    *slog.Logger
}

func NewAlertManagementService(alertRepo database.AlertRepository, ruleCache AlertRuleCache, logger *slog.Logger) AlertManagementServiceInterface {
	return &AlertManagementService{
		alertRepo: alertRepo,
		ruleCache: ruleCache,
		logger:    logger.With("component", "alert_management_service"),
	}
}

func (s *AlertManagementService) invalidateRuleCache() {
	if s.ruleCache != nil {
		s.ruleCache.InvalidateRules()
	}
}

func (s *AlertManagementService) ServiceGetAllAlertRules(ctx context.Context) ([]alerting.AlertRule, error) {
	return s.alertRepo.GetAllAlertRules(ctx)
}

func (s *AlertManagementService) ServiceGetAlertRuleByID(ctx context.Context, ruleID int) (*alerting.AlertRule, error) {
	return s.alertRepo.GetAlertRuleByID(ctx, ruleID)
}

func (s *AlertManagementService) ServiceGetAlertRuleBySensorID(ctx context.Context, sensorID int) (*alerting.AlertRule, error) {
	return s.alertRepo.GetAlertRuleBySensorID(ctx, sensorID)
}

func (s *AlertManagementService) ServiceGetAlertRulesBySensorID(ctx context.Context, sensorID int) ([]alerting.AlertRule, error) {
	return s.alertRepo.GetAlertRulesBySensorID(ctx, sensorID)
}

func (s *AlertManagementService) ServiceCreateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	defer s.invalidateRuleCache()
	return s.alertRepo.CreateAlertRule(ctx, rule)
}

func (s *AlertManagementService) ServiceUpdateAlertRule(ctx context.Context, rule *alerting.AlertRule) error {
	defer s.invalidateRuleCache()
	return s.alertRepo.UpdateAlertRule(ctx, rule)
}

func (s *AlertManagementService) ServiceDeleteAlertRule(ctx context.Context, ruleID int) error {
	defer s.invalidateRuleCache()
	return s.alertRepo.DeleteAlertRule(ctx, ruleID)
}

func (s *AlertManagementService) ServiceGetAlertHistory(ctx context.Context, sensorID int, limit int) ([]gen.AlertHistoryEntry, error) {
	return s.alertRepo.GetAlertHistory(ctx, sensorID, limit)
}
