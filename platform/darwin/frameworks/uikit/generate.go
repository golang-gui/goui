//go:build ignore

package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("platform/darwin/frameworks/uikit", map[string]string{
		"protocols.m": `#import <UIKit/UIKit.h>
// A Go-created class does not cause Clang to emit SDK protocol definitions.
// Retain only standard Objective-C metadata, without any host implementation.
__attribute__((constructor)) static void referenceProtocols(void) {
    Protocol * volatile protocols[] = {
        @protocol(UIApplicationDelegate),
        @protocol(UIWindowSceneDelegate),
    };
    (void)protocols;
}
`,
	})
}
