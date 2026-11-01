package x11

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// 无需桌面：独立 helper 经真实 exec 边界模拟 stdout、成功时的 stderr 警告、
// 用户取消及进程错误。
func TestDialogHelperOutputAndErrors(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf 'helper warning\\n' >&2\nprintf '%s\\n' \"$GOUI_DIALOG_TEST_PATH\"\nexit \"$GOUI_DIALOG_TEST_EXIT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "zenity"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "filename with trailing spaces  ")
	t.Setenv("PATH", dir)
	t.Setenv("GOUI_DIALOG_TEST_PATH", path)
	t.Setenv("GOUI_DIALOG_TEST_EXIT", "0")
	output, err := runDialog(dialogZenity, nil)
	if err != nil || output != path {
		t.Fatalf("diagnostic/whitespace corrupted filename: %q %v", output, err)
	}
	t.Setenv("GOUI_DIALOG_TEST_EXIT", "1")
	if _, err := runDialog(dialogZenity, nil); !errors.Is(err, errDialogCancelled) {
		t.Fatal(err)
	}
	t.Setenv("GOUI_DIALOG_TEST_EXIT", "2")
	if _, err := runDialog(dialogZenity, nil); err == nil || errors.Is(err, errDialogCancelled) || !strings.Contains(err.Error(), "helper warning") {
		t.Fatal(err)
	}
}

// 通过真实进程边界验证 helper 协议，不创建原生窗口。
func runDialog(helper dialogHelper, args []string) (string, error) {
	cmd, err := dialogCommand(helper, args)
	if err != nil {
		return "", err
	}
	output, err := cmd.Output()
	var stderr []byte
	if e, ok := err.(*exec.ExitError); ok {
		stderr = e.Stderr
	}
	return dialogOutput(output, stderr, err)
}

func TestDialogOwnerArguments(t *testing.T) {
	for _, test := range []struct {
		helper dialogHelper
		want   []string
	}{
		{dialogZenity, []string{"--attach=1234", "--modal"}},
		{dialogYad, []string{"--attach=1234", "--modal"}},
		{dialogKdialog, []string{"--attach", "1234"}},
	} {
		if got := dialogOwnerArgs(test.helper, 1234); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("helper=%d owner 参数=%v", test.helper, got)
		}
	}
}
