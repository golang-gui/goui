package dragdrop

import (
	"bytes"
	"image"
	"math"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestDataCopyAndFormats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "中文 a%.txt")
	uri, err := FileURLFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	paths, urls, payload := []string{path}, []string{uri}, []byte{0, 1, 2}
	data := new(Data)
	data.SetText("")
	data.SetFiles(paths)
	data.SetURLs(urls)
	data.SetBytes("Application/X-Test; B=2; A=1", payload)
	paths[0] = "changed"
	urls[0] = "changed"
	payload[0] = 9
	if err := data.Validate(); err != nil {
		t.Fatal(err)
	}
	if text, ok := data.Text(); !ok || text != "" {
		t.Fatalf("empty text: %q %t", text, ok)
	}
	if got, ok := data.Files(); !ok || !reflect.DeepEqual(got, []string{path}) {
		t.Fatalf("files: %v %t", got, ok)
	}
	if got, ok := data.URLs(); !ok || !reflect.DeepEqual(got, []string{uri}) {
		t.Fatalf("URLs: %v %t", got, ok)
	}
	if got, ok := data.Bytes("application/x-test; a=1; b=2"); !ok || !bytes.Equal(got, []byte{0, 1, 2}) {
		t.Fatalf("MIME bytes: %v %t", got, ok)
	}
	copy := data.Clone()
	data.SetBytes("application/x-test; a=1; b=2", []byte{7})
	if got, _ := copy.Bytes("application/x-test; a=1; b=2"); !bytes.Equal(got, []byte{0, 1, 2}) {
		t.Fatalf("clone changed with original: %v", got)
	}
	if got := copy.Formats(); !reflect.DeepEqual(got, []Format{FormatText, FormatFiles, FormatURLs, MIMEFormat("application/x-test; a=1; b=2")}) {
		t.Fatalf("formats: %v", got)
	}
}

func TestDataValidation(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "file")
	uri, err := FileURLFromPath(absolute)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data *Data
	}{
		{"invalid UTF-8", &Data{text: ptr("\xff")}},
		{"NUL text", &Data{text: ptr("a\x00b")}},
		{"empty files", &Data{files: []string{}}},
		{"relative path", &Data{files: []string{"relative"}}},
		{"NUL path", &Data{files: []string{absolute + "\x00"}}},
		{"empty URLs", &Data{urls: []string{}}},
		{"relative URI", &Data{urls: []string{"path"}}},
		{"NUL URL", &Data{urls: []string{"https://host/\x00"}}},
		{"reserved MIME", &Data{mimes: map[string][]byte{"text/plain": {1}}}},
		{"invalid MIME", &Data{mimes: map[string][]byte{"not a mime": {1}}}},
		{"oversized bytes", &Data{mimes: map[string][]byte{"application/x-test": make([]byte, MaxDataBytes+1)}}},
		{"conflicting representations", &Data{files: []string{absolute}, urls: []string{"https://example.test"}}},
		{"wrong file URI", &Data{files: []string{absolute + "2"}, urls: []string{uri}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.data.Validate(); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
	if err := (*Data)(nil).Validate(); err != nil {
		t.Fatalf("local-only payload: %v", err)
	}
}

func ptr(s string) *string { return &s }

func TestURIListAndFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "中 文#%.txt")
	uri, err := FileURLFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(uri, " ") || !strings.Contains(uri, "%23%25") {
		t.Fatalf("not URL encoded: %q", uri)
	}
	gotPath, err := FilePathFromURL(uri)
	if err != nil || gotPath != path {
		t.Fatalf("round trip: %q %v", gotPath, err)
	}
	encoded, err := EncodeURIList([]string{uri, "https://example.test/%E4%B8%AD"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(encoded, []byte("\r\n")) {
		t.Fatalf("missing CRLF: %q", encoded)
	}
	decoded, err := DecodeURIList(append([]byte("# comment\r\n\r\n"), encoded...))
	if err != nil || !reflect.DeepEqual(decoded, []string{uri, "https://example.test/%E4%B8%AD"}) {
		t.Fatalf("decoded %v, %v", decoded, err)
	}
	for _, input := range [][]byte{[]byte("relative\r\n"), []byte("https://a/\x00\r\n"), []byte("https://a/\xff\r\n")} {
		if _, err := DecodeURIList(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	if runtime.GOOS != "windows" {
		if _, err := FilePathFromURL("file://remote.example/tmp/file"); err == nil {
			t.Fatal("accepted remote file URI as local path")
		}
	}
}

func TestActions(t *testing.T) {
	if !(Copy | Move | Link).ValidSet() || (Action(8)).ValidSet() || (Copy | Move).ValidResult() {
		t.Fatal("action set/result validation disagrees with contract")
	}
	if err := (Result{Action: Copy, Canceled: true}).Validate(); err == nil {
		t.Fatal("accepted completed and canceled result")
	}
}

func TestPreviewValidation(t *testing.T) {
	for _, preview := range []Preview{
		{Scale: -1},
		{Scale: float32(math.NaN())},
		{Image: image.NewRGBA(image.Rect(0, 0, 0, 1))},
		{Image: image.NewRGBA(image.Rect(0, 0, 16385, 1))},
	} {
		if err := preview.Validate(); err == nil {
			t.Fatalf("accepted invalid preview %+v", preview)
		}
	}
	if err := (Preview{Image: image.NewRGBA(image.Rect(4, 5, 12, 13))}).Validate(); err != nil {
		t.Fatal(err)
	}
}
