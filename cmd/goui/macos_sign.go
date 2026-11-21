package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aluedeke/go-codesign/pkg/codesign"
	gomacho "github.com/blacktop/go-macho"
	machotypes "github.com/blacktop/go-macho/pkg/codesign/types"
	"github.com/golang-gui/goui/cmd/goui/internal/applecerts"
	"go.mozilla.org/pkcs7"
	appleplist "howett.net/plist"
)

var developerIDApplicationOID = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 1, 13}

func developerIDIdentity(key crypto.PrivateKey, certs []*x509.Certificate, roots *x509.CertPool, at time.Time) (*codesign.SigningIdentity, error) {
	if len(certs) == 0 || roots == nil {
		return nil, fmt.Errorf("Developer ID signing requires a certificate and explicit Apple trust roots")
	}
	leaf := certs[0]
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("unsupported Developer ID signing key")
	}
	pub, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil || !bytes.Equal(pub, leaf.RawSubjectPublicKeyInfo) {
		return nil, fmt.Errorf("Developer ID signing key does not match certificate")
	}
	if err = checkDeveloperIDCertificate(leaf); err != nil {
		return nil, err
	}
	intermediate := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediate.AddCert(c)
	}
	roleCert, err := appleRoleCertificate(leaf, developerIDApplicationOID)
	if err != nil {
		return nil, err
	}
	chains, err := roleCert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediate, CurrentTime: at, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}})
	if err != nil {
		return nil, fmt.Errorf("verify Developer ID chain (include intermediate certificates in PKCS#12): %w", err)
	}
	return &codesign.SigningIdentity{PrivateKey: key, Certificate: leaf, CertChain: chains[0], TeamID: leaf.Subject.OrganizationalUnit[0]}, nil
}

func checkDeveloperIDCertificate(c *x509.Certificate) error {
	if c.IsCA || c.KeyUsage&x509.KeyUsageDigitalSignature == 0 || len(c.Subject.OrganizationalUnit) != 1 || c.Subject.OrganizationalUnit[0] == "" {
		return fmt.Errorf("invalid Developer ID Application certificate or Team ID")
	}
	for _, e := range c.Extensions {
		if e.Id.Equal(developerIDApplicationOID) && bytes.Equal(e.Value, []byte{5, 0}) {
			return nil
		}
	}
	return fmt.Errorf("macOS distribution requires a Developer ID Application certificate")
}

func signMacOSPKCS12(ctx context.Context, c *config, app string) error {
	key, chain, err := loadSigningIdentity(c.path(c.MacOS.Signing.PKCS12), c.MacOS.Signing.PasswordEnv)
	if err != nil {
		return err
	}
	roots, err := applecerts.Roots()
	if err != nil {
		return err
	}
	identity, err := developerIDIdentity(key, chain, roots, time.Now())
	if err != nil {
		return err
	}
	err = signMacOSBundle(ctx, c, app, identity, func(sig []byte) ([]byte, error) {
		return requestAppleTimestamp(ctx, appleHTTPClient(), appleTimestampURL, roots, sig)
	})
	if err != nil {
		return err
	}
	_, err = verifyMacOSBundle(app, roots)
	return err
}

func signMacOSBundle(ctx context.Context, c *config, app string, identity *codesign.SigningIdentity, stamp func([]byte) ([]byte, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	executable, err := macOSBundleExecutable(app)
	if err != nil {
		return err
	}
	if executable != c.Executable {
		return fmt.Errorf("macOS executable does not match configuration")
	}
	seal, err := macOSResourceSeal(app, executable)
	if err != nil {
		return err
	}
	base := filepath.Join(app, "Contents")
	info, err := os.ReadFile(filepath.Join(base, "Info.plist"))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(base, "_CodeSignature"), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(base, "_CodeSignature", "CodeResources"), seal, 0644); err != nil {
		return err
	}
	if stamp == nil {
		return fmt.Errorf("Developer ID signing requires a secure timestamp")
	}
	if err = signAppleMachO(filepath.Join(base, "MacOS", executable), c.ID, info, seal, macOSEntitlements(c), identity, true, stamp); err != nil {
		return fmt.Errorf("sign macOS app: %w", err)
	}
	if err = verifyAppleBundleSignature(base, filepath.ToSlash(filepath.Join("MacOS", executable)), identity.Certificate, true); err != nil {
		return err
	}
	return ctx.Err()
}

