package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"debug/macho"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aluedeke/go-codesign/pkg/codesign"
	gomacho "github.com/blacktop/go-macho"
	machosign "github.com/blacktop/go-macho/pkg/codesign"
	machotypes "github.com/blacktop/go-macho/pkg/codesign/types"
	"github.com/golang-gui/goui/cmd/goui/internal/applecerts"
	"go.mozilla.org/pkcs7"
	appleplist "howett.net/plist"
)

func iosSigningIdentity(c *config, roots *x509.CertPool) (*codesign.SigningIdentity, []byte, []byte, error) {
	if roots == nil {
		return nil, nil, nil, fmt.Errorf("iOS signing requires explicit trust roots")
	}
	key, certs, err := mobileIdentity(c, c.IOS.Signing)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(certs) == 0 || time.Now().Before(certs[0].NotBefore) || time.Now().After(certs[0].NotAfter) {
		return nil, nil, nil, fmt.Errorf("iOS signing certificate is outside its validity period")
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, nil, fmt.Errorf("unsupported iOS signing key")
	}
	pub, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil || !bytes.Equal(pub, certs[0].RawSubjectPublicKeyInfo) {
		return nil, nil, nil, fmt.Errorf("iOS signing key does not match its certificate")
	}
	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		intermediates.AddCert(cert)
	}
	if _, err = certs[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}); err != nil {
		return nil, nil, nil, fmt.Errorf("verify iOS certificate chain (include intermediate CA certificates in PKCS#12): %w", err)
	}
	raw, err := os.ReadFile(c.path(c.IOS.Signing.Profile))
	if err != nil {
		return nil, nil, nil, err
	}
	container, err := pkcs7.Parse(raw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse provisioning CMS: %w", err)
	}
	if err = container.VerifyWithChain(roots); err != nil {
		return nil, nil, nil, fmt.Errorf("verify provisioning CMS: %w", err)
	}
	// Apple's SecPolicyCreateiPhoneProvisioningProfileSigning restricts this
	// role as well as the trust anchor. A developer certificate chaining to
	// Apple does not authorize its holder to issue provisioning profiles.
	profileSigner := container.GetOnlySigner()
	if profileSigner == nil || profileSigner.Subject.CommonName != "Apple iPhone OS Provisioning Profile Signing" || profileSigner.Issuer.CommonName != "Apple iPhone Certification Authority" {
		return nil, nil, nil, fmt.Errorf("unexpected iOS provisioning profile signer")
	}
	profile, err := codesign.ParseProvisioningProfile(raw)
	if err != nil {
		return nil, nil, nil, err
	}
	if profile.IsExpired() || time.Now().Before(profile.CreationDate) {
		return nil, nil, nil, fmt.Errorf("iOS provisioning profile is outside its validity period")
	}
	if !profile.MatchesCertificate(certs[0]) {
		return nil, nil, nil, fmt.Errorf("iOS certificate is not authorized by the provisioning profile")
	}
	platform := false
	for _, p := range profile.Platform {
		if p == "iOS" {
			platform = true
		}
	}
	if !platform {
		return nil, nil, nil, fmt.Errorf("provisioning profile does not support iOS")
	}
	team := profile.GetTeamID()
	if team == "" || len(profile.ApplicationIdentifierPrefix) != 1 {
		return nil, nil, nil, fmt.Errorf("provisioning profile requires one application identifier prefix and a team")
	}
	authorized := false
	for _, ou := range certs[0].Subject.OrganizationalUnit {
		if ou == team {
			authorized = true
		}
	}
	if !authorized {
		return nil, nil, nil, fmt.Errorf("iOS certificate team does not match provisioning profile")
	}
	appID := profile.ApplicationIdentifierPrefix[0] + "." + c.ID
	allowed := profile.GetApplicationIdentifier()
	if allowed != appID && !(strings.HasSuffix(allowed, ".*") && strings.HasPrefix(appID, strings.TrimSuffix(allowed, "*"))) {
		return nil, nil, nil, fmt.Errorf("provisioning profile does not authorize application %s", c.ID)
	}
	// Request only the baseline signing entitlements. Platform capabilities will
	// add their own explicit configuration; a profile is not a request for all
	// capabilities that it happens to authorize.
	entitlements := map[string]any{"application-identifier": appID, "com.apple.developer.team-identifier": team}
	if value, ok := profile.Entitlements["get-task-allow"].(bool); ok {
		entitlements["get-task-allow"] = value
	}
	entitlementsXML, err := codesign.EntitlementsToXML(entitlements)
	if err != nil {
		return nil, nil, nil, err
	}
	identity := &codesign.SigningIdentity{Certificate: certs[0], PrivateKey: key, CertChain: certs, TeamID: team}
	return identity, raw, entitlementsXML, nil
}

