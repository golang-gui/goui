package main

import (
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/androidtext"
)

func newTypography() (typography.Context, error) { return androidtext.NewContext() }
