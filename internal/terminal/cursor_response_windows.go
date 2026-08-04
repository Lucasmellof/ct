//go:build windows

package terminal

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/sys/windows"
)

var cursorPositionQuery = []byte("\x1b[6n")

type cursorResponseReader struct {
	reader     io.Reader
	response   io.Writer
	terminalFD int
	carry      []byte
	pending    []byte
}

func (r *cursorResponseReader) Read(data []byte) (int, error) {
	for len(r.pending) == 0 {
		buffer := make([]byte, len(data))
		n, err := r.reader.Read(buffer)
		if n > 0 {
			r.consume(buffer[:n])
			if len(r.pending) > 0 {
				break
			}
		}
		if err != nil {
			return 0, err
		}
	}

	n := copy(data, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *cursorResponseReader) consume(data []byte) {
	data = append(r.carry, data...)
	r.carry = nil

	for len(data) > 0 {
		if bytes.HasPrefix(data, cursorPositionQuery) {
			r.replyCursorPosition()
			data = data[len(cursorPositionQuery):]
			continue
		}
		if bytes.HasPrefix(cursorPositionQuery, data) {
			r.carry = append(r.carry, data...)
			return
		}
		r.pending = append(r.pending, data[0])
		data = data[1:]
	}
}

func (r *cursorResponseReader) replyCursorPosition() {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(r.terminalFD), &info); err != nil {
		return
	}
	_, _ = fmt.Fprintf(r.response, "\x1b[%d;%dR", int(info.CursorPosition.Y)+1, int(info.CursorPosition.X)+1)
}
