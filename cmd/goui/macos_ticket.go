package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const appleTicketURL = "https://api.apple-cloudkit.com/database/1/com.apple.gk.ticket-delivery/production/public/records/lookup"

var errNotaryTicketPending = errors.New("Apple notarization ticket is not available yet")

// verifyNotaryTicket accepts the observed s8ch/v1 and positive g8tk/v2 format.
// Apple does not publish a stable ticket-format contract; unknown formats fail closed.
func verifyNotaryTicket(data []byte, expected [20]byte, roots *x509.CertPool) error {
	le := binary.LittleEndian
	if expected == ([20]byte{}) || roots == nil {
		return fmt.Errorf("ticket validation requires a code hash and explicit Apple trust roots")
	}
	if len(data) < 16 || len(data) > 4<<20 || string(data[:4]) != "s8ch" || le.Uint32(data[4:]) != 1 {
		return fmt.Errorf("unsupported notarization ticket envelope")
	}
	certEnd := uint64(16) + uint64(le.Uint32(data[8:]))
	signedEnd := certEnd + uint64(le.Uint32(data[12:]))
	if certEnd <= 16 || signedEnd < certEnd+24 || signedEnd >= uint64(len(data)) || uint64(len(data))-signedEnd > 144 {
		return fmt.Errorf("invalid notarization ticket bounds")
	}
	payload := data[certEnd:signedEnd]
	if string(payload[:4]) != "g8tk" || le.Uint16(payload[4:]) != 2 || le.Uint16(payload[6:]) != 20 || le.Uint32(payload[12:]) != 0 {
		return fmt.Errorf("unsupported or non-positive notarization ticket payload")
	}
	count := uint64(le.Uint32(payload[8:]))
	if count == 0 || count > 100000 || 24+count*21 != uint64(len(payload)) {
		return fmt.Errorf("invalid notarization ticket hash list")
	}
	seconds := le.Uint64(payload[16:])
	if seconds < 946684800 || seconds > uint64(time.Now().Add(5*time.Minute).Unix()) {
		return fmt.Errorf("invalid notarization ticket issuance time")
	}
	found := false
	for i := 24; i < len(payload); i += 21 {
		if payload[i] != 2 {
			return fmt.Errorf("unsupported notarization hash algorithm")
		}
		if bytes.Equal(payload[i+1:i+21], expected[:]) {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("notarization ticket does not cover code directory %x", expected)
	}
	var encoded asn1.RawValue
	rest, err := asn1.Unmarshal(data[16:certEnd], &encoded)
	if err != nil || len(rest) != 0 || encoded.Tag != asn1.TagSequence || encoded.Class != 0 || !encoded.IsCompound {
		return fmt.Errorf("invalid ticket certificate sequence")
	}
	var certs []*x509.Certificate
	for raw := encoded.Bytes; len(raw) > 0; {
		var value asn1.RawValue
		remaining, err := asn1.Unmarshal(raw, &value)
		if err != nil {
			return err
		}
		cert, err := x509.ParseCertificate(value.FullBytes)
		if err != nil {
			return err
		}
		certs = append(certs, cert)
		if len(certs) > 8 {
			return fmt.Errorf("too many ticket certificates")
		}
		raw = remaining
	}
	if len(certs) == 0 {
		return fmt.Errorf("missing ticket signing certificate")
	}
	leaf := certs[0]
	authorized := false
	for _, extension := range leaf.Extensions {
		if extension.Id.Equal(asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 1, 30}) && bytes.Equal(extension.Value, []byte{5, 0}) {
			authorized = true
		}
	}
	if !authorized || leaf.IsCA || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return fmt.Errorf("certificate is not authorized for Apple software tickets")
	}
	intermediate := x509.NewCertPool()
	for _, cert := range certs[1:] {
		intermediate.AddCert(cert)
	}
	roleCert, err := appleRoleCertificate(leaf, asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 1, 30})
	if err != nil {
		return err
	}
	if _, err = roleCert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediate, CurrentTime: time.Unix(int64(seconds), 0), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		return fmt.Errorf("verify Apple ticket chain: %w", err)
	}
	var signature asn1.RawValue
	padding, err := asn1.Unmarshal(data[signedEnd:], &signature)
	if err != nil || len(data[signedEnd:])%4 != 0 || len(padding) > 3 || !bytes.Equal(padding, make([]byte, len(padding))) || signature.Tag != asn1.TagSequence || signature.Class != 0 {
		return fmt.Errorf("invalid ticket signature encoding")
	}
	if err = leaf.CheckSignature(x509.ECDSAWithSHA256, data[:signedEnd], signature.FullBytes); err != nil {
		return fmt.Errorf("invalid Apple ticket signature: %w", err)
	}
	return nil
}

func fetchNotaryTicket(ctx context.Context, client *http.Client, endpoint string, hash [20]byte, roots *x509.CertPool) ([]byte, error) {
	name := "2/2/" + hex.EncodeToString(hash[:])
	query := map[string]any{"records": []map[string]string{{"recordName": name}}}
	body, _ := json.Marshal(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query Apple notarization ticket: %w", err)
	}
	data, err := readAppleResponse(resp, 8<<20)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Records []struct {
			Name    string `json:"recordName"`
			Type    string `json:"recordType"`
			Deleted bool   `json:"deleted"`
			Error   string `json:"serverErrorCode"`
			Fields  struct {
				Ticket struct {
					Type  string `json:"type"`
					Value []byte `json:"value"`
				} `json:"signedTicket"`
			} `json:"fields"`
		} `json:"records"`
	}
	if err = json.Unmarshal(data, &reply); err != nil {
		return nil, err
	}
	if len(reply.Records) != 1 || reply.Records[0].Name != name {
		return nil, fmt.Errorf("unexpected notarization ticket response")
	}
	record := reply.Records[0]
	if record.Error == "UNKNOWN_ITEM" || record.Error == "NOT_FOUND" {
		return nil, errNotaryTicketPending
	}
	if record.Deleted || record.Error != "" || record.Type != "DeveloperIDTicket" || record.Fields.Ticket.Type != "BYTES" {
		return nil, fmt.Errorf("missing or invalid notarization ticket record")
	}
	ticket := record.Fields.Ticket.Value
	if err = verifyNotaryTicket(ticket, hash, roots); err != nil {
		return nil, err
	}
	return ticket, nil
}

func stapleMacOS(ctx context.Context, app string, roots *x509.CertPool) error {
	hash, err := verifyMacOSBundle(app, roots)
	if err != nil {
		return err
	}
	var ticket []byte
	for {
		ticket, err = fetchNotaryTicket(ctx, appleHTTPClient(), appleTicketURL, hash, roots)
		if !errors.Is(err, errNotaryTicketPending) {
			break
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("waiting for notarization ticket: %w", ctx.Err())
		case <-timer.C:
		}
	}
	if err != nil {
		return err
	}
	current, err := verifyMacOSBundle(app, roots)
	if err != nil {
		return err
	}
	if current != hash {
		return fmt.Errorf("macOS app changed while fetching its ticket")
	}
	path := filepath.Join(app, "Contents", "CodeResources")
	if st, e := os.Lstat(path); e == nil && !st.Mode().IsRegular() {
		return fmt.Errorf("existing notarization ticket must be a regular file")
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".goui-ticket-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(ticket); err != nil {
		file.Close()
		return err
	}
	if err = file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
