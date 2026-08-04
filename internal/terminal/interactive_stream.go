package terminal

import (
	"bytes"
	"errors"
	"io"
	"time"

	highlight "github.com/lucasmellof/ct/internal/highlighter"
)

const (
	interactiveStreamBufferSize = 8 * 1024
	echoFlushDelay              = 2 * time.Millisecond
)

type interactiveReadResult struct {
	data []byte
	err  error
}

func copyHighlightedInteractive(dst io.Writer, source io.Reader, highlighter *highlight.Highlighter, input *inputTracker) error {
	result := make(chan interactiveReadResult, 16)
	go readInteractiveStream(source, result)

	state := streamingState{dst: dst, highlighter: highlighter}
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
	startEchoTimer := func() {
		if timerActive || !input.MatchesEchoPrefix(state.pending) {
			return
		}
		timer.Reset(echoFlushDelay)
		timerActive = true
	}

	defer timer.Stop()
	for {
		select {
		case result, ok := <-result:
			if !ok {
				return state.finish()
			}
			stopTimer()
			if len(result.data) > 0 {
				if err := state.push(result.data); err != nil {
					return err
				}
				if isLikelyEchoChunk(result.data) {
					if err := state.flushPendingRaw(); err != nil {
						return err
					}
				} else {
					startEchoTimer()
				}
			}
			if result.err != nil {
				if err := state.finish(); err != nil {
					return err
				}
				if errors.Is(result.err, io.EOF) {
					return nil
				}
				return result.err
			}
		case <-timer.C:
			timerActive = false
			if input.ConsumeEchoPrefix(state.pending) {
				if err := state.flushPendingRaw(); err != nil {
					return err
				}
			}
		}
	}
}

type streamingState struct {
	dst         io.Writer
	highlighter *highlight.Highlighter
	ready       []byte
	pending     []byte
	ansiCarry   []byte
}

func (s *streamingState) push(data []byte) error {
	if len(s.ansiCarry) > 0 {
		data = append(append([]byte(nil), s.ansiCarry...), data...)
		s.ansiCarry = nil
	}

	for index := 0; index < len(data); {
		nextEscape := bytes.IndexByte(data[index:], '\x1b')
		if nextEscape < 0 {
			if err := s.pushText(data[index:]); err != nil {
				return err
			}
			return s.flushReady()
		}
		nextEscape += index
		if err := s.pushText(data[index:nextEscape]); err != nil {
			return err
		}
		end, complete := ansiSequenceEnd(data[nextEscape:])
		if !complete {
			if err := s.flushReady(); err != nil {
				return err
			}
			s.ansiCarry = append(s.ansiCarry, data[nextEscape:]...)
			return nil
		}
		if err := s.flushReady(); err != nil {
			return err
		}
		if err := s.flushPending(); err != nil {
			return err
		}
		sequence := data[nextEscape : nextEscape+end]
		if _, err := s.dst.Write(sequence); err != nil {
			return err
		}
		index = nextEscape + end
	}
	return nil
}

func (s *streamingState) pushText(data []byte) error {
	for _, value := range data {
		s.pending = append(s.pending, value)
		if !isTokenContinuation(value) {
			s.ready = append(s.ready, s.pending...)
			s.pending = s.pending[:0]
		}
	}
	return nil
}

func (s *streamingState) flushReady() error {
	if len(s.ready) == 0 {
		return nil
	}
	data := s.highlighter.Highlight(s.ready)
	s.ready = s.ready[:0]
	_, err := s.dst.Write(data)
	return err
}

func (s *streamingState) flushPending() error {
	if len(s.pending) == 0 {
		return nil
	}
	data := s.highlighter.Highlight(s.pending)
	s.pending = nil
	_, err := s.dst.Write(data)
	return err
}

func (s *streamingState) flushPendingRaw() error {
	if len(s.pending) == 0 {
		return nil
	}
	data := s.pending
	s.pending = nil
	_, err := s.dst.Write(data)
	return err
}

func (s *streamingState) finish() error {
	if err := s.flushReady(); err != nil {
		return err
	}
	if err := s.flushPending(); err != nil {
		return err
	}
	if len(s.ansiCarry) == 0 {
		return nil
	}
	_, err := s.dst.Write(s.ansiCarry)
	s.ansiCarry = nil
	return err
}

func isTokenContinuation(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		bytes.IndexByte([]byte("._:/-"), value) >= 0
}

func isLikelyEchoChunk(data []byte) bool {
	if len(data) == 0 || len(data) > 64 {
		return false
	}
	for _, value := range data {
		if !isTokenContinuation(value) {
			return false
		}
	}
	return true
}

func ansiSequenceEnd(data []byte) (int, bool) {
	if len(data) < 2 || data[0] != '\x1b' {
		return 0, false
	}
	switch data[1] {
	case '[':
		for index, value := range data[2:] {
			if value >= 0x40 && value <= 0x7e {
				return index + 3, true
			}
		}
		return 0, false
	case ']', 'P', '^', '_':
		for index := 2; index < len(data); index++ {
			if data[index] == '\a' {
				return index + 1, true
			}
			if data[index] == '\x1b' && index+1 < len(data) && data[index+1] == '\\' {
				return index + 2, true
			}
		}
		return 0, false
	default:
		return 2, true
	}
}

func readInteractiveStream(source io.Reader, result chan<- interactiveReadResult) {
	defer close(result)
	buffer := make([]byte, interactiveStreamBufferSize)
	for {
		n, err := source.Read(buffer)
		if n > 0 {
			result <- interactiveReadResult{data: append([]byte(nil), buffer[:n]...)}
		}
		if err != nil {
			result <- interactiveReadResult{err: err}
			return
		}
	}
}
