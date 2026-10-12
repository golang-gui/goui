package resource

import (
	"os"
	"path/filepath"
	"testing"
)

// go:embed 不能读取父目录；复制到例程的资源须与设计源文件逐字节一致。
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
