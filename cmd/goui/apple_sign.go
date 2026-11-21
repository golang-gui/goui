package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"os"
	"slices"

	"github.com/aluedeke/go-codesign/pkg/codesign"
	gomacho "github.com/blacktop/go-macho"
	machosign "github.com/blacktop/go-macho/pkg/codesign"
	machotypes "github.com/blacktop/go-macho/pkg/codesign/types"
	"github.com/digitorus/pkcs7"
	appleplist "howett.net/plist"
)

// signAppleMachO shares binary signing mechanics; callers own platform policy and bundle layout.
func signAppleMachO(path, id string, info, resources, entitlements []byte, identity *codesign.SigningIdentity, runtime bool, stamp func([]byte) ([]byte, error)) error {
	ent, err := codesign.ParseEntitlementsXML(entitlements)
	if err != nil {
		return err
	}
	der, err := codesign.EntitlementsToDER(ent)
	if err != nil {
		return err
	}
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
		return fmt.Errorf("Apple signing requires a linked ARM64 executable with a signature slot")
	}
	headerEnd := uint64(32) + uint64(binary.LittleEndian.Uint32(before[20:]))
	signatureOffset := uint64(m.CodeSignature().Offset)
	hash := sha256.Sum256(resources)
	config := &machosign.Config{ID: id, TeamID: identity.TeamID, InfoPlist: info, Entitlements: entitlements, EntitlementsDER: der, ResourceDirSlotHash: hash[:], CertChain: identity.CertChain, SpecialSlots: []machotypes.SpecialSlot{}}
	if runtime {
		config.Flags = machotypes.RUNTIME
		config.CertChain = append([]*x509.Certificate(nil), identity.CertChain...)
		slices.Reverse(config.CertChain)
	}
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
		if stamp != nil {
			si := &signed.GetSignedData().SignerInfos[0]
			token, err := stamp(si.EncryptedDigest)
			if err != nil {
				return nil, err
			}
			if err := si.SetUnauthenticatedAttributes([]pkcs7.Attribute{{Type: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}, Value: asn1.RawValue{FullBytes: token}}}); err != nil {
				return nil, err
			}
		}
		signed.Detach()
		return signed.Finish()
	}

	// Sign file offsets directly. Debug-only segments such as __DWARF have no
	// mapped virtual range and must never be reconstructed through VM readers.
	if headerEnd > signatureOffset || signatureOffset > uint64(len(before)) || signatureOffset < 32 {
		return fmt.Errorf("invalid Mach-O load commands or signature offset")
	}
	text, linkedit := m.Segment("__TEXT"), m.Segment("__LINKEDIT")
	if text == nil || linkedit == nil || linkedit.Offset > signatureOffset {
		return fmt.Errorf("missing or invalid Mach-O signing segments")
	}
	config.IsMain = true
	config.TextOffset, config.TextSize = text.Offset, text.Filesz
	config.CodeSize = signatureOffset
	config.NSpecialSlots = 7
	config.InitSlotHashes()
	config.SlotHashes.ResourceDir = hash[:]
	if runtime {
		if versions := m.BuildVersions(); len(versions) > 0 {
			config.RuntimeVersion = versions[0].Sdk
		} else if version := m.VersionMin(); version != nil {
			config.RuntimeVersion = version.Sdk
		}
		if config.RuntimeVersion == 0 {
			return fmt.Errorf("runtime signature requires Mach-O SDK metadata")
		}
	}
	align := func(n, a uint64) uint64 { return (n + a - 1) &^ (a - 1) }
	linkSize := align(signatureOffset-linkedit.Offset+machosign.EstimateCodeSignatureSize(config), 0x4000)
	signatureSize := linkedit.Offset + linkSize - signatureOffset
	if signatureSize > 1<<32-1 || linkedit.Offset+linkSize > uint64(int(^uint(0)>>1)) {
		return fmt.Errorf("Mach-O signature exceeds supported bounds")
	}
	data := bytes.Clone(before[:signatureOffset])
	foundLink, foundSignature := false, false
	for at := uint64(32); at < headerEnd; {
		if at+8 > headerEnd {
			return fmt.Errorf("invalid Mach-O load command")
		}
		kind, size := binary.LittleEndian.Uint32(data[at:]), uint64(binary.LittleEndian.Uint32(data[at+4:]))
		if size < 8 || at+size > headerEnd {
			return fmt.Errorf("invalid Mach-O load command size")
		}
		if kind == 0x19 && size >= 72 && string(bytes.TrimRight(data[at+8:at+24], "\x00")) == "__LINKEDIT" {
			binary.LittleEndian.PutUint64(data[at+32:], align(linkSize, 0x8000))
			binary.LittleEndian.PutUint64(data[at+48:], linkSize)
			foundLink = true
		}
		if kind == 0x1d && size == 16 {
			binary.LittleEndian.PutUint32(data[at+12:], uint32(signatureSize))
			foundSignature = true
		}
		at += size
	}
	if !foundLink || !foundSignature {
		return fmt.Errorf("missing Mach-O signature load commands")
	}
	signature, err := machosign.Sign(bytes.NewReader(data), config)
	if err != nil {
		return err
	}
	if uint64(len(signature)) > signatureSize {
		return fmt.Errorf("CMS signature exceeds reserved signing space")
	}
	output := append(data, signature...)
	output = append(output, make([]byte, signatureSize-uint64(len(signature)))...)
	if !bytes.Equal(before[headerEnd:signatureOffset], output[headerEnd:signatureOffset]) {
		return fmt.Errorf("Apple signer changed executable data outside load commands and signature")
	}
	return os.WriteFile(path, output, 0755)
}
