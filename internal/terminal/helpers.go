package terminal

import (
	"context"
	"time"

	ptylib "github.com/aymanbagabas/go-pty"
	"golang.org/x/term"
)

func terminalSize(fd int) (int, int) {
	width, height, err := term.GetSize(fd)

	if err != nil || width <= 0 || height <= 0 {
		return 80, 24
	}

	return width, height
}

func resize(p ptylib.Pty, stopResize context.Context, terminalFD int) {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	lastWidth, lastHeight := terminalSize(terminalFD)

	for {
		select {
		case <-stopResize.Done():
			return
		case <-ticker.C:
			width, height := terminalSize(terminalFD)

			if width == lastWidth && height == lastHeight {
				continue
			}
			if err := p.Resize(width, height); err == nil {
				lastWidth, lastHeight = width, height
			}
		}
	}
}
