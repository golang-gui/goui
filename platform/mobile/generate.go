//go:build ignore

package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("platform/mobile", map[string]string{
		"ios/main.c": `extern int goui_run(void);
int main(int argc, char **argv) {
    (void)argc; (void)argv;
    return goui_run();
}
`,
	})
}
