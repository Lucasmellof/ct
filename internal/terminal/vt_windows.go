//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

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
