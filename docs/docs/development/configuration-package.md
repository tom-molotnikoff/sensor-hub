---
id: configuration-package
title: Configuration Package
sidebar_position: 7
---

# Configuration Package

The `application_properties` package manages all runtime configuration for
Sensor Hub. It loads property files from disk, provides typed access via a
struct, and watches for external changes. Every configurable setting is
declared as a single annotated field on the `ApplicationConfiguration` struct —
that struct is the only place you need to edit to add a new property.

## Adding a New Configuration Property

To add a new property you only need to edit **one place** — the
`ApplicationConfiguration` struct:

```go
type ApplicationConfiguration struct {
    // ... existing fields ...

    MyNewSetting int `prop:"my.new.setting" default:"42" file:"application" validate:"positive"`
}
```

The struct tags define the behaviour for this new property:

- **Default value** — written to the property file if absent on first load
- **Loading** — parsed from the correct file and typed into the struct field
- **Saving** — serialised back to the correct file when the API writes changes
- **Logging** — printed on startup and reload
- **Validation** — checked during loading according to the `validate` tag

### Struct Tags Reference

| Tag         | Required | Values                                        | Purpose                                               |
|-------------|----------|-----------------------------------------------|-------------------------------------------------------|
| `prop`      | yes      | Dotted key, e.g. `sensor.collection.interval` | Property key in the `.properties` file                |
| `default`   | yes      | String representation of the default value    | Written to file if absent; used as fallback           |
| `file`      | yes      | `application`, `smtp`, or `database`          | Which property file this setting belongs to           |
| `validate`  | no       | `positive`, `non_negative`, or `non_empty` | Per-field validation applied during loading          |

### Cross-Field Validation

Per-field rules (like `positive`) are handled automatically. If
you need a rule that has more complex validation logic, add it to
`validateApplicationProperties()` in `application_properties_files.go`. An
example is the check that `openapi.yaml.location` must be non-empty when
`sensor.discovery.skip` is `false`.

### Post-Processing Hooks

Some fields need transformation after loading that doesn't fit a tag (e.g.
resolving relative file paths against the config directory). Add these cases
to `postProcessConfig()`.

## File Watcher

The file watcher polls all property files for modification-time changes. When
an external edit is detected, the files are re-read and the configuration is
reloaded automatically. Writes made by the API are coordinated with the watcher
to avoid reload loops.
