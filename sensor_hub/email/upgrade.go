package email

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"example/sensorHub/utils"
)

// legacySMTPProperties is the file 1.5.x kept the sender address in. 2.0 no
// longer reads or writes it, but an upgrade leaves it on disk.
const legacySMTPProperties = "smtp.properties"

// legacyOAuthFiles are the Gmail OAuth client secret and token 1.5.x kept in
// the configuration directory by default, and the 1.5.x properties that could
// put them elsewhere, which an upgraded application.properties may still hold.
var legacyOAuthFiles = []struct{ name, property string }{
	{"credentials.json", "oauth.credentials.file.path"},
	{"token.json", "oauth.token.file.path"},
}

// UpgradeSeeder fills in the email settings a 1.5.x upgrade created.
type UpgradeSeeder interface {
	SeedFromSMTPUser(ctx context.Context, smtpUser string) (bool, error)
}

// FinishUpgrade completes the move from Gmail OAuth to SMTP once the schema
// migration has run. It carries the old smtp.user from smtp.properties into
// the email settings the first time it runs on them, and deletes the OAuth
// credentials and token in the configuration directory, which nothing reads
// any more, logging each file it deletes. A leftover oauth.* property naming
// a file anywhere else gets a warning instead, at every start until a save of
// the properties drops the key: the hub does not delete a file at a path an
// operator chose. Otherwise it does nothing once both are done.
func FinishUpgrade(ctx context.Context, seeder UpgradeSeeder, configDir string, logger *slog.Logger) error {
	smtpUser := legacySMTPUser(configDir, logger)
	seeded, err := seeder.SeedFromSMTPUser(ctx, smtpUser)
	if err != nil {
		return err
	}
	if seeded && smtpUser != "" {
		logger.Info("carried smtp.user into the email settings as the username and from address; enter an SMTP password on the Alerts & Notifications page to send email again",
			"smtp_user", smtpUser)
	}
	for _, file := range legacyOAuthFiles {
		deleteLegacyOAuthFile(filepath.Join(configDir, file.name), logger)
	}
	warnOfOAuthFilesElsewhere(configDir, logger)
	return nil
}

// legacySMTPUser reads smtp.user from a 1.5.x smtp.properties, or returns ""
// when there is no such file or it cannot be read.
func legacySMTPUser(configDir string, logger *slog.Logger) string {
	path := filepath.Join(configDir, legacySMTPProperties)
	props, err := utils.ReadPropertiesFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("could not read smtp.user from the old smtp.properties; set the email username and from address by hand", "path", path, "error", err)
		}
		return ""
	}
	return props["smtp.user"]
}

// warnOfOAuthFilesElsewhere warns about each OAuth file a leftover
// oauth.*.file.path puts outside the configuration directory's default name,
// resolved against the configuration directory as 1.5.x did.
func warnOfOAuthFilesElsewhere(configDir string, logger *slog.Logger) {
	props, err := utils.ReadPropertiesFile(filepath.Join(configDir, "application.properties"))
	if err != nil {
		return
	}
	for _, file := range legacyOAuthFiles {
		configured := props[file.property]
		if configured == "" {
			continue
		}
		if !filepath.IsAbs(configured) {
			configured = filepath.Join(configDir, configured)
		}
		path := filepath.Clean(configured)
		if path == filepath.Join(configDir, file.name) {
			continue
		}
		logger.Warn("a Gmail OAuth file from before 2.0 may still be on disk, which email no longer uses; delete it and revoke the hub's Google token in the Google account",
			"path", path, "property", file.property)
	}
}

// deleteLegacyOAuthFile deletes one OAuth file, if it is there and is a
// regular file, and logs what happened.
func deleteLegacyOAuthFile(path string, logger *slog.Logger) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil && !info.Mode().IsRegular() {
		logger.Warn("left the Gmail OAuth path alone, since it is not a regular file; email no longer uses it", "path", path)
		return
	}
	if err == nil {
		err = os.Remove(path)
	}
	if err != nil {
		logger.Warn("could not delete the Gmail OAuth file, which email no longer uses; delete it by hand", "path", path, "error", err)
		return
	}
	logger.Info("deleted the Gmail OAuth file, which email no longer uses", "path", path)
}