func macOSBundleExecutable(app string) (string, error) {
	stat, err := os.Lstat(app)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() || filepath.Ext(app) != ".app" {
		return "", fmt.Errorf("expected a regular .app directory")
	}
	for _, p := range []string{"Contents", "Contents/Info.plist", "Contents/MacOS"} {
		st, e := os.Lstat(filepath.Join(app, filepath.FromSlash(p)))
		if e != nil {
			return "", e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("macOS bundle must not contain symbolic links: %s", p)
		}
	}
	raw, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return "", err
	}
	var info struct {
		Executable string `plist:"CFBundleExecutable"`
		ID         string `plist:"CFBundleIdentifier"`
		Package    string `plist:"CFBundlePackageType"`
	}
	if _, err = appleplist.Unmarshal(raw, &info); err != nil {
		return "", err
	}
	if !executableName.MatchString(info.Executable) || !applicationID.MatchString(info.ID) || info.Package != "APPL" {
		return "", fmt.Errorf("invalid macOS bundle identity")
	}
	return info.Executable, nil
}

func macOSResources(app, executable string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(app, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(app, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported symbolic link: %s", rel)
		}
		if d.IsDir() {
			if rel == "Contents" || rel == "Contents/MacOS" || rel == "Contents/Resources" || rel == "Contents/_CodeSignature" {
				return nil
			}
			if strings.HasPrefix(rel, "Contents/Resources/") && !strings.HasSuffix(rel, ".app") && !strings.HasSuffix(rel, ".framework") && !strings.HasSuffix(rel, ".bundle") {
				return nil
			}
			return fmt.Errorf("unsupported nested code or bundle directory: %s", rel)
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("unsupported bundle file: %s", rel)
		}
		switch rel {
		case "Contents/Info.plist", "Contents/MacOS/" + executable, "Contents/_CodeSignature/CodeResources", "Contents/CodeResources":
			return nil
		}
		if !strings.HasPrefix(rel, "Contents/Resources/") {
			return fmt.Errorf("unsupported bundle file: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) >= 4 {
			switch string(data[:4]) {
			case "\xcf\xfa\xed\xfe", "\xfe\xed\xfa\xcf", "\xca\xfe\xba\xbe", "\xce\xfa\xed\xfe":
				return fmt.Errorf("nested executable is unsupported: %s", rel)
			}
		}
		files[strings.TrimPrefix(rel, "Contents/")] = data
		return nil
	})
	return files, err
}

func macOSResourceSeal(app, executable string) ([]byte, error) {
	files, err := macOSResources(app, executable)
	if err != nil {
		return nil, err
	}
	legacy, modern := map[string]any{}, map[string]any{}
	for p, data := range files {
		h1 := sha1.Sum(data)
		h2 := sha256.Sum256(data)
		legacy[p] = h1[:]
		modern[p] = map[string]any{"hash": h1[:], "hash2": h2[:]}
	}
	// Rules follow codesign's desktop bundle format; the supported layout excludes nested code.
	rules := map[string]any{"^Resources/": true, "^version.plist$": true}
	rules2 := map[string]any{"^.*": true, "^Resources/": map[string]any{"weight": 20}, "^Info\\.plist$": map[string]any{"omit": true, "weight": 20}, "^PkgInfo$": map[string]any{"omit": true, "weight": 20}, "^(Frameworks|SharedFrameworks|PlugIns|Plug-ins|XPCServices|Helpers|MacOS|Library/(Automator|Spotlight|LoginItems))/": map[string]any{"nested": true, "weight": 10}, "^[^/]+$": map[string]any{"nested": true, "weight": 10}}
	return appleplist.MarshalIndent(map[string]any{"files": legacy, "files2": modern, "rules": rules, "rules2": rules2}, appleplist.XMLFormat, "\t")
}

