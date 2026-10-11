package security

import (
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

type SecTaskRef = CFTypeRef

var (
	fnSecTaskCreateFromSelf          func(allocator CFTypeRef) SecTaskRef
	fnSecTaskCopyValueForEntitlement func(task SecTaskRef, entitlement CFStringRef, err *CFErrorRef) CFTypeRef
)

func InitSecurity() error {
	framework, err := utils.LoadSystemFramework("Security")
	if err != nil {
		return err
	}
	return framework.LoadFunctions([]utils.Function{
		{Name: "SecTaskCreateFromSelf", PFunc: &fnSecTaskCreateFromSelf},
		{Name: "SecTaskCopyValueForEntitlement", PFunc: &fnSecTaskCopyValueForEntitlement},
	})
}

// SecTaskCreateFromSelf returns an owned reference. Release it with CFRelease.
func SecTaskCreateFromSelf(allocator CFTypeRef) SecTaskRef { return fnSecTaskCreateFromSelf(allocator) }

// SecTaskCopyValueForEntitlement returns an owned value/error. An absent
// entitlement returns zero with no error; no sandbox policy is applied here.
func SecTaskCopyValueForEntitlement(task SecTaskRef, entitlement CFStringRef, err *CFErrorRef) CFTypeRef {
	return fnSecTaskCopyValueForEntitlement(task, entitlement, err)
}
