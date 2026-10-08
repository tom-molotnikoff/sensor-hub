package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func caPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "home CA"},
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestParseBrokerCA_AcceptsCertificatesWithTextAroundThem(t *testing.T) {
	ca := caPEM(t)

	_, err := ParseBrokerCA("Certificate:\n    Subject: CN = home CA\n" + ca + "\n" + caPEM(t))

	assert.NoError(t, err)
}

func TestParseBrokerCA_RefusesAnythingButCertificates(t *testing.T) {
	ca := caPEM(t)
	cases := map[string]string{
		"no PEM at all":         "not a certificate",
		"a private key":         ca + string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}})),
		"a damaged certificate": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("damaged")})),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseBrokerCA(input)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "broker CA certificate", "the API reports it as a validation error")
		})
	}
}
