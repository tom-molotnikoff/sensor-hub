package appProps

import (
	"example/sensorHub/utils"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var databaseProperties map[string]string

var configDir = "configuration"
var applicationPropertiesFilePath string
var databasePropertiesFilePath string

func init() {
	setConfigPaths(configDir)
}

func setConfigPaths(dir string) {
	configDir = dir
	applicationPropertiesFilePath = filepath.Join(dir, "application.properties")
	databasePropertiesFilePath = filepath.Join(dir, "database.properties")
}

func GetConfigDir() string {
	return configDir
}

func ReadApplicationPropertiesFile() (map[string]string, error) {
	applicationProperties, _ := BuildDefaults()
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
	_, dbDefaults := BuildDefaults()
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
		databasePropertiesFilePath,
	}
}

// Deprecated: Use [os.Stat] directly. Kept only for test compatibility.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
