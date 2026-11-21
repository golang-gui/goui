package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/digitorus/timestamp"
	"go.mozilla.org/pkcs7"
)

const appleTimestampURL = "http://timestamp.apple.com/ts01"

func appleHTTPClient() *http.Client {
	return &http.Client{Timeout: time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func readAppleResponse(resp *http.Response, limit int64) ([]byte, error) {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Apple service returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Apple service response exceeds %d bytes", limit)
	}
	return data, nil
}

func requestAppleTimestamp(ctx context.Context, client *http.Client, endpoint string, roots *x509.CertPool, signature []byte) ([]byte, error) {
	nonce, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	body, err := timestamp.CreateRequest(bytes.NewReader(signature), &timestamp.RequestOptions{Hash: crypto.SHA256, Certificates: true, Nonce: nonce})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/timestamp-query")
	req.Header.Set("Accept", "application/timestamp-reply")
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Apple timestamp: %w", err)
	}
	raw, err := readAppleResponse(resp, 2<<20)
	if err != nil {
		return nil, err
	}
	stamp, err := timestamp.ParseResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse Apple timestamp: %w", err)
	}
	if stamp.Nonce == nil || stamp.Nonce.Cmp(nonce) != 0 || stamp.Time.Before(started.Add(-5*time.Minute)) {
		return nil, fmt.Errorf("timestamp nonce or issuance time does not match request")
	}
	if _, err = verifyAppleTimestamp(stamp.RawToken, signature, roots); err != nil {
		return nil, err
	}
	return stamp.RawToken, nil
}

func verifyAppleTimestamp(raw, signature []byte, roots *x509.CertPool) (time.Time, error) {
	stamp, err := timestamp.Parse(raw)
	if err != nil {
		return time.Time{}, err
	}
	hash := sha256.Sum256(signature)
	if stamp.HashAlgorithm != crypto.SHA256 || !bytes.Equal(stamp.HashedMessage, hash[:]) || stamp.Time.After(time.Now().Add(5*time.Minute)) {
		return time.Time{}, fmt.Errorf("invalid timestamp digest or issuance time")
	}
	cms, err := pkcs7.Parse(raw)
	if err != nil {
		return time.Time{}, err
	}
	signer := cms.GetOnlySigner()
	if signer == nil || signer.IsCA || signer.KeyUsage&x509.KeyUsageDigitalSignature == 0 || len(signer.ExtKeyUsage) != 1 || signer.ExtKeyUsage[0] != x509.ExtKeyUsageTimeStamping {
		return time.Time{}, fmt.Errorf("timestamp requires one TSA signer")
	}
	criticalUsage := false
	for _, extension := range signer.Extensions {
		if extension.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 37}) && extension.Critical {
			criticalUsage = true
		}
	}
	if !criticalUsage {
		return time.Time{}, fmt.Errorf("timestamp signer requires a critical time-stamping EKU")
	}
	intermediates := x509.NewCertPool()
	for _, cert := range cms.Certificates {
		if !cert.Equal(signer) {
			intermediates.AddCert(cert)
		}
	}
	if roots == nil {
		return time.Time{}, fmt.Errorf("timestamp requires explicit Apple trust roots")
	}
	if _, err = signer.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, CurrentTime: stamp.Time, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}}); err != nil {
		return time.Time{}, fmt.Errorf("verify timestamp certificate: %w", err)
	}
	return stamp.Time, nil
}

func appleCMSTimestamp(cms *pkcs7.PKCS7) ([]byte, error) {
	if len(cms.Signers) != 1 {
		return nil, fmt.Errorf("expected one CMS signer")
	}
	var token []byte
	for _, a := range cms.Signers[0].UnauthenticatedAttributes {
		if a.Type.Equal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}) {
			if token != nil {
				return nil, fmt.Errorf("duplicate secure timestamp")
			}
			var raw asn1.RawValue
			rest, err := asn1.Unmarshal(a.Value.Bytes, &raw)
			if err != nil || len(rest) != 0 || raw.Tag != asn1.TagSequence {
				return nil, fmt.Errorf("invalid secure timestamp attribute")
			}
			token = raw.FullBytes
		}
	}
	if token == nil {
		return nil, fmt.Errorf("missing secure timestamp")
	}
	return token, nil
}