func checkIOSSigning(c *config) error {
	roots, err := applecerts.Roots()
	if err != nil {
		return err
	}
	_, _, _, err = iosSigningIdentity(c, roots)
	return err
}

func signIOS(ctx context.Context, c *config, app string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	roots, err := applecerts.Roots()
	if err != nil {
		return err
	}
	identity, profile, entitlements, err := iosSigningIdentity(c, roots)
	if err != nil {
		return err
	}
	return signIOSBundle(ctx, c, app, identity, profile, entitlements)
}

func signIOSBundle(ctx context.Context, c *config, app string, identity *codesign.SigningIdentity, profile, entitlements []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	if err = os.WriteFile(filepath.Join(app, "embedded.mobileprovision"), profile, 0644); err != nil {
		return err
	}
	// Constructing the identity explicitly keeps signing offline. The library's
	// identity loader may otherwise download intermediate CA certificates.
	if err = signIOSMachO(app, c.Executable, identity, entitlements, c.ID); err != nil {
		return fmt.Errorf("sign iOS app: %w", err)
	}
	if err = verifyIOSSignature(app, c.Executable, identity.Certificate); err != nil {
		return fmt.Errorf("verify signed iOS app: %w", err)
	}
	return ctx.Err()
}

func signIOSMachO(app, executable string, identity *codesign.SigningIdentity, entitlements []byte, id string) error {
	if err := codesign.WriteCodeResources(app); err != nil {
		return err
	}
	info, err := os.ReadFile(filepath.Join(app, "Info.plist"))
	if err != nil {
		return err
	}
	resources, err := os.ReadFile(filepath.Join(app, "_CodeSignature", "CodeResources"))
	if err != nil {
		return err
	}
	ent, err := codesign.ParseEntitlementsXML(entitlements)
	if err != nil {
		return err
	}
	der, err := codesign.EntitlementsToDER(ent)
	if err != nil {
		return err
	}
	path := filepath.Join(app, executable)
	m, err := gomacho.Open(path)
	if err != nil {
		return err
	}
	defer m.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(before) < 32 || m.CodeSignature() == nil {
		return fmt.Errorf("iOS signing requires a linked ARM64 executable with a signature slot")
	}
	headerEnd := uint64(32) + uint64(binary.LittleEndian.Uint32(before[20:]))
	signatureOffset := uint64(m.CodeSignature().Offset)
	hash := sha256.Sum256(resources)
	config := &machosign.Config{ID: id, TeamID: identity.TeamID, InfoPlist: info, Entitlements: entitlements, EntitlementsDER: der, ResourceDirSlotHash: hash[:], CertChain: identity.CertChain, SpecialSlots: []machotypes.SpecialSlot{}}
	config.SignerFunction = func(directory []byte) ([]byte, error) {
		signed, err := pkcs7.NewSignedData(directory)
		if err != nil {
			return nil, err
		}
		signed.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
		digest := sha256.Sum256(directory)
		plistHash, err := appleplist.Marshal(map[string]interface{}{"cdhashes": [][]byte{digest[:20]}}, appleplist.XMLFormat)
		if err != nil {
			return nil, err
		}
		hashSequence, err := asn1.Marshal(struct {
			Algorithm asn1.ObjectIdentifier
			Digest    []byte
		}{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}, digest[:]})
		if err != nil {
			return nil, err
		}
		attributes := []pkcs7.Attribute{{Type: asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 9, 1}, Value: plistHash}, {Type: asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 9, 2}, Value: asn1.RawValue{FullBytes: hashSequence}}}
		if err = signed.AddSignerChain(identity.Certificate, identity.PrivateKey, identity.CertChain[1:], pkcs7.SignerInfoConfig{ExtraSignedAttributes: attributes}); err != nil {
			return nil, err
		}
		signed.Detach()
		return signed.Finish()
	}
	if err = m.CodeSign(config); err != nil {
		return err
	}
	var output bytes.Buffer
	if err = m.SaveBuffer(&output); err != nil {
		return err
	}
	if headerEnd > signatureOffset || signatureOffset > uint64(len(before)) || signatureOffset > uint64(output.Len()) || !bytes.Equal(before[headerEnd:signatureOffset], output.Bytes()[headerEnd:signatureOffset]) {
		return fmt.Errorf("iOS signer changed executable data outside load commands and signature")
	}
	return os.WriteFile(path, output.Bytes(), 0755)
}

