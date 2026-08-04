//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

func disableVirtualTerminalInput(fd int) error {
	handle := windows.Handle(fd)
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return err
	}
	return windows.SetConsoleMode(handle, mode&^windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
}

func EnableVirtualTerminal() error {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return err
	}
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(handle, mode); err != nil {
		return err
	}
	return nil
}
