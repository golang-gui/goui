package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/agusibrahim/apksig-go/pkg/algo"
	"github.com/agusibrahim/apksig-go/pkg/apkverifier"
	"github.com/agusibrahim/apksig-go/pkg/apkwriter"
	"github.com/agusibrahim/apksig-go/pkg/datasource"
	"github.com/agusibrahim/apksig-go/pkg/signer"
	"software.sslmate.com/src/go-pkcs12"
)

func mobileIdentity(c *config, config mobileSigning) (crypto.PrivateKey, []*x509.Certificate, error) {
	if config.PKCS12 == "" {
		return nil, nil, fmt.Errorf("signing requires pkcs12")
	}
	password := ""
	if config.PasswordEnv != "" {
		var ok bool
		password, ok = os.LookupEnv(config.PasswordEnv)
		if !ok {
			return nil, nil, fmt.Errorf("signing password environment variable %q is unset", config.PasswordEnv)
		}
	}
	data, err := os.ReadFile(c.path(config.PKCS12))
	if err != nil {
		return nil, nil, err
	}
	key, cert, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return nil, nil, fmt.Errorf("decode signing identity: %w", err)
	}
	return key, append([]*x509.Certificate{cert}, chain...), nil
}

func signAndroid(ctx context.Context, c *config, input, output string, log io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var key crypto.PrivateKey
	var certs []*x509.Certificate
	var err error
	development := c.Android.Signing.PKCS12 == ""
	if development {
		keyPath, certPath, err := androidDevelopmentKey()
		if err != nil {
			return err
		}
		identity, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return err
		}
		key = identity.PrivateKey
		for _, der := range identity.Certificate {
			cert, err := x509.ParseCertificate(der)
			if err != nil {
				return err
			}
			certs = append(certs, cert)
		}
	} else {
		key, certs, err = mobileIdentity(c, c.Android.Signing)
		if err != nil {
			return err
		}
	}
	if len(certs) == 0 || time.Now().Before(certs[0].NotBefore) || time.Now().After(certs[0].NotAfter) {
		return fmt.Errorf("Android signing certificate is missing or outside its validity period")
	}
	var algorithm algo.SigID
	switch k := key.(type) {
	case *rsa.PrivateKey:
		if k.N.BitLen() < 2048 {
			return fmt.Errorf("Android signing requires RSA keys of at least 2048 bits")
		}
		algorithm = algo.SigRSAPKCS1SHA256
	case *ecdsa.PrivateKey:
		algorithm = algo.SigECDSASHA256
	default:
		return fmt.Errorf("unsupported Android signing key type %T", key)
	}
	pub, err := x509.MarshalPKIXPublicKey(key.(crypto.Signer).Public())
	if err != nil || !bytes.Equal(pub, certs[0].RawSubjectPublicKeyInfo) {
		return fmt.Errorf("Android signing key does not match its certificate")
	}
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	a, _ := algo.ByID(algorithm)
	w := apkwriter.SignedAPKWriter{
		Src:      datasource.NewReaderAt(f, info.Size()),
		Signers:  []*signer.SignerConfig{{PrivateKey: key, Certs: certs, Algorithms: []algo.Algorithm{a}}},
		V3MinSdk: int32(c.Android.MinSDK), V3MaxSdk: 0x7fffffff, Align: true,
	}
	dest, err := os.Create(output)
	if err != nil {
		return err
	}
	if err = w.Write(dest); err != nil {
		dest.Close()
		return err
	}
	if err = dest.Close(); err != nil {
		return err
	}
	signed, err := os.Open(output)
	if err != nil {
		return err
	}
	defer signed.Close()
	info, err = signed.Stat()
	if err != nil {
		return err
	}
	result, err := apkverifier.Verify(datasource.NewReaderAt(signed, info.Size()), c.Android.MinSDK, c.Android.TargetSDK)
	if err != nil {
		return fmt.Errorf("verify signed APK: %w", err)
	}
	if !result.V2Verified || !result.V3Verified || len(result.SignerCerts) == 0 || !bytes.Equal(result.SignerCerts[0], certs[0].Raw) {
		return fmt.Errorf("APK signing verification failed: %v", result.Errors)
	}
	if development && log != nil {
		fmt.Fprintln(log, "Android APK uses a persistent local development signing key.")
	}
	return ctx.Err()
}
