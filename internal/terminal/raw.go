package terminal

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

type RawMode struct {
	fd       int
	oldState *term.State

	restoreOnce sync.Once
	restoreErr  error
}

func EnterRaw(file *os.File) (*RawMode, error) {
	if file == nil {
		return nil, fmt.Errorf("file is nil")
	}

	fd := int(file.Fd())

	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("file is not a terminal")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to set terminal raw mode: %w", err)
	}
	if err := disableVirtualTerminalInput(fd); err != nil {
		_ = term.Restore(fd, oldState)
		return nil, fmt.Errorf("failed to configure console input: %w", err)
	}

	return &RawMode{
		fd:       fd,
		oldState: oldState,
	}, nil
}

func (r *RawMode) Restore() error {
	if r == nil || r.oldState == nil {
		return fmt.Errorf("raw mode is not initialized")
	}

	r.restoreOnce.Do(func() {
		r.restoreErr = term.Restore(r.fd, r.oldState)
	})

	if r.restoreErr != nil {
		return fmt.Errorf("failed to restore terminal state: %w", r.restoreErr)
	}

	return nil
}
