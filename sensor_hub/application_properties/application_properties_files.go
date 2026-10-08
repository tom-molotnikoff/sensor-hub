package appProps

import (
	"example/sensorHub/utils"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var smtpProperties map[string]string
var databaseProperties map[string]string

var configDir = "configuration"
var applicationPropertiesFilePath string
var smtpPropertiesFilePath string
var databasePropertiesFilePath string

func init() {
	setConfigPaths(configDir)
}

func setConfigPaths(dir string) {
	configDir = dir
	applicationPropertiesFilePath = filepath.Join(dir, "application.properties")
	smtpPropertiesFilePath = filepath.Join(dir, "smtp.properties")
	databasePropertiesFilePath = filepath.Join(dir, "database.properties")
}

func GetConfigDir() string {
	return configDir
}

func validateSMTPProperties() error {
	if smtpProperties["smtp.user"] == "" {
		slog.Warn("smtp.user is empty, email alerts will not be sent; check smtp.properties file")
	}
	return nil
}

func ReadApplicationPropertiesFile() (map[string]string, error) {
	applicationProperties, _, _ := BuildDefaults()
	propertiesFromFile, err := utils.ReadPropertiesFile(applicationPropertiesFilePath)

	if err != nil {
		return nil, fmt.Errorf("failed to read application properties file: %w", err)
	}

	for k, v := range propertiesFromFile {
		applicationProperties[k] = v
	}
	clampToBounds("application", applicationProperties)

	return applicationProperties, nil
}

func ReadDatabasePropertiesFile() (map[string]string, error) {
	_, _, dbDefaults := BuildDefaults()
	databaseProperties = dbDefaults
	propertiesFromFile, err := utils.ReadPropertiesFile(databasePropertiesFilePath)

	if err != nil {
		return nil, fmt.Errorf("failed to read database properties file: %w", err)
	}

	for k, v := range propertiesFromFile {
		databaseProperties[k] = v
	}
	clampToBounds("database", databaseProperties)

	return databaseProperties, nil
}

func ReadSMTPPropertiesFile() (map[string]string, error) {
	_, smtpDefaults, _ := BuildDefaults()
	smtpProperties = smtpDefaults
	propertiesFromFile, err := utils.ReadPropertiesFile(smtpPropertiesFilePath)

	if err != nil {
		return nil, fmt.Errorf("failed to read SMTP properties file: %w", err)
	}

	for k, v := range propertiesFromFile {
		smtpProperties[k] = v
	}

	if err := validateSMTPProperties(); err != nil {
		return nil, fmt.Errorf("validation failed on SMTP properties: %w", err)
	}

	return smtpProperties, nil
}

func SaveConfigurationToFiles() error {
	if AppConfig() == nil {
		slog.Warn("no application configuration loaded; cannot save")
		return fmt.Errorf("no application configuration loaded; cannot save")
	}

	markWriteInProgress()
	defer clearWriteInProgress()

	return SaveToFiles(AppConfig())
}

// ConfigFilePaths returns the paths of all property files.
func ConfigFilePaths() []string {
	return []string{
		applicationPropertiesFilePath,
		smtpPropertiesFilePath,
		databasePropertiesFilePath,
	}
}

// Deprecated: Use [os.Stat] directly. Kept only for test compatibility.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
