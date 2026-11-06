// Package applecerts provides the public Apple roots used for offline iOS
// signing-input validation. These are public certificates, never private keys.
package applecerts

//go:generate go run generate.go

import "crypto/x509"

func Roots() (*x509.CertPool, error) {
	p := x509.NewCertPool()
	for _, der := range certificates {
		cert, err := x509.ParseCertificate([]byte(der))
		if err != nil {
			return nil, err
		}
		p.AddCert(cert)
	}
	return p, nil
}
