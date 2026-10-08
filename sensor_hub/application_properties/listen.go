package appProps

import (
	"net"
	"strconv"
	"strings"
)

// TrustedProxies returns the addresses and CIDR ranges in http.trusted.proxies.
func (cfg *ApplicationConfiguration) TrustedProxies() []string {
	return splitList(cfg.HTTPTrustedProxies)
}

func splitList(s string) []string {
	var items []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func isIPOrCIDRList(s string) bool {
	for _, item := range splitList(s) {
		if net.ParseIP(item) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(item); err != nil {
			return false
		}
	}
	return true
}

// isListenAddress accepts host:port with a port a server can be told to bind.
// The host may be empty, meaning every interface.
func isListenAddress(s string) bool {
	_, port, err := net.SplitHostPort(s)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}
