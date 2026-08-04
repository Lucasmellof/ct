//go:build !windows

package terminal

import (
	"io"
	"os"
)

func newConsoleInputReader(file *os.File) io.Reader {
	return file
}
