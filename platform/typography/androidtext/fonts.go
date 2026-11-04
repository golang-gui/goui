package androidtext

import (
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/image/font/sfnt"
)

type fontFace struct {
	family       string
	index, width int32
}

func fontFaces(path string) ([]fontFace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	if collection.NumFonts() > 4096 {
		return nil, fmt.Errorf("androidtext: font collection exceeds 4096 faces")
	}
	var buffer sfnt.Buffer
	result := make([]fontFace, 0, collection.NumFonts())
	for i := 0; i < collection.NumFonts(); i++ {
		font, err := collection.Font(i)
		if err != nil {
			return nil, err
		}
		family, err := font.Name(&buffer, sfnt.NameIDTypographicFamily)
		if err != nil || family == "" {
			family, err = font.Name(&buffer, sfnt.NameIDFamily)
		}
		if err != nil || family == "" {
			return nil, fmt.Errorf("androidtext: font face %d has no family", i)
		}
		base := uint64(0)
		if string(data[:4]) == "ttcf" {
			base = uint64(binary.BigEndian.Uint32(data[12+i*4 : 16+i*4]))
		}
		if base+12 > uint64(len(data)) {
			return nil, fmt.Errorf("androidtext: invalid font directory")
		}
		count := int(binary.BigEndian.Uint16(data[base+4 : base+6]))
		width := int32(5)
		for table := 0; table < count; table++ {
			at := base + 12 + uint64(table)*16
			if at+16 > uint64(len(data)) {
				return nil, fmt.Errorf("androidtext: invalid font table")
			}
			if string(data[at:at+4]) != "OS/2" {
				continue
			}
			offset := uint64(binary.BigEndian.Uint32(data[at+8 : at+12]))
			length := uint64(binary.BigEndian.Uint32(data[at+12 : at+16]))
			if length < 8 || offset+length > uint64(len(data)) {
				return nil, fmt.Errorf("androidtext: invalid OS/2 table")
			}
			value := int32(binary.BigEndian.Uint16(data[offset+6 : offset+8]))
			if value >= 1 && value <= 9 {
				width = value
			}
		}
		result = append(result, fontFace{family, int32(i), width})
	}
	return result, nil
}
