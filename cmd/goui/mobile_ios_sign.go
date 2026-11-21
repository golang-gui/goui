package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aluedeke/go-codesign/pkg/codesign"
	"github.com/golang-gui/goui/cmd/goui/internal/applecerts"
	"go.mozilla.org/pkcs7"
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
	return signAppleMachO(filepath.Join(app, executable), id, info, resources, entitlements, identity, false, nil)
}

func verifyIOSSignature(app, executable string, certificate *x509.Certificate) error {
	return verifyAppleBundleSignature(app, executable, certificate, false)
}
