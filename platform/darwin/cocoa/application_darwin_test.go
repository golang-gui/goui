package cocoa

import (
	"testing"
)

func TestBundleIdentity(t *testing.T) {
	for _, tc := range []struct {
		requested, bundle string
		app, valid        bool
	}{
		{"", "", false, true},
		{"", "com.example.Editor", true, true},
		{"com.example.Editor", "", false, true},
		{"com.example.Editor", "com.example.Editor", true, true},
		{"com.example.Editor", "com.example.Other", true, false},
		{"com.example.Editor", "", true, false},
		{"com.example.Editor", "com.example.Other", false, false},
	} {
		if err := validateBundleIdentity(tc.requested, tc.bundle, tc.app); (err == nil) != tc.valid {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}
