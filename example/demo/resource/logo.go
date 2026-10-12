package resource

import _ "embed"

// These embedded copies match doc/logo exactly. Embedding keeps the demo
// independent of its working directory and of the source tree at runtime.

//go:embed goui-mark.svg
var logoMarkSVG string

//go:embed goui-logo.svg
var logoSVG string

func LogoMarkSVG() string { return logoMarkSVG }
func LogoSVG() string     { return logoSVG }
