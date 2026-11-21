package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"debug/macho"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aluedeke/go-codesign/pkg/codesign"
	"go.mozilla.org/pkcs7"
	appleplist "howett.net/plist"
)

// Apple signing roles are critical extensions that crypto/x509 does not know.
// Consume only the exact role and NULL value validated by our policy; preserve
// every other unhandled critical extension for x509.Verify to reject.
func appleRoleCertificate(cert *x509.Certificate, role asn1.ObjectIdentifier) (*x509.Certificate, error) {
	matched := false
	for _, extension := range cert.Extensions {
		if extension.Id.Equal(role) && bytes.Equal(extension.Value, []byte{5, 0}) {
			matched = true
		}
	}
	if !matched {
		return nil, fmt.Errorf("missing or invalid Apple certificate role %s", role)
	}
	copy := *cert
	copy.UnhandledCriticalExtensions = nil
	for _, oid := range cert.UnhandledCriticalExtensions {
		if !oid.Equal(role) {
			copy.UnhandledCriticalExtensions = append(copy.UnhandledCriticalExtensions, oid)
		}
	}
	return &copy, nil
}

func verifyAppleBundleSignature(app, executable string, certificate *x509.Certificate, macOS bool) error {
	path := filepath.Join(app, executable)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	blobs, offset, err := appleCodeBlobs(data)
	if err != nil {
		return err
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
	if signer := cms.GetOnlySigner(); signer == nil || (certificate != nil && !signer.Equal(certificate)) {
		return fmt.Errorf("unexpected Apple signature certificate")
	}
	info, err := codesign.ParseSignature(path)
	if err != nil {
		return err
	}
	for _, cd := range info.CodeDirs {
		if macOS && (cd.HashType != 2 || len(cd.SpecialHashes[-1]) != sha256.Size || len(cd.SpecialHashes[-3]) != sha256.Size || bytes.Equal(cd.SpecialHashes[-1], make([]byte, sha256.Size)) || bytes.Equal(cd.SpecialHashes[-3], make([]byte, sha256.Size))) {
			return fmt.Errorf("macOS signature requires SHA256 code, Info.plist and resource seals")
		}
		if cd.CodeLimit != offset || cd.PageSize == 0 || uint64(len(cd.CodeHashes)) != (uint64(cd.CodeLimit)+uint64(cd.PageSize)-1)/uint64(cd.PageSize) {
			return fmt.Errorf("invalid signed code range")
		}
		for i, want := range cd.CodeHashes {
			start := uint64(i) * uint64(cd.PageSize)
			end := min(start+uint64(cd.PageSize), uint64(cd.CodeLimit))
			if start >= end || !appleHashEqual(data[start:end], want, cd.HashType) {
				return fmt.Errorf("Apple code page %d checksum mismatch", i)
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
			if !appleHashEqual(b, want, cd.HashType) {
				return fmt.Errorf("Apple special slot %d checksum mismatch", slot)
			}
		}
	}
	if macOS {
		return nil // Desktop resource sealing is checked by verifyMacOSBundle.
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
		if !ok || !appleHashEqual(b, entry.Hash2, 2) {
			return fmt.Errorf("Apple resource checksum mismatch: %s", rel)
		}
		return nil
	})
}

func appleHashEqual(data, want []byte, kind uint8) bool {
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

func appleCodeBlobs(data []byte) (map[uint32][]byte, uint32, error) {
	m, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
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
		return nil, 0, fmt.Errorf("invalid Mach-O signature bounds")
	}
	super := data[uint64(offset) : uint64(offset)+uint64(size)]
	length := uint64(binary.BigEndian.Uint32(super[4:]))
	if binary.BigEndian.Uint32(super) != 0xfade0cc0 || length < 12 || length > uint64(len(super)) {
		return nil, 0, fmt.Errorf("invalid embedded signature envelope")
	}
	super = super[:length]
	count := binary.BigEndian.Uint32(super[8:])
	if uint64(count)*8+12 > uint64(len(super)) {
		return nil, 0, fmt.Errorf("invalid signature index")
	}
	blobs := map[uint32][]byte{}
	for i := uint32(0); i < count; i++ {
		slot, at := binary.BigEndian.Uint32(super[12+8*i:]), binary.BigEndian.Uint32(super[16+8*i:])
		if uint64(at)+8 > uint64(len(super)) {
			return nil, 0, fmt.Errorf("invalid signature blob")
		}
		n := binary.BigEndian.Uint32(super[at+4:])
		if n < 8 || uint64(at)+uint64(n) > uint64(len(super)) {
			return nil, 0, fmt.Errorf("invalid signature blob size")
		}
		if _, exists := blobs[slot]; exists {
			return nil, 0, fmt.Errorf("duplicate signature slot")
		}
		blobs[slot] = super[at : at+n]
	}
	return blobs, offset, nil
}