func verifyIOSSignature(app, executable string, certificate *x509.Certificate) error {
	path := filepath.Join(app, executable)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer m.Close()
	var offset, size uint32
	for _, load := range m.Loads {
		raw := load.Raw()
		if len(raw) >= 16 && binary.LittleEndian.Uint32(raw) == 0x1d {
			offset, size = binary.LittleEndian.Uint32(raw[8:]), binary.LittleEndian.Uint32(raw[12:])
		}
	}
	if size < 12 || uint64(offset)+uint64(size) > uint64(len(data)) {
		return fmt.Errorf("invalid Mach-O signature bounds")
	}
	super := data[offset : offset+size]
	count := binary.BigEndian.Uint32(super[8:])
	if uint64(count)*8+12 > uint64(len(super)) {
		return fmt.Errorf("invalid signature index")
	}
	blobs := map[uint32][]byte{}
	for i := uint32(0); i < count; i++ {
		slot, at := binary.BigEndian.Uint32(super[12+8*i:]), binary.BigEndian.Uint32(super[16+8*i:])
		if uint64(at)+8 > uint64(len(super)) {
			return fmt.Errorf("invalid signature blob")
		}
		n := binary.BigEndian.Uint32(super[at+4:])
		if n < 8 || uint64(at)+uint64(n) > uint64(len(super)) {
			return fmt.Errorf("invalid signature blob size")
		}
		blobs[slot] = super[at : at+n]
	}
	if len(blobs[0]) == 0 || len(blobs[0x10000]) < 8 {
		return fmt.Errorf("missing code directory or CMS signature")
	}
	cms, err := pkcs7.Parse(blobs[0x10000][8:])
	if err != nil {
		return err
	}
	cms.Content = blobs[0]
	if err = cms.Verify(); err != nil {
		return err
	}
	if signer := cms.GetOnlySigner(); signer == nil || !signer.Equal(certificate) {
		return fmt.Errorf("unexpected iOS signature certificate")
	}
	info, err := codesign.ParseSignature(path)
	if err != nil {
		return err
	}
	for _, cd := range info.CodeDirs {
		if cd.CodeLimit != offset || cd.PageSize == 0 || uint64(len(cd.CodeHashes)) != (uint64(cd.CodeLimit)+uint64(cd.PageSize)-1)/uint64(cd.PageSize) {
			return fmt.Errorf("invalid signed code range")
		}
		for i, want := range cd.CodeHashes {
			start := uint64(i) * uint64(cd.PageSize)
			end := min(start+uint64(cd.PageSize), uint64(cd.CodeLimit))
			if start >= end || !iosHashEqual(data[start:end], want, cd.HashType) {
				return fmt.Errorf("iOS code page %d checksum mismatch", i)
			}
		}
		for slot, want := range cd.SpecialHashes {
			var b []byte
			switch slot {
			case -1:
				b, err = os.ReadFile(filepath.Join(app, "Info.plist"))
			case -3:
				b, err = os.ReadFile(filepath.Join(app, "_CodeSignature", "CodeResources"))
			default:
				b = blobs[uint32(-slot)]
			}
			if err != nil {
				return err
			}
			if len(b) == 0 && bytes.Equal(want, make([]byte, len(want))) {
				continue
			}
			if !iosHashEqual(b, want, cd.HashType) {
				return fmt.Errorf("iOS special slot %d checksum mismatch", slot)
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(app, "_CodeSignature", "CodeResources"))
	if err != nil {
		return err
	}
	var seal struct {
		Files map[string]struct {
			Hash  []byte `plist:"hash"`
			Hash2 []byte `plist:"hash2"`
		} `plist:"files2"`
	}
	if _, err = appleplist.Unmarshal(raw, &seal); err != nil {
		return err
	}
	return filepath.WalkDir(app, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "_CodeSignature" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(app, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == executable || rel == "Info.plist" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entry, ok := seal.Files[rel]
		if !ok || !iosHashEqual(b, entry.Hash2, 2) {
			return fmt.Errorf("iOS resource checksum mismatch: %s", rel)
		}
		return nil
	})
}

func iosHashEqual(data, want []byte, kind uint8) bool {
	switch kind {
	case 1:
		sum := sha1.Sum(data)
		return bytes.Equal(sum[:], want)
	case 2:
		sum := sha256.Sum256(data)
		return bytes.Equal(sum[:], want)
	}
	return false
}
