package appProps

import (
	"os"
	"strings"
	"time"
	_ "time/tzdata" // hub.timezone must load on hosts without /usr/share/zoneinfo
)

// hostDefaults supplies the default for a property whose default depends on
// the machine it runs on. They apply only where the default tag is empty.
var hostDefaults = map[string]func() string{
	"hub.timezone": serverTimezone,
}

// serverTimezone names the server's zone so a fresh install schedules in local
// time. time.Local cannot be used for this: it reports "Local", not an IANA
// name, and the property has to hold a name that loads anywhere.
func serverTimezone() string {
	candidates := []string{strings.TrimPrefix(os.Getenv("TZ"), ":")}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		candidates = append(candidates, zoneFromLocaltimeLink(target))
	}
	if contents, err := os.ReadFile("/etc/timezone"); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(contents)))
	}
	for _, candidate := range candidates {
		if isIANAZone(candidate) {
			return candidate
		}
	}
	return "UTC"
}

// zoneFromLocaltimeLink reads the zone name out of an /etc/localtime symlink
// target such as /usr/share/zoneinfo/Europe/London.
func zoneFromLocaltimeLink(target string) string {
	_, name, found := strings.Cut(target, "zoneinfo/")
	if !found {
		return ""
	}
	return name
}

func isIANAZone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// HubLocation is the zone automation schedules run in. A config that passed
// validation always loads, so the UTC fallback only covers a config built by
// hand in a test.
func (cfg *ApplicationConfiguration) HubLocation() *time.Location {
	location, err := time.LoadLocation(cfg.HubTimezone)
	if err != nil {
		return time.UTC
	}
	return location
}
