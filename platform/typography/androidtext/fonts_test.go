package androidtext

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

func TestFontFaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	faces, err := fontFaces(path)
	if err != nil || len(faces) != 1 || faces[0].family != "Go" || faces[0].index != 0 || faces[0].width != 5 {
		t.Fatalf("字体族元数据错误: %+v %v", faces, err)
	}
	// TTC 各目录中的偏移以整个文件为基准；两个 face 必须都能取得族名。
	fonts := [][]byte{goregular.TTF, gobold.TTF}
	data := make([]byte, 12+len(fonts)*4)
	copy(data, "ttcf")
	binary.BigEndian.PutUint32(data[4:8], 0x10000)
	binary.BigEndian.PutUint32(data[8:12], uint32(len(fonts)))
	for i, font := range fonts {
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		base := len(data)
		binary.BigEndian.PutUint32(data[12+i*4:16+i*4], uint32(base))
		data = append(data, font...)
		for j := 0; j < int(binary.BigEndian.Uint16(font[4:6])); j++ {
			at := base + 12 + j*16 + 8
			binary.BigEndian.PutUint32(data[at:at+4], binary.BigEndian.Uint32(data[at:at+4])+uint32(base))
		}
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	faces, err = fontFaces(path)
	if err != nil || len(faces) != 2 || faces[0].family != "Go" || faces[1].family != "Go" || faces[1].index != 1 {
		t.Fatalf("TTC 字体元数据错误: %+v %v", faces, err)
	}
}

func TestFontFacesInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.ttf")
	for _, data := range [][]byte{nil, []byte("not a font"), goregular.TTF[:20]} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := fontFaces(path); err == nil {
			t.Fatal("接受了无效字体")
		}
	}
}
