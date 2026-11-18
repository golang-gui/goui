package main

import (
	"fmt"

	"github.com/golang-gui/goui/platform/dragdrop"
)

// 只检查公开契约；实际释放坐标仍需结合桌面输入验证。
func verifyResult(expect string, localTarget, destroyed bool, begins, ends, drops int, accepted bool, result dragdrop.Result) error {
	if begins != 1 || ends != 1 {
		return fmt.Errorf("expected one Begin/End; got %d/%d", begins, ends)
	}
	if err := result.Validate(); err != nil {
		return err
	}
	if destroyed && result.PositionValid {
		return fmt.Errorf("destroyed source returned a valid position")
	}
	if accepted && (result.Action != dragdrop.Copy || result.Canceled || result.Err != nil) {
		return fmt.Errorf("target accepted payload but source reported %+v", result)
	}
	switch expect {
	case "accepted":
		if result.Action != dragdrop.Copy || (localTarget && (!accepted || drops != 1)) {
			return fmt.Errorf("expected Copy and one successful local drop: %+v drops=%d accepted=%t", result, drops, accepted)
		}
	case "unaccepted":
		if result.Action != 0 || result.Canceled || result.Err != nil || accepted {
			return fmt.Errorf("expected unaccepted release, got %+v", result)
		}
	case "canceled":
		if !result.Canceled || drops != 0 {
			return fmt.Errorf("expected cancellation without Drop, got %+v drops=%d", result, drops)
		}
	case "error":
		if result.Err == nil {
			return fmt.Errorf("expected error, got %+v", result)
		}
	default:
		return fmt.Errorf("unknown expectation %q", expect)
	}
	if !destroyed && !result.Canceled && result.Err == nil && !result.PositionValid {
		return fmt.Errorf("normal completion has no release position")
	}
	return nil
}
