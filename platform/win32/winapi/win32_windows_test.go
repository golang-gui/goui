package winapi

import (
	"syscall"
	"testing"
)

func Test_Win32GUI(t *testing.T) {
	hInstance, err := GetModuleHandle(nil)
	if err != nil {
		t.Fatal(err)
	}

	//wndTitle, err := syscall.UTF16PtrFromString("TestWindow")

	clsName, err := syscall.UTF16PtrFromString("TestWindowClass")

	wndCls := WNDCLASSEX{
		Size:       Sizeof_WNDCLASSEX,
		ClassName:  clsName,
		Instance:   hInstance,
		Style:      CS_HREDRAW | CS_VREDRAW,
		Background: HBRUSH(6),
		WndProc: MakeWindowProc(func(wnd HWND, message UINT, wParam WPARAM, lParam LPARAM) LRESULT {
			switch message {
			case WM_PAINT:
				paint := PAINTSTRUCT{}
				BeginPaint(wnd, &paint)
				EndPaint(wnd, &paint)
				return 0

			case WM_DESTROY:
				PostQuitMessage(0)
				return 0
			}
			return DefWindowProc(wnd, message, wParam, lParam)
		}),
	}

	_, err = RegisterClassEx(&wndCls)
	if err != nil {
		t.Fatal(err)
	}

	wnd, err := CreateWindowEx(0, clsName, nil, WS_OVERLAPPEDWINDOW, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, 0, 0, hInstance, nil)
	if err != nil {
		t.Fatal(err)
	}

	wndTitle, _ := syscall.UTF16PtrFromString("Parent Window")
	SetWindowText(wnd, wndTitle)

	child, err := CreateWindowEx(0, clsName, nil, WS_OVERLAPPEDWINDOW, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, wnd, 0, hInstance, nil)
	if err != nil {
		t.Fatal(err)
	}

	childTitle, _ := syscall.UTF16PtrFromString("Child Window")
	SetWindowText(child, childTitle)

	ShowWindow(wnd, SW_NORMAL)
	UpdateWindow(wnd)

	ShowWindow(child, SW_NORMAL)
	UpdateWindow(child)

	for {
		var msg MSG
		ret, err := GetMessage(&msg, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}

		if ret == FALSE {
			return
		}

		TranslateMessage(&msg)
		DispatchMessage(&msg)
	}
}
