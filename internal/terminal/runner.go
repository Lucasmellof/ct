package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"time"

	ptylib "github.com/aymanbagabas/go-pty"
	"golang.org/x/term"

	"github.com/lucasmellof/ct/internal/highlighter"
)

func Run(command string, args []string) error {
	pty, err := ptylib.New()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	var closePtyOnce sync.Once
	closePty := func() {
		closePtyOnce.Do(func() {
			pty.Close()
		})
	}
	defer closePty()
	terminalFD := int(os.Stdout.Fd())
	width, height := terminalSize(terminalFD)
	if err := pty.Resize(width, height); err != nil {
		return fmt.Errorf("failed to resize terminal: %w", err)
	}

	raw, err := EnterRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to enter raw mode: %w", err)
	}

	defer raw.Restore()

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	cmd := pty.CommandContext(signalCtx, command, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start program: %w", err)
	}

	resizeCtx, stopResize := context.WithCancel(signalCtx)
	defer stopResize()

	go resize(pty, resizeCtx, terminalFD)

	go func() {
		_, _ = io.Copy(pty, os.Stdin)
	}()

	highlighter := highlighter.NewHighlighter()

	outputDone := make(chan error, 1)

	go func() {
		err := copyHighlighted(os.Stdout, pty, highlighter)
		outputDone <- err
	}()

	waitErr := cmd.Wait()
	stopResize()

	select {
	case <-outputDone:
	case <-time.After(500 * time.Millisecond):
		closePty()
		<-outputDone
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		_, _ = fmt.Fprint(os.Stdout, "\x1b[0m")
	}

	if waitErr != nil {
		return waitErr
	}
	return nil
}
