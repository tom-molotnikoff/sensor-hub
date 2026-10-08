package mqtt

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	gen "example/sensorHub/gen"
	"example/sensorHub/service"
)

// externalBrokerURL is the address Paho dials for an external broker, and the
// TLS settings it dials with. With TLS off it is plain TCP and the config is
// nil. With TLS on the broker's certificate must chain to the pasted CA, or to
// the system roots when none is pasted, and be issued for the broker's host;
// anything else fails the handshake before a credential is sent.
func externalBrokerURL(broker gen.MQTTBroker, host string, port int) (string, *tls.Config, error) {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	if broker.Tls == nil || !*broker.Tls {
		return "tcp://" + address, nil, nil
	}
	config := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if broker.CaCertPem != nil && *broker.CaCertPem != "" {
		pool, err := service.ParseBrokerCA(*broker.CaCertPem)
		if err != nil {
			return "", nil, fmt.Errorf("broker %s: %w", broker.Name, err)
		}
		config.RootCAs = pool
	}
	return "ssl://" + address, config, nil
}
