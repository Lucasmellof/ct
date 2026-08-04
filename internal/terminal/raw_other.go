//go:build !windows

package terminal

func disableVirtualTerminalInput(int) error {
	return nil
}

func EnableVirtualTerminal() error {
	return nil
}
