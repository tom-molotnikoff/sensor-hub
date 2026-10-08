package appProps

import (
	"net"
	"strconv"
	"strings"
)

// MQTTBrokerAddress returns the address the embedded MQTT broker binds:
// mqtt.broker.listen.address and mqtt.broker.port joined as host:port.
func (cfg *ApplicationConfiguration) MQTTBrokerAddress() string {
	return net.JoinHostPort(cfg.MQTTBrokerListenAddress, strconv.Itoa(cfg.MQTTBrokerPort))
}

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

// isListenHost accepts the host part of a listen address: an IP address, or a
// name such as localhost. It refuses a port, and refuses empty, which would
// bind every interface without anyone having asked for it.
func isListenHost(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
