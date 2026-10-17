package modern_test

import (
	"os/exec"
	"testing"
)

func TestThirdPartyRegistrationStartup(t *testing.T) {
	cmd := exec.Command("go", "run", "./testdata/registration")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registration startup: %v\n%s", err, out)
	}
}
