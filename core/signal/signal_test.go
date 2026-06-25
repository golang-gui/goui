package signal

import "testing"

func TestSignal0Emit(t *testing.T) {
	var sig Signal0
	count := 0

	sig.Connect(func() {
		count++
	})
	sig.Connect(func() {
		count++
	})

	sig.Emit()
	if count != 2 {
		t.Fatalf("expected 2 calls, got %d", count)
	}
}

func TestSignal0Disconnect(t *testing.T) {
	var sig Signal0
	count := 0

	handle := sig.Connect(func() {
		count++
	})

	sig.Emit()
	handle.Disconnect()
	handle.Disconnect()
	sig.Emit()

	if count != 1 {
		t.Fatalf("expected 1 call after disconnect, got %d", count)
	}
}

func TestSignal0Block(t *testing.T) {
	var sig Signal0
	count := 0

	handle := sig.Connect(func() {
		count++
	})

	handle.Block()
	sig.Emit()
	if count != 0 {
		t.Fatalf("expected 0 calls while blocked, got %d", count)
	}

	handle.Unblock()
	sig.Emit()
	if count != 1 {
		t.Fatalf("expected 1 call after unblock, got %d", count)
	}
}

func TestSignal0NestedBlock(t *testing.T) {
	var sig Signal0
	count := 0

	handle := sig.Connect(func() {
		count++
	})

	handle.Block()
	handle.Block()
	handle.Unblock()
	sig.Emit()
	if count != 0 {
		t.Fatalf("expected 0 calls while still nested-blocked, got %d", count)
	}

	handle.Unblock()
	sig.Emit()
	if count != 1 {
		t.Fatalf("expected 1 call after nested unblock, got %d", count)
	}

	handle.Unblock()
	sig.Emit()
	if count != 2 {
		t.Fatalf("expected extra unblock to be ignored, got %d calls", count)
	}
}

func TestSignal0DisconnectDuringEmit(t *testing.T) {
	var sig Signal0
	var handle Handle
	count := 0

	handle = sig.Connect(func() {
		count++
		handle.Disconnect()
	})
	sig.Connect(func() {
		count++
	})

	sig.Emit()
	if count != 2 {
		t.Fatalf("expected 2 calls on first emit, got %d", count)
	}

	count = 0
	sig.Emit()
	if count != 1 {
		t.Fatalf("expected 1 call after disconnect during emit, got %d", count)
	}
}

func TestSignalDisconnectSkipsPendingSlot(t *testing.T) {
	var sig Signal0
	called := false

	var second Handle
	sig.Connect(func() {
		second.Disconnect()
	})
	second = sig.Connect(func() {
		called = true
	})

	sig.Emit()
	if called {
		t.Fatal("disconnected pending slot was called")
	}
}

func TestSignal1Emit(t *testing.T) {
	var sig Signal1[int]
	got := 0

	sig.Connect(func(v int) {
		got = v
	})
	sig.Emit(42)

	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestSignal2Emit(t *testing.T) {
	var sig Signal2[string, int]
	gotText := ""
	gotNumber := 0

	sig.Connect(func(text string, number int) {
		gotText = text
		gotNumber = number
	})
	sig.Emit("value", 7)

	if gotText != "value" || gotNumber != 7 {
		t.Fatalf("expected value and 7, got %q and %d", gotText, gotNumber)
	}
}

func TestSignal3Emit(t *testing.T) {
	var sig Signal3[int, string, bool]
	gotNumber := 0
	gotText := ""
	gotFlag := false

	sig.Connect(func(number int, text string, flag bool) {
		gotNumber = number
		gotText = text
		gotFlag = flag
	})
	sig.Emit(1, "ok", true)

	if gotNumber != 1 || gotText != "ok" || !gotFlag {
		t.Fatalf("unexpected signal values: %d %q %v", gotNumber, gotText, gotFlag)
	}
}

func TestHandles(t *testing.T) {
	var sig Signal0
	count := 0

	handles := Handles{
		sig.Connect(func() {
			count++
		}),
		sig.Connect(func() {
			count++
		}),
	}

	handles.Block()
	sig.Emit()
	if count != 0 {
		t.Fatalf("expected 0 calls while handles blocked, got %d", count)
	}

	handles.Unblock()
	sig.Emit()
	if count != 2 {
		t.Fatalf("expected 2 calls after handles unblocked, got %d", count)
	}

	handles.Disconnect()
	sig.Emit()
	if count != 2 {
		t.Fatalf("expected no extra calls after handles disconnected, got %d", count)
	}
}

func TestSignal0EmitNoSlots(t *testing.T) {
	var sig Signal0
	sig.Emit()
}
