package service

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

// ParseBrokerCA reads the CA certificates an external broker's certificate is
// verified against, pasted as PEM. Text around the PEM blocks is ignored, as
// openssl prints a certificate's details above it. Every block must be a
// certificate that parses, so a pasted private key or a damaged certificate
// is refused rather than skipped, and there must be at least one.
func ParseBrokerCA(pemText string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	rest := []byte(pemText)
	count := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		count++
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("broker CA certificate contains a %s block (block %d); paste only the CA certificates and remove it", block.Type, count)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("broker CA certificate %d does not parse (%v); check it was pasted whole", count, err)
		}
		pool.AddCert(cert)
	}
	if count == 0 {
		return nil, fmt.Errorf("broker CA certificate holds no PEM certificate; paste the CA in PEM form, from -----BEGIN CERTIFICATE----- to -----END CERTIFICATE-----")
	}
	return pool, nil
}

// brokerCAGiven reports whether the broker carries a CA to verify against.
func brokerCAGiven(pemText *string) bool {
	return pemText != nil && strings.TrimSpace(*pemText) != ""
}
