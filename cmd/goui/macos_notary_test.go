package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type notaryTestTransport func(*http.Request) (*http.Response, error)

func (f notaryTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNotaryProtocol(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const id = "11111111-2222-3333-4444-555555555555"
	const issuer = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	var statusCalls atomic.Int32
	terminal := "Accepted"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
		if len(parts) != 3 {
			t.Error("缺少 JWT")
			w.WriteHeader(401)
			return
		}
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err != nil || len(signature) != 64 || !ecdsa.Verify(&key.PublicKey, sum[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
			t.Error("JWT ES256 签名不正确")
			w.WriteHeader(401)
			return
		}
		claimsRaw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims struct {
			Issuer   string `json:"iss"`
			Audience string `json:"aud"`
			Expiry   int64  `json:"exp"`
			Issued   int64  `json:"iat"`
		}
		_ = json.Unmarshal(claimsRaw, &claims)
		if claims.Issuer != issuer || claims.Audience != "appstoreconnect-v1" || claims.Expiry-claims.Issued != 600 || claims.Issued > time.Now().Unix()+1 {
			t.Error("JWT 身份或有效期不正确")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/submissions":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["submissionName"] != "Test.app.zip" || body["sha256"] != strings.Repeat("a", 64) {
				t.Error("上传名称或摘要不正确")
			}
			_, _ = io.WriteString(w, `{"data":{"id":"`+id+`","attributes":{"awsAccessKeyId":"UPLOAD_ID","awsSecretAccessKey":"UPLOAD_SECRET","awsSessionToken":"UPLOAD_TOKEN","bucket":"notary-test-bucket","object":"prod/role:session+ space/%2F/文件.zip"}}}`)
		case r.URL.Path == "/submissions/"+id:
			state := "In Progress"
			if statusCalls.Add(1) > 1 {
				state = terminal
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id, "attributes": map[string]string{"status": state}}})
		case r.URL.Path == "/submissions/"+id+"/logs":
			_, _ = io.WriteString(w, `{"data":{"id":"`+id+`","attributes":{"developerLogUrl":"https://notary-test.s3.us-west-2.amazonaws.com/log?secret=temporary"}}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := &notaryClient{http: server.Client(), endpoint: server.URL, key: key, keyID: "TESTKEY", issuerID: issuer, pollInterval: time.Millisecond}
	submission, err := client.submit(context.Background(), "Test.app.zip", strings.Repeat("a", 64))
	if err != nil || submission.Data.ID != id {
		t.Fatal(submission.Data.ID, err)
	}
	originalTransport := client.http.Transport
	uploadSeen, logSeen := false, false
	client.http.Transport = notaryTestTransport(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "notary-test-bucket.s3.us-west-2.amazonaws.com":
			uploadSeen = true
			data, _ := io.ReadAll(r.Body)
			const canonicalURI = "/prod/role%3Asession%2B%20space/%252F/%E6%96%87%E4%BB%B6.zip"
			if string(data) != "archive fixture" || r.URL.Path != "/prod/role:session+ space/%2F/文件.zip" || r.URL.EscapedPath() != canonicalURI || r.Header.Get("X-Amz-Security-Token") != "UPLOAD_TOKEN" || !strings.Contains(r.Header.Get("Authorization"), "Credential=UPLOAD_ID/") || strings.Contains(r.Header.Get("Authorization"), "Bearer") {
				t.Error("S3 上传必须使用临时凭据并保留归档字节")
			}
			notaryTestUploadSignature(t, r, canonicalURI)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		case "notary-test.s3.us-west-2.amazonaws.com":
			logSeen = true
			if r.Header.Get("Authorization") != "" {
				t.Error("日志下载不得转发 JWT")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"` + terminal + `","issues":[]}`)), Header: make(http.Header)}, nil
		default:
			return originalTransport.RoundTrip(r)
		}
	})
	file := filepath.Join(t.TempDir(), "input.zip")
	if err = os.WriteFile(file, []byte("archive fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("archive fixture"))
	if err = client.upload(context.Background(), submission, file, hex.EncodeToString(digest[:])); err != nil || !uploadSeen {
		t.Fatal("S3 上传未完成", err)
	}
	receipt := notaryReceipt{Format: 1, ID: id}
	path := filepath.Join(t.TempDir(), "notary.json")
	if err = saveNotaryReceipt(path, &receipt, true); err != nil {
		t.Fatal(err)
	}
	if err = saveNotaryReceipt(path, &receipt, true); err == nil {
		t.Fatal("不能覆盖已有任务收据")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = client.poll(ctx, &receipt, true, func() error { return saveNotaryReceipt(path, &receipt, false) }); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "Accepted" || !logSeen || statusCalls.Load() != 2 {
		t.Fatal("必须等待终态并保存诊断日志", receipt.Status, statusCalls.Load())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("UPLOAD_SECRET")) || bytes.Contains(raw, []byte("UPLOAD_TOKEN")) || bytes.Contains(raw, []byte("Bearer")) {
		t.Fatal("收据不能包含凭据")
	}
	terminal = "Invalid"
	statusCalls.Store(2)
	if err = client.poll(ctx, &receipt, true, func() error { return nil }); err == nil || receipt.Status != "Invalid" || len(receipt.Log) == 0 {
		t.Fatal("Invalid 必须返回失败并保留诊断", err)
	}
	terminal = "unknown"
	if _, err = client.status(ctx, id); err == nil {
		t.Fatal("不能将未知状态算作通过")
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if err = client.poll(canceled, &receipt, true, func() error { return nil }); err == nil {
		t.Fatal("必须响应取消")
	}
	receipt.ID = "aaaaaaaa-2222-3333-4444-555555555555"
	if err = saveNotaryReceipt(path, &receipt, false); err == nil {
		t.Fatal("不能覆盖另一个任务的收据")
	}
}

// 按 AWS SigV4 文档独立验算，预期 URI 是明确字面量，不复用生产编码器或签名器。
func notaryTestUploadSignature(t *testing.T, r *http.Request, canonicalURI string) {
	t.Helper()
	fields := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 "), ", ") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			t.Fatal("无效的 SigV4 授权格式")
		}
		fields[key] = value
	}
	date := r.Header.Get("X-Amz-Date")
	if len(date) != 16 || r.URL.RawQuery != "" || fields["Credential"] != "UPLOAD_ID/"+date[:8]+"/us-west-2/s3/aws4_request" {
		t.Fatal("无效的签名日期、区域或服务范围")
	}
	names := strings.Split(fields["SignedHeaders"], ";")
	if !sort.StringsAreSorted(names) || !strings.Contains(";"+fields["SignedHeaders"]+";", ";x-amz-security-token;") {
		t.Fatal("签名头必须排序，并涵盖临时会话 token")
	}
	var headers strings.Builder
	for _, name := range names {
		value := strings.Join(r.Header.Values(name), ",")
		switch name {
		case "host":
			value = r.URL.Host
		case "content-length":
			value = strconv.FormatInt(r.ContentLength, 10)
		}
		fmt.Fprintf(&headers, "%s:%s\n", name, strings.Join(strings.Fields(value), " "))
	}
	payload := sha256.Sum256([]byte("archive fixture"))
	canonical := strings.Join([]string{"PUT", canonicalURI, "", headers.String(), fields["SignedHeaders"], hex.EncodeToString(payload[:])}, "\n")
	digest := sha256.Sum256([]byte(canonical))
	scope := date[:8] + "/us-west-2/s3/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + date + "\n" + scope + "\n" + hex.EncodeToString(digest[:])
	sign := func(key []byte, value string) []byte {
		hash := hmac.New(sha256.New, key)
		_, _ = hash.Write([]byte(value))
		return hash.Sum(nil)
	}
	key := sign([]byte("AWS4UPLOAD_SECRET"), date[:8])
	for _, part := range []string{"us-west-2", "s3", "aws4_request"} {
		key = sign(key, part)
	}
	if fields["Signature"] != hex.EncodeToString(sign(key, toSign)) {
		t.Error("上传签名与 AWS 规范的独立预期不一致")
	}
}

func TestNotaryFailureBoundaries(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const id = "11111111-2222-3333-4444-555555555555"
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"secret":"must not print"}`)
			}))
			defer server.Close()
			c := notaryClient{http: server.Client(), endpoint: server.URL, key: key, keyID: "KEY", issuerID: id}
			_, err := c.status(context.Background(), id)
			if err == nil || strings.Contains(err.Error(), "must not print") {
				t.Fatal("服务错误不能伪造成功或泄露响应内容", err)
			}
		})
	}
	c := testConfig(t)
	c.MacOS.Notary.PrivateKey = "missing.p8"
	if _, err = newNotaryClient(c); err == nil {
		t.Fatal("缺失的身份字段必须报错")
	}
	var out bytes.Buffer
	for _, args := range [][]string{{"notarize", "-h"}, {"staple", "-h"}, {"archive", "-h"}} {
		if err = run(context.Background(), args, &out, &out); err != nil {
			t.Fatal(err)
		}
	}
	if err = run(context.Background(), []string{"notarize", "-target", "linux/amd64"}, &out, &out); err == nil {
		t.Fatal("必须拒绝非 macOS 目标")
	}
}

func TestNotaryClientConfig(t *testing.T) {
	c := testConfig(t)
	c.MacOS.Notary.PrivateKey = "AuthKey.p8"
	c.MacOS.Notary.KeyID = "KEY123"
	c.MacOS.Notary.IssuerID = "11111111-2222-3333-4444-555555555555"
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := c.path(c.MacOS.Notary.PrivateKey)
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	writeTestFile(t, path, encoded)
	client, err := newNotaryClient(c)
	if err != nil || !client.key.PublicKey.Equal(&key.PublicKey) || client.keyID != c.MacOS.Notary.KeyID || client.issuerID != c.MacOS.Notary.IssuerID {
		t.Fatal("团队 API 配置或 PKCS#8 密钥未正确加载", err)
	}
	writeTestFile(t, path, append(encoded, encoded...))
	if _, err = newNotaryClient(c); err == nil {
		t.Fatal("不能接受含多个密钥的文件")
	}
	other, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err = x509.MarshalPKCS8PrivateKey(other)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if _, err = newNotaryClient(c); err == nil {
		t.Fatal("ES256 必须使用 P-256 密钥")
	}
}

func notaryTestTicket(t *testing.T, hash [20]byte, authorized bool) ([]byte, *x509.CertPool) {
	t.Helper()
	caKey, ca := iosTestCertificate(t, 401, pkix.Name{CommonName: "GOUI ticket test root"}, nil, nil, true)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	purpose := asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 1, 30}
	if !authorized {
		purpose = developerIDApplicationOID
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(402), Subject: pkix.Name{CommonName: "GOUI ticket test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtraExtensions: []pkix.Extension{{Id: purpose, Critical: true, Value: []byte{5, 0}}}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	certificates, err := asn1.Marshal(asn1.RawValue{Tag: asn1.TagSequence, IsCompound: true, Bytes: append(der, ca.Raw...)})
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 45)
	copy(payload, "g8tk")
	binary.LittleEndian.PutUint16(payload[4:], 2)
	binary.LittleEndian.PutUint16(payload[6:], 20)
	binary.LittleEndian.PutUint32(payload[8:], 1)
	binary.LittleEndian.PutUint64(payload[16:], uint64(time.Now().Unix()))
	payload[24] = 2
	copy(payload[25:], hash[:])
	header := make([]byte, 16)
	copy(header, "s8ch")
	binary.LittleEndian.PutUint32(header[4:], 1)
	binary.LittleEndian.PutUint32(header[8:], uint32(len(certificates)))
	binary.LittleEndian.PutUint32(header[12:], uint32(len(payload)))
	data := append(append(header, certificates...), payload...)
	digest := sha256.Sum256(data)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature = append(signature, make([]byte, (4-len(signature)%4)%4)...)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return append(data, signature...), roots
}

func TestNotaryTickets(t *testing.T) {
	hash := [20]byte{1, 2, 3}
	ticket, roots := notaryTestTicket(t, hash, true)
	if err := verifyNotaryTicket(ticket, hash, roots); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{0, 4, 8, 12, 16, len(ticket) - 4} {
		changed := bytes.Clone(ticket)
		changed[index] ^= 1
		if err := verifyNotaryTicket(changed, hash, roots); err == nil {
			t.Fatalf("篡改 %d 必须失败", index)
		}
	}
	for n := 0; n < 16; n++ {
		if err := verifyNotaryTicket(ticket[:n], hash, roots); err == nil {
			t.Fatal("截断票据必须失败")
		}
	}
	if err := verifyNotaryTicket(ticket, [20]byte{9}, roots); err == nil {
		t.Fatal("票据必须绑定目标 cdhash")
	}
	if err := verifyNotaryTicket(ticket, hash, x509.NewCertPool()); err == nil {
		t.Fatal("票据必须验证 Apple 信任链")
	}
	wrongRole, roleRoots := notaryTestTicket(t, hash, false)
	if err := verifyNotaryTicket(wrongRole, hash, roleRoots); err == nil {
		t.Fatal("普通代码证书不能签发票据")
	}
	mode := "valid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("公开票据查询不应发送身份凭据")
		}
		var query struct {
			Records []struct {
				Name string `json:"recordName"`
			} `json:"records"`
		}
		_ = json.NewDecoder(r.Body).Decode(&query)
		if len(query.Records) != 1 || query.Records[0].Name != "2/2/"+hex.EncodeToString(hash[:]) {
			t.Error("票据查询必须使用实际 cdhash")
		}
		record := map[string]any{"recordName": query.Records[0].Name, "recordType": "DeveloperIDTicket", "fields": map[string]any{"signedTicket": map[string]any{"type": "BYTES", "value": ticket}}}
		switch mode {
		case "wrong record":
			record["recordName"] = "other"
		case "pending":
			record["serverErrorCode"] = "UNKNOWN_ITEM"
		case "deleted":
			record["deleted"] = true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"records": []any{record}})
	}))
	defer server.Close()
	if _, err := fetchNotaryTicket(context.Background(), server.Client(), server.URL, hash, roots); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"wrong record", "pending", "deleted"} {
		mode = value
		if _, err := fetchNotaryTicket(context.Background(), server.Client(), server.URL, hash, roots); err == nil {
			t.Fatal("错误或缺失记录不能算作装订成功", value)
		}
	}
}
