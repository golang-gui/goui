package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCommandArguments(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "usage:"},
		{[]string{"unknown"}, "unknown command"},
		{[]string{"driver"}, "requires -o"},
		{[]string{"export", "."}, "requires -o"},
		{[]string{"verify"}, "requires a base.json"},
		{[]string{"verify", "one", "two"}, "requires a base.json"},
		{[]string{"link-inputs"}, "requires -config"},
		{[]string{"link-tools"}, "requires -config"},
		{[]string{"build"}, "requires one main package"},
		{[]string{"build", "."}, "requires output"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			var output bytes.Buffer
			err := run(context.Background(), test.args, &output, &output)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want=%s", err, test.want)
			}
		})
	}
}
