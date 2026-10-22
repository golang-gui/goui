package resource

import (
	"os"
	"path/filepath"
	"testing"
)

// go:embed cannot read a parent directory. The demo's copied resource must
// remain byte-for-byte identical to its design source.
func TestEmbeddedLogosMatchDesigns(t *testing.T) {
	for name, embedded := range map[string]string{
		"goui-mark.svg": LogoMarkSVG(),
		"goui-logo.svg": LogoSVG(),
	} {
		t.Run(name, func(t *testing.T) {
			original, err := os.ReadFile(filepath.Join("..", "..", "..", "doc", "logo", name))
			if err != nil {
				t.Fatal(err)
			}
			if string(original) != embedded {
				t.Fatalf("embedded %s differs from doc/logo; synchronize the resource copy", name)
			}
		})
	}
}
