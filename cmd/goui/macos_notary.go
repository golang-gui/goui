package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/encoding/httpbinding"
)

const notaryAPIURL = "https://appstoreconnect.apple.com/notary/v2"

var notaryIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var notaryKeyPattern = regexp.MustCompile(`^[A-Za-z0-9]{2,64}$`)
var notaryBucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type notaryClient struct {
	http            *http.Client
	endpoint        string
	key             *ecdsa.PrivateKey
	keyID, issuerID string
	pollInterval    time.Duration
}

type notarySubmission struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			AccessKey string `json:"awsAccessKeyId"`
			SecretKey string `json:"awsSecretAccessKey"`
			Token     string `json:"awsSessionToken"`
			Bucket    string `json:"bucket"`
			Object    string `json:"object"`
		} `json:"attributes"`
	} `json:"data"`
}

type notaryReceipt struct {
	Format   int             `json:"format"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	SHA256   string          `json:"sha256,omitempty"`
	CDHash   string          `json:"cdhash,omitempty"`
	Uploaded bool            `json:"uploaded"`
	Status   string          `json:"status"`
	Log      json.RawMessage `json:"log,omitempty"`
}

func newNotaryClient(c *config) (*notaryClient, error) {
	cfg := c.MacOS.Notary
	if !notaryKeyPattern.MatchString(cfg.KeyID) || !notaryIDPattern.MatchString(cfg.IssuerID) || cfg.PrivateKey == "" {
		return nil, fmt.Errorf("macos.notary requires private_key, key_id and issuer_id for an App Store Connect team API key")
	}
	raw, err := os.ReadFile(c.path(cfg.PrivateKey))
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf("Notary API private_key must be one PKCS#8 PEM key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid Notary API private key")
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("Notary API requires an ECDSA P-256 private key")
	}
	return &notaryClient{http: appleHTTPClient(), endpoint: notaryAPIURL, key: key, keyID: cfg.KeyID, issuerID: cfg.IssuerID, pollInterval: 2 * time.Second}, nil
}

func (c *notaryClient) token() (string, error) {
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "JWT", "kid": c.keyID})
	claims, _ := json.Marshal(map[string]any{"iss": c.issuerID, "iat": now, "exp": now + 600, "aud": "appstoreconnect-v1"})
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	sum := sha256.Sum256([]byte(encoded))
	r, s, err := ecdsa.Sign(rand.Reader, c.key, sum[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (c *notaryClient) request(ctx context.Context, method, path string, body any, response any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	token, err := c.token()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Notary API request: %w", err)
	}
	data, err = readAppleResponse(resp, 2<<20)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, response); err != nil {
		return fmt.Errorf("decode Notary API response: %w", err)
	}
	return nil
}

func (c *notaryClient) submit(ctx context.Context, name, hash string) (notarySubmission, error) {
	var reply notarySubmission
	err := c.request(ctx, http.MethodPost, "/submissions", map[string]string{"submissionName": name, "sha256": hash}, &reply)
	if err == nil && !notaryIDPattern.MatchString(reply.Data.ID) {
		err = fmt.Errorf("Notary API returned an invalid submission ID")
	}
	return reply, err
}

func (c *notaryClient) upload(ctx context.Context, submission notarySubmission, path, payloadHash string) error {
	info := submission.Data.Attributes
	if info.AccessKey == "" || info.SecretKey == "" || info.Token == "" || info.Object == "" || !notaryBucketPattern.MatchString(info.Bucket) {
		return fmt.Errorf("Notary API returned invalid temporary upload credentials")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	endpoint := url.URL{Scheme: "https", Host: info.Bucket + ".s3.us-west-2.amazonaws.com", Path: "/" + info.Object}
	// S3 canonical URIs encode reserved bytes such as ':' and '+', while Go's
	// default URL path escaping preserves them. Use the same escaped path on
	// the wire and in SigV4; signing must not apply the encoding a second time.
	endpoint.RawPath = httpbinding.EscapePath(endpoint.Path, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint.String(), file)
	if err != nil {
		return err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	credentials := aws.Credentials{AccessKeyID: info.AccessKey, SecretAccessKey: info.SecretKey, SessionToken: info.Token}
	if err = v4.NewSigner().SignHTTP(ctx, credentials, req, payloadHash, "s3", "us-west-2", time.Now(), func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true }); err != nil {
		return fmt.Errorf("sign notarization upload: %w", err)
	}
	client := *c.http
	client.Timeout = 0 // The operation's context bounds large uploads.
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("notarization upload transport failed")
	}
	_, err = readAppleResponse(resp, 64<<10)
	return err
}

func (c *notaryClient) status(ctx context.Context, id string) (string, error) {
	if !notaryIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid notarization submission ID")
	}
	var reply struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Status string `json:"status"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, "/submissions/"+id, nil, &reply); err != nil {
		return "", err
	}
	if reply.Data.ID != id {
		return "", fmt.Errorf("Notary API returned a different submission ID")
	}
	switch reply.Data.Attributes.Status {
	case "In Progress", "Accepted", "Invalid", "Rejected":
		return reply.Data.Attributes.Status, nil
	}
	return "", fmt.Errorf("Notary API returned an unknown submission status")
}

func (c *notaryClient) log(ctx context.Context, id string) (json.RawMessage, error) {
	var reply struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				URL string `json:"developerLogUrl"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, "/submissions/"+id+"/logs", nil, &reply); err != nil {
		return nil, err
	}
	u, err := url.Parse(reply.Data.Attributes.URL)
	if err != nil || reply.Data.ID != id || u == nil || u.Scheme != "https" || u.User != nil || !strings.HasSuffix(u.Hostname(), ".amazonaws.com") {
		return nil, fmt.Errorf("Notary API returned an invalid log location")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid notarization log request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("notarization log download transport failed")
	}
	data, err := readAppleResponse(resp, 16<<20)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("notarization log is not JSON")
	}
	return data, nil
}

func (c *notaryClient) poll(ctx context.Context, receipt *notaryReceipt, wait bool, save func() error) error {
	for {
		status, err := c.status(ctx, receipt.ID)
		if err != nil {
			return err
		}
		receipt.Status = status
		if err = save(); err != nil {
			return err
		}
		if status != "In Progress" {
			receipt.Log, err = c.log(ctx, receipt.ID)
			if err != nil {
				return err
			}
			if err = save(); err != nil {
				return err
			}
			if status != "Accepted" {
				return fmt.Errorf("notarization %s: %s; inspect the saved log", receipt.ID, status)
			}
			return nil
		}
		if !wait {
			return nil
		}
		interval := c.pollInterval
		if interval <= 0 {
			interval = 2 * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("waiting for notarization %s: %w", receipt.ID, ctx.Err())
		case <-timer.C:
		}
	}
}

func saveNotaryReceipt(path string, receipt *notaryReceipt, create bool) error {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if create {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("notarization receipt must be a regular file")
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var old notaryReceipt
	if err = json.Unmarshal(existing, &old); err != nil {
		return err
	}
	if old.ID != "" && old.ID != receipt.ID {
		return fmt.Errorf("notarization receipt belongs to another submission")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".goui-notary-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
