package test

import (
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"
)

func newContext() (typography.Context, error) { return directwrite.NewContext() }