func verifyMacOSBundle(app string, roots *x509.CertPool) ([20]byte, error) {
	var cdhash [20]byte
	exe, err := macOSBundleExecutable(app)
	if err != nil {
		return cdhash, err
	}
	path := filepath.Join(app, "Contents", "MacOS", exe)
	if err = checkBinary(path, target{"darwin", "arm64"}); err != nil {
		return cdhash, err
	}
	m, err := gomacho.Open(path)
	if err != nil {
		return cdhash, err
	}
	defer m.Close()
	cs := m.CodeSignature()
	if cs == nil || len(cs.CodeDirectories) != 1 || len(cs.CMSSignature) == 0 {
		return cdhash, fmt.Errorf("macOS app requires one signed code directory")
	}
	cms, err := pkcs7.Parse(cs.CMSSignature)
	if err != nil {
		return cdhash, err
	}
	if len(cms.Signers) != 1 {
		return cdhash, fmt.Errorf("macOS app requires one CMS signer")
	}
	signature := cms.Signers[0].EncryptedDigest
	token, err := appleCMSTimestamp(cms)
	if err != nil {
		return cdhash, err
	}
	issued, err := verifyAppleTimestamp(token, signature, roots)
	if err != nil {
		return cdhash, err
	}
	signer := cms.GetOnlySigner()
	if signer == nil {
		return cdhash, fmt.Errorf("missing Developer ID signer")
	}
	if err = checkDeveloperIDCertificate(signer); err != nil {
		return cdhash, err
	}
	intermediate := x509.NewCertPool()
	for _, cert := range cms.Certificates {
		if !cert.Equal(signer) {
			intermediate.AddCert(cert)
		}
	}
	roleCert, err := appleRoleCertificate(signer, developerIDApplicationOID)
	if err != nil {
		return cdhash, err
	}
	if _, err = roleCert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediate, CurrentTime: issued, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}); err != nil {
		return cdhash, err
	}
	var info struct {
		ID string `plist:"CFBundleIdentifier"`
	}
	raw, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return cdhash, err
	}
	if _, err = appleplist.Unmarshal(raw, &info); err != nil {
		return cdhash, err
	}
	cd := cs.CodeDirectories[0]
	if cd.Header.Flags&machotypes.RUNTIME == 0 || cd.ID != info.ID || cd.TeamID != signer.Subject.OrganizationalUnit[0] {
		return cdhash, fmt.Errorf("invalid macOS runtime, application or team identity")
	}
	if cs.Entitlements != "" {
		ent, err := codesign.ParseEntitlementsXML([]byte(cs.Entitlements))
		if err != nil {
			return cdhash, err
		}
		if ent["com.apple.security.get-task-allow"] == true {
			return cdhash, fmt.Errorf("distribution must not enable get-task-allow")
		}
	}
	base := filepath.Join(app, "Contents")
	if err = verifyAppleBundleSignature(base, "MacOS/"+exe, signer, true); err != nil {
		return cdhash, err
	}
	files, err := macOSResources(app, exe)
	if err != nil {
		return cdhash, err
	}
	raw, err = os.ReadFile(filepath.Join(base, "_CodeSignature", "CodeResources"))
	if err != nil {
		return cdhash, err
	}
	var seal struct {
		Files map[string]struct {
			Hash2 []byte `plist:"hash2"`
		} `plist:"files2"`
	}
	if _, err = appleplist.Unmarshal(raw, &seal); err != nil {
		return cdhash, err
	}
	if len(files) != len(seal.Files) {
		return cdhash, fmt.Errorf("macOS sealed resource set differs from bundle")
	}
	for p, data := range files {
		sum := sha256.Sum256(data)
		if !bytes.Equal(seal.Files[p].Hash2, sum[:]) {
			return cdhash, fmt.Errorf("macOS resource checksum mismatch: %s", p)
		}
	}
	// The raw CodeDirectory is read independently of the CMS attributes.
	data, err := os.ReadFile(path)
	if err != nil {
		return cdhash, err
	}
	blobs, _, err := appleCodeBlobs(data)
	if err != nil {
		return cdhash, err
	}
	sum := sha256.Sum256(blobs[0])
	copy(cdhash[:], sum[:20])
	return cdhash, nil
}
