package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"

	"github.com/lucasmellof/ct/internal/highlighter"
	"github.com/rurreac/conpty"
)

func Run(command string, args []string, highlighter *highlighter.Highlighter) error {
	if highlighter == nil {
		return fmt.Errorf("highlighter is nil")
	}
	terminalFD := int(os.Stdout.Fd())
	width, height := terminalSize(terminalFD)

	raw, err := EnterRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to enter raw mode: %w", err)
	}

	defer raw.Restore()

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	commandLine := windows.ComposeCommandLine(append([]string{command}, args...))
	pty, err := conpty.StartConPty(commandLine, width, height, os.Environ(), conpty.WithInheritCursor(true))
	if err != nil {
		return fmt.Errorf("failed to start ConPTY program: %w", err)
	}
	var closePtyOnce sync.Once
	closePty := func() {
		closePtyOnce.Do(func() {
			_ = pty.Close()
		})
	}
	defer closePty()

	resizeCtx, stopResize := context.WithCancel(signalCtx)
	defer stopResize()

	go resize(pty, resizeCtx, terminalFD)

	input := &inputTracker{}
	go func() {
		stdin := trackingReader{reader: newConsoleInputReader(os.Stdin), tracker: input}
		_, _ = io.Copy(pty, stdin)
	}()

	outputDone := make(chan error, 1)

	go func() {
		output := os.Stdout
		source := &cursorResponseReader{reader: pty, response: pty, terminalFD: terminalFD}
		outputDone <- copyHighlightedInteractive(output, source, highlighter, input)
	}()

	_, waitErr := pty.Wait(signalCtx)
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
