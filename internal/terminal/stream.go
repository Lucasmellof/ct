package terminal

import (
	"bytes"
	"errors"
	"io"
	"time"

	highlight "github.com/lucasmellof/ct/internal/highlighter"
)

const (
	streamBufferSize    = 32 * 1024 // todo: check if this is the best size
	streamFlushInterval = 100 * time.Millisecond
)

type streamReadResult struct {
	data []byte
	err  error
}

func copyHighlighted(dst io.Writer, source io.Reader, highlighter *highlight.Highlighter) error {
	result := make(chan streamReadResult, 16)

	go readStream(source, result)

	var pending []byte

	timer := time.NewTimer(time.Hour)

	if !timer.Stop() {
		<-timer.C
	}

	timerActive := false
	stopTimer := func() {
		if !timerActive {
			return
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerActive = false
	}

	resetTimer := func() {
		stopTimer()

		timer.Reset(streamFlushInterval)
		timerActive = true
	}

	flushPending := func() error {
		if len(pending) == 0 {
			return nil
		}

		highlight := highlighter.Highlight(pending)
		pending = nil

		_, err := dst.Write(highlight)
		return err
	}

	flushCompleteLines := func() error {
		lastNewLine := bytes.LastIndexByte(pending, '\n')
		if lastNewLine < 0 {
			return nil
		}

		ready := append([]byte(nil), pending[:lastNewLine+1]...)
		remaining := append([]byte(nil), pending[lastNewLine+1:]...)

		pending = remaining

		highlighted := highlighter.Highlight(ready)
		_, err := dst.Write(highlighted)
		return err
	}

	defer timer.Stop()

	for {
		select {
		case result, ok := <-result:
			if !ok {
				stopTimer()
				return flushPending()
			}

			if len(result.data) > 0 {
				pending = append(pending, result.data...)

				if err := flushCompleteLines(); err != nil {
					return err
				}

				if len(pending) > 0 {
					resetTimer()
				}
			}

			if result.err != nil {
				stopTimer()
				flushErr := flushPending()
				if flushErr != nil {
					return flushErr
				}

				if errors.Is(result.err, io.EOF) {
					return nil
				}
				return result.err
			}

		case <-timer.C:
			timerActive = true
			if err := flushPending(); err != nil {
				return err
			}
		}
	}
}

func readStream(source io.Reader, result chan<- streamReadResult) {
	defer close(result)

	buffer := make([]byte, streamBufferSize)

	for {
		n, err := source.Read(buffer)
		if n > 0 {
			data := append([]byte(nil), buffer[:n]...)

			result <- streamReadResult{
				data: data,
			}
		}
		if err != nil {
			result <- streamReadResult{
				err: err,
			}
			return
		}
	}

}
