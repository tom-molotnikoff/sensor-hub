package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	appProps "example/sensorHub/application_properties"
	database "example/sensorHub/db"
	gen "example/sensorHub/gen"
	"log/slog"
	"math"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AuthServiceInterface interface {
	Login(ctx context.Context, username, password, ip, userAgent string) (rawToken string, csrfToken string, mustChange bool, err error)
	ValidateSession(ctx context.Context, rawToken string) (*gen.User, error)
	Logout(ctx context.Context, rawToken string) error
	ChangePassword(ctx context.Context, userId int, newPassword string) error
	CreateInitialAdminIfNone(ctx context.Context, username, password string) error
	ListSessionsForUser(ctx context.Context, userId int) ([]database.SessionInfo, error)
	RevokeSessionById(ctx context.Context, sessionId int64) error
	RevokeSessionByIdWithActor(ctx context.Context, sessionId int64, revokedByUserId *int, reason *string) error
	GetCSRFForToken(ctx context.Context, rawToken string) (string, error)
	GetSessionIdForToken(ctx context.Context, rawToken string) (int64, error)
}

type AuthService struct {
	userRepo    database.UserRepository
	sessionRepo database.SessionRepository
	failedRepo  database.FailedLoginRepository
	roleRepo    database.RoleRepository
	logger      *slog.Logger
}

func NewAuthService(u database.UserRepository, s database.SessionRepository, f database.FailedLoginRepository, r database.RoleRepository, logger *slog.Logger) *AuthService {
	return &AuthService{userRepo: u, sessionRepo: s, failedRepo: f, roleRepo: r, logger: logger.With("component", "auth_service")}
}

