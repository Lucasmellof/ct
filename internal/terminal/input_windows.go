//go:build windows

package terminal

import (
	"io"
	"os"
	"syscall"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	keyEvent = 0x0001

	leftAltPressed  = 0x0002
	rightAltPressed = 0x0001

	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0d
	vkEscape = 0x1b
	vkPrior  = 0x21
	vkNext   = 0x22
	vkEnd    = 0x23
	vkHome   = 0x24
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkInsert = 0x2d
	vkDelete = 0x2e
)

var readConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

type consoleInputRecord struct {
	eventType uint16
	_         uint16
	key       consoleKeyEvent
}

type consoleKeyEvent struct {
	keyDown         uint32
	repeatCount     uint16
	virtualKeyCode  uint16
	virtualScanCode uint16
	unicodeChar     uint16
	controlKeyState uint32
}

type consoleInputReader struct {
	handle  windows.Handle
	pending []byte
}

func newConsoleInputReader(file *os.File) io.Reader {
	if file == nil || !isConsoleInput(int(file.Fd())) {
		return file
	}
	return &consoleInputReader{handle: windows.Handle(file.Fd())}
}

func (r *consoleInputReader) Read(data []byte) (int, error) {
	for len(r.pending) == 0 {
		var record consoleInputRecord
		var read uint32
		ok, _, callErr := readConsoleInput.Call(
			uintptr(r.handle),
			uintptr(unsafe.Pointer(&record)),
			1,
			uintptr(unsafe.Pointer(&read)),
		)
		if ok == 0 {
			if callErr != syscall.Errno(0) {
				return 0, callErr
			}
			return 0, syscall.EINVAL
		}
		if read == 0 || record.eventType != keyEvent || record.key.keyDown == 0 {
			continue
		}
		r.pending = append(r.pending, encodeConsoleKey(record.key)...)
	}

	n := copy(data, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func isConsoleInput(fd int) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(fd), &mode) == nil
}

func encodeConsoleKey(key consoleKeyEvent) []byte {
	repeats := int(key.repeatCount)
	if repeats == 0 {
		repeats = 1
	}

	var value []byte
	if key.unicodeChar != 0 {
		if key.controlKeyState&(leftAltPressed|rightAltPressed) != 0 {
			value = append(value, '\x1b')
		}
		value = utf8.AppendRune(value, rune(key.unicodeChar))
	} else {
		switch key.virtualKeyCode {
		case vkBack:
			value = []byte{'\b'}
		case vkTab:
			value = []byte{'\t'}
		case vkReturn:
			value = []byte{'\r'}
		case vkEscape:
			value = []byte{'\x1b'}
		case vkUp:
			value = []byte("\x1b[A")
		case vkDown:
			value = []byte("\x1b[B")
		case vkRight:
			value = []byte("\x1b[C")
		case vkLeft:
			value = []byte("\x1b[D")
		case vkHome:
			value = []byte("\x1b[H")
		case vkEnd:
			value = []byte("\x1b[F")
		case vkInsert:
			value = []byte("\x1b[2~")
		case vkDelete:
			value = []byte("\x1b[3~")
		case vkPrior:
			value = []byte("\x1b[5~")
		case vkNext:
			value = []byte("\x1b[6~")
		}
	}

	if len(value) == 0 {
		return nil
	}

	output := make([]byte, 0, len(value)*repeats)
	for range repeats {
		output = append(output, value...)
	}
	return output
}
