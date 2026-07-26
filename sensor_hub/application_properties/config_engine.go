package appProps

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

// PropertyDef holds metadata for a single configuration property, derived from struct tags.
type PropertyDef struct {
	FieldName  string // Go struct field name, e.g. "SensorCollectionInterval"
	FieldIndex int    // index in the struct for reflect access
	Key        string // dotted property key, e.g. "sensor.collection.interval"
	Kind       reflect.Kind
	Default    string // default value from `default` tag
	File       string // "application", "smtp", or "database"
	Validate   string // "positive", "non_negative", or ""
}

var registry []PropertyDef

func init() {
	registry = buildRegistry()
}

func buildRegistry() []PropertyDef {
	t := reflect.TypeOf(ApplicationConfiguration{})
	defs := make([]PropertyDef, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		key := field.Tag.Get("prop")
		if key == "" {
			continue
		}

		defs = append(defs, PropertyDef{
			FieldName:  field.Name,
			FieldIndex: i,
			Key:        key,
			Kind:       field.Type.Kind(),
			Default:    field.Tag.Get("default"),
			File:       field.Tag.Get("file"),
			Validate:   field.Tag.Get("validate"),
		})
	}

	return defs
}

// BuildDefaults returns default property maps grouped by file tag.
func BuildDefaults() (application map[string]string, smtp map[string]string, database map[string]string) {
	application = make(map[string]string)
	smtp = make(map[string]string)
	database = make(map[string]string)

	for _, def := range registry {
		switch def.File {
		case "application":
			application[def.Key] = def.Default
		case "smtp":
			smtp[def.Key] = def.Default
		case "database":
			database[def.Key] = def.Default
		}
	}

	return application, smtp, database
}

// mapForFile selects the correct map based on the file tag.
func mapForFile(file string, app, smtp, db map[string]string) map[string]string {
	switch file {
	case "application":
		return app
	case "smtp":
		return smtp
	case "database":
		return db
	default:
		return nil
	}
}

// LoadFromMaps parses three string maps into a typed ApplicationConfiguration.
func LoadFromMaps(appProps, smtpProps, dbProps map[string]string) (*ApplicationConfiguration, error) {
	cfg := &ApplicationConfiguration{}
	val := reflect.ValueOf(cfg).Elem()

	for _, def := range registry {
		m := mapForFile(def.File, appProps, smtpProps, dbProps)
		if m == nil {
			continue
		}

		raw, ok := m[def.Key]
		if !ok {
			continue
		}

		field := val.Field(def.FieldIndex)

		switch def.Kind {
		case reflect.Int:
			if raw == "" {
				continue
			}
			i, err := strconv.Atoi(raw)
			if err != nil {
				slog.Error("invalid config value", "key", def.Key, "value", raw, "error", err)
				return nil, err
			}
			if err := validateInt(def, i, raw); err != nil {
				return nil, err
			}
			field.SetInt(int64(i))

		case reflect.Bool:
			if raw == "" {
				continue
			}
			b, err := strconv.ParseBool(raw)
			if err != nil {
				slog.Error("invalid config value", "key", def.Key, "value", raw, "error", err)
				return nil, err
			}
			field.SetBool(b)

		case reflect.String:
			if err := validateString(def, raw); err != nil {
				return nil, err
			}
			field.SetString(raw)
		}
	}

	return cfg, nil
}

func validateInt(def PropertyDef, value int, raw string) error {
	switch def.Validate {
	case "positive":
		if value <= 0 {
			return fmt.Errorf("invalid %s value: %s", def.Key, raw)
		}
	case "non_negative":
		if value < 0 {
			return fmt.Errorf("invalid %s value: %s", def.Key, raw)
		}
	}
	return nil
}

func validateString(def PropertyDef, value string) error {
	switch def.Validate {
	case "non_empty":
		if value == "" {
			return fmt.Errorf("%s must not be empty", def.Key)
		}
	}
	return nil
}

// ConvertToMaps serialises an ApplicationConfiguration into three string maps.
func ConvertToMaps(cfg *ApplicationConfiguration) (application map[string]string, smtp map[string]string, database map[string]string) {
	application = make(map[string]string)
	smtp = make(map[string]string)
	database = make(map[string]string)

	val := reflect.ValueOf(cfg).Elem()

	for _, def := range registry {
		field := val.Field(def.FieldIndex)

		var s string
		switch def.Kind {
		case reflect.Int:
			s = strconv.Itoa(int(field.Int()))
		case reflect.Bool:
			s = strconv.FormatBool(field.Bool())
		case reflect.String:
			s = field.String()
		}

		switch def.File {
		case "application":
			application[def.Key] = s
		case "smtp":
			smtp[def.Key] = s
		case "database":
			database[def.Key] = s
		}
	}

	return application, smtp, database
}

// LogConfig logs all configuration values.
func LogConfig(cfg *ApplicationConfiguration) {
	val := reflect.ValueOf(cfg).Elem()
	args := make([]any, 0, len(registry)*2)

	for _, def := range registry {
		field := val.Field(def.FieldIndex)
		args = append(args, toSnakeCase(def.FieldName), field.Interface())
	}

	slog.Info("configuration reloaded", args...)
}

// SaveToFiles writes the current configuration to the property files.
func SaveToFiles(cfg *ApplicationConfiguration) error {
	appMap, smtpMap, dbMap := ConvertToMaps(cfg)

	type fileEntry struct {
		path string
		m    map[string]string
	}

	files := []fileEntry{
		{applicationPropertiesFilePath, appMap},
		{smtpPropertiesFilePath, smtpMap},
		{databasePropertiesFilePath, dbMap},
	}

	// Collect ordered keys per file from the registry so output order is deterministic.
	orderedKeys := make(map[string][]string) // file tag → ordered keys
	for _, def := range registry {
		orderedKeys[def.File] = append(orderedKeys[def.File], def.Key)
	}

	fileTagForPath := map[string]string{
		applicationPropertiesFilePath: "application",
		smtpPropertiesFilePath:        "smtp",
		databasePropertiesFilePath:    "database",
	}

	for _, fe := range files {
		f, err := os.OpenFile(fe.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		tag := fileTagForPath[fe.path]
		for _, key := range orderedKeys[tag] {
			if _, werr := f.WriteString(key + "=" + fe.m[key] + "\n"); werr != nil {
				f.Close()
				return werr
			}
		}

		if cerr := f.Close(); cerr != nil {
			slog.Error("error closing properties file", "path", fe.path, "error", cerr)
		}
	}

	return nil
}

// toSnakeCase converts a PascalCase field name to snake_case for log keys.
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// writeInProgress is set while SaveConfigurationToFiles is writing to disk.
// The file watcher checks this to avoid reloading changes the app itself wrote.
var writeInProgress atomic.Bool

// lastWriteCompletedAt stores the UnixNano timestamp of the most recent write
// completion. The watcher uses this for cooldown even if the write finished
// between ticks (i.e. the watcher never saw writeInProgress == true).
var lastWriteCompletedAt atomic.Int64

func markWriteInProgress() {
	writeInProgress.Store(true)
}

func clearWriteInProgress() {
	lastWriteCompletedAt.Store(time.Now().UnixNano())
	writeInProgress.Store(false)
}

// IsWriteInProgress reports whether the application is currently writing config files.
func IsWriteInProgress() bool {
	return writeInProgress.Load()
}

// LastWriteCompletedAt returns the time the last write finished.
// Returns zero time if no write has ever completed.
func LastWriteCompletedAt() time.Time {
	ns := lastWriteCompletedAt.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