func (a *AuthService) generateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *AuthService) bcryptHash(password string) (string, error) {
	cost := 12
	if appProps.AppConfig() != nil && appProps.AppConfig().AuthBcryptCost > 0 {
		cost = appProps.AppConfig().AuthBcryptCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type TooManyAttemptsError struct {
	RetryAfterSeconds int
	FailedByUser      int
	FailedByIP        int
	Threshold         int
	Exponent          int
}

func (e *TooManyAttemptsError) Error() string {
	return "too many failed login attempts"
}

func (a *AuthService) Login(ctx context.Context, username, password, ip, userAgent string) (string, string, bool, error) {
	windowMinutes := 15
	threshold := 5
	baseSeconds := 2
	maxSeconds := 300
	if appProps.AppConfig() != nil {
		if appProps.AppConfig().AuthLoginBackoffWindowMinutes > 0 {
			windowMinutes = appProps.AppConfig().AuthLoginBackoffWindowMinutes
		}
		if appProps.AppConfig().AuthLoginBackoffThreshold > 0 {
			threshold = appProps.AppConfig().AuthLoginBackoffThreshold
		}
		if appProps.AppConfig().AuthLoginBackoffBaseSeconds > 0 {
			baseSeconds = appProps.AppConfig().AuthLoginBackoffBaseSeconds
		}
		if appProps.AppConfig().AuthLoginBackoffMaxSeconds > 0 {
			maxSeconds = appProps.AppConfig().AuthLoginBackoffMaxSeconds
		}
	}

	ipAllowedOnce := ipBlocker.consumeAllowOnceIfReady("ip:" + ip)
	userAllowedOnce := userBlocker.consumeAllowOnceIfReady("user:" + username)
	if ipAllowedOnce {
		a.logger.Info("allowing post-block login attempt", "type", "ip", "ip", ip)
	}
	if userAllowedOnce {
		a.logger.Info("allowing post-block login attempt", "type", "user", "username", username)
	}

	if remaining := ipBlocker.getRemainingSeconds("ip:" + ip); remaining > 0 {
		if !ipAllowedOnce {
			failedByUser := 0
			if c, err := a.failedRepo.CountRecentFailedAttemptsByUsername(ctx, username, time.Duration(windowMinutes)*time.Minute); err == nil {
				failedByUser = c
			}
			failedByIP := 0
			if c, err := a.failedRepo.CountRecentFailedAttemptsByIP(ctx, ip, time.Duration(windowMinutes)*time.Minute); err == nil {
				failedByIP = c
			}
			expiresAt := time.Now().Add(time.Duration(remaining) * time.Second).UTC().Format(time.RFC3339)
			a.logger.Warn("login blocked by ip rate limiter", "expires_at", expiresAt, "remaining_seconds", remaining, "failed_by_user", failedByUser, "failed_by_ip", failedByIP)
			return "", "", false, &TooManyAttemptsError{RetryAfterSeconds: remaining, FailedByUser: failedByUser, FailedByIP: failedByIP}
		}
	}
	if remaining := userBlocker.getRemainingSeconds("user:" + username); remaining > 0 {
		if !userAllowedOnce {
			failedByUser := 0
			if c, err := a.failedRepo.CountRecentFailedAttemptsByUsername(ctx, username, time.Duration(windowMinutes)*time.Minute); err == nil {
				failedByUser = c
			}
			failedByIP := 0
			if c, err := a.failedRepo.CountRecentFailedAttemptsByIP(ctx, ip, time.Duration(windowMinutes)*time.Minute); err == nil {
				failedByIP = c
			}
			expiresAt := time.Now().Add(time.Duration(remaining) * time.Second).UTC().Format(time.RFC3339)
			a.logger.Warn("login blocked by user rate limiter", "expires_at", expiresAt, "remaining_seconds", remaining, "failed_by_user", failedByUser, "failed_by_ip", failedByIP)
			return "", "", false, &TooManyAttemptsError{RetryAfterSeconds: remaining, FailedByUser: failedByUser, FailedByIP: failedByIP}
		}
	}

	failedByUser := 0
	if c, err := a.failedRepo.CountRecentFailedAttemptsByUsername(ctx, username, time.Duration(windowMinutes)*time.Minute); err == nil {
		failedByUser = c
	}
	failedByIP := 0
	if c, err := a.failedRepo.CountRecentFailedAttemptsByIP(ctx, ip, time.Duration(windowMinutes)*time.Minute); err == nil {
		failedByIP = c
	}
	failedCount := failedByUser
	if failedByIP > failedCount {
		failedCount = failedByIP
	}
	if failedCount >= threshold {
		exponent := failedCount - threshold
		d := float64(baseSeconds) * math.Pow(2, float64(exponent))
		delay := int(math.Min(float64(maxSeconds), d))
		minBlock := 30
		finalDelay := delay
		if finalDelay < minBlock {
			finalDelay = minBlock
		}
		if !(ipAllowedOnce || userAllowedOnce) {
			expiresAt := time.Now().Add(time.Duration(finalDelay) * time.Second).UTC().Format(time.RFC3339)
			a.logger.Warn("blocking login due to failed attempts", "block_seconds", finalDelay, "expires_at", expiresAt, "failed_by_user", failedByUser, "failed_by_ip", failedByIP, "threshold", threshold, "exponent", exponent)
			ipBlocker.blockFor("ip:"+ip, finalDelay)
			userBlocker.blockFor("user:"+username, finalDelay)
			return "", "", false, &TooManyAttemptsError{RetryAfterSeconds: finalDelay, FailedByUser: failedByUser, FailedByIP: failedByIP, Threshold: threshold, Exponent: exponent}
		}
		a.logger.Info("allow-once consumed, proceeding to credential check", "failed_count", failedCount, "threshold", threshold)
	}

	user, passwordHash, err := a.userRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return "", "", false, err
	}
	if user == nil {
		err = a.failedRepo.RecordFailedAttempt(ctx, username, nil, ip, "no_such_user")
		if err != nil {
			a.logger.Error("error recording failed login attempt", "error", err)
		}
		return "", "", false, errors.New("invalid credentials")
	}
	if user.Disabled {
		return "", "", false, errors.New("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		err = a.failedRepo.RecordFailedAttempt(ctx, username, &user.Id, ip, "bad_password")
		if err != nil {
			a.logger.Error("error recording failed login attempt", "error", err)
		}
		return "", "", false, errors.New("invalid credentials")
	}

	token, err := a.generateToken(32)
	if err != nil {
		a.logger.Error("error generating session token", "error", err)
		return "", "", false, err
	}

	ttlMinutes := 60 * 24 * 30 // 30 days default
	if appProps.AppConfig() != nil && appProps.AppConfig().AuthSessionTTLMinutes > 0 {
		ttlMinutes = appProps.AppConfig().AuthSessionTTLMinutes
	}
	expires := time.Now().Add(time.Duration(ttlMinutes) * time.Minute)
	csrf, err := a.sessionRepo.CreateSession(ctx, user.Id, token, expires, ip, userAgent)
	if err != nil {
		a.logger.Error("error creating session", "error", err)
		return "", "", false, err
	}

	ipBlocker.forceClearAllowOnce("ip:" + ip)
	userBlocker.forceClearAllowOnce("user:" + username)

	if appProps.AppConfig() != nil {
		windowMinutes := appProps.AppConfig().AuthLoginBackoffWindowMinutes
		if windowMinutes <= 0 {
			windowMinutes = 15
		}
		if err := a.failedRepo.DeleteRecentFailedAttemptsByIP(ctx, ip, time.Duration(windowMinutes)*time.Minute); err != nil {
			a.logger.Error("error clearing failed login attempts", "ip", ip, "error", err)
		}
	} else {
		if err := a.failedRepo.DeleteRecentFailedAttemptsByIP(ctx, ip, time.Duration(15)*time.Minute); err != nil {
			a.logger.Error("error clearing failed login attempts", "ip", ip, "error", err)
		}
	}
	return token, csrf, user.MustChangePassword, nil
}

func (a *AuthService) ValidateSession(ctx context.Context, rawToken string) (*gen.User, error) {
	userId, err := a.sessionRepo.GetUserIdByToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if userId == 0 {
		return nil, nil
	}
	user, err := a.userRepo.GetUserById(ctx, userId)
	if err != nil {
		return nil, err
	}
	if a.roleRepo != nil {
		perms, err := a.roleRepo.GetPermissionsForUser(ctx, user.Id)
		if err == nil {
			user.Permissions = perms
		}
	}
	return user, nil
}

func (a *AuthService) Logout(ctx context.Context, rawToken string) error {
	return a.sessionRepo.DeleteSessionByToken(ctx, rawToken)
}

func (a *AuthService) ChangePassword(ctx context.Context, userId int, newPassword string) error {
	hash, err := a.bcryptHash(newPassword)
	if err != nil {
		return err
	}
	return a.userRepo.UpdatePassword(ctx, userId, hash, false)
}

func (a *AuthService) CreateInitialAdminIfNone(ctx context.Context, username, password string) error {
	users, err := a.userRepo.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil // already users present
	}
	hash, err := a.bcryptHash(password)
	if err != nil {
		return err
	}
	user := gen.User{Username: username, Email: "", Disabled: false, MustChangePassword: true, Roles: []string{RoleAdmin}}
	id, err := a.userRepo.CreateUser(ctx, user, hash)
	if err != nil {
		return err
	}
	err = a.userRepo.AssignRoleToUser(ctx, id, RoleAdmin)
	if err != nil {
		return err
	}
	return nil
}

func (a *AuthService) ListSessionsForUser(ctx context.Context, userId int) ([]database.SessionInfo, error) {
	return a.sessionRepo.ListSessionsForUser(ctx, userId)
}

func (a *AuthService) RevokeSessionByIdWithActor(ctx context.Context, sessionId int64, revokedByUserId *int, reason *string) error {
	err := a.sessionRepo.InsertSessionAudit(ctx, sessionId, revokedByUserId, "revoked", reason)
	if err != nil {
		a.logger.Error("error inserting session audit record", "session_id", sessionId, "error", err)
	}
	return a.sessionRepo.RevokeSessionById(ctx, sessionId)
}

func (a *AuthService) RevokeSessionById(ctx context.Context, sessionId int64) error {
	return a.RevokeSessionByIdWithActor(ctx, sessionId, nil, nil)
}

func (a *AuthService) GetCSRFForToken(ctx context.Context, rawToken string) (string, error) {
	return a.sessionRepo.GetCSRFForToken(ctx, rawToken)
}

func (a *AuthService) GetSessionIdForToken(ctx context.Context, rawToken string) (int64, error) {
	return a.sessionRepo.GetSessionIdByToken(ctx, rawToken)
}
