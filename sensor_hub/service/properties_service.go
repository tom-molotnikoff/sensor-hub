package service

import (
	"context"

	appProps "example/sensorHub/application_properties"
	"example/sensorHub/ws"
	"log/slog"
	"sync"
)

type PropertiesService struct {
	logger     *slog.Logger
	background sync.WaitGroup
}

func NewPropertiesService(logger *slog.Logger) *PropertiesService {
	return &PropertiesService{logger: logger.With("component", "properties_service")}
}

// inBackground runs work the caller is deliberately not made to wait for: a
// save has already taken effect in memory by the time it starts, so the file
// write and the broadcast follow behind it. The service tracks them only so
// that tests can join the work rather than guess at how long it takes.
// Nothing in production waits, and nothing should: a join running alongside
// an in-flight save would race the counter back up from zero.
func (ps *PropertiesService) inBackground(work func()) {
	ps.background.Add(1)
	go func() {
		defer ps.background.Done()
		work()
	}()
}

// waitForBackgroundWork blocks until the work started by every
// [PropertiesService.ServiceUpdateProperties] call so far has finished.
func (ps *PropertiesService) waitForBackgroundWork() {
	ps.background.Wait()
}

func (ps *PropertiesService) ServiceUpdateProperties(ctx context.Context, properties map[string]string) error {
	appProperties, smtpProperties, dbProperties := appProps.ConvertConfigurationToMaps(appProps.AppConfig())

	for key, value := range properties {
		if _, ok := appProperties[key]; ok {
			appProperties[key] = value
		} else if _, ok := dbProperties[key]; ok {
			dbProperties[key] = value
		} else if _, ok := smtpProperties[key]; ok {
			smtpProperties[key] = value
		}
	}

	// Pre-flight validation: LoadConfigurationFromMaps surfaces parse/validation
	// errors so the API can return them to the caller. ReloadConfig's error is
	// ignored below because the same maps were just validated here. Running
	// both is safe — the load step no longer mutates the configuration
	// (issue #44).
	if _, err := appProps.LoadConfigurationFromMaps(appProperties, smtpProperties, dbProperties); err != nil {
		return err
	}

	appProps.ReloadConfig(appProperties, smtpProperties, dbProperties)

	ps.inBackground(func() {
		err := appProps.SaveConfigurationToFiles()
		if err != nil {
			ps.logger.Error("error saving configuration to files", "error", err)
		}
	})

	ps.inBackground(func() {
		ps.BroadcastProperties(context.Background())
	})

	return nil
}

// BroadcastProperties sends the current property values to every properties
// websocket subscriber. It runs after a PATCH and after the config watcher
// reloads an external file edit, so both paths broadcast identically.
func (ps *PropertiesService) BroadcastProperties(ctx context.Context) {
	properties, err := ps.ServiceGetProperties(ctx)
	if err != nil {
		ps.logger.Error("error fetching updated properties for broadcast", "error", err)
		return
	}
	ws.BroadcastToTopic("properties", properties)
}

func (ps *PropertiesService) ServiceGetProperties(ctx context.Context) (map[string]interface{}, error) {
	propertiesMap := make(map[string]interface{})

	appProperties, smtpProperties, dbProperties := appProps.ConvertConfigurationToMaps(appProps.AppConfig())

	for key, value := range appProperties {
		propertiesMap[key] = value
	}
	for key, value := range dbProperties {
		propertiesMap[key] = value
	}
	for key, value := range smtpProperties {
		propertiesMap[key] = value
	}

	return propertiesMap, nil
}
