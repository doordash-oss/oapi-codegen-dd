// Copyright 2026 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// framing identifies how a sequential response body delimits its items.
type framing int

const (
	// framingSSE is "field: value" lines grouped into blank-line-separated frames.
	framingSSE framing = iota
	// framingLines is one JSON value per line.
	framingLines
)

// Event is a single frame of a sequential response body. Server-Sent Events
// populate every field; line-delimited JSON only sets Data.
type Event struct {
	ID   string
	Type string
	// Data is the frame payload, with multiple "data" lines joined by "\n".
	Data  []byte
	Retry time.Duration
}

// Stream decodes a sequential response body one frame at a time into values of
// type T, without buffering, so it works against an endless stream. The caller
// must Close it. To stop early, break out of the loop and Close, or cancel the
// request context - the pending read unblocks and Err reports the cancellation.
type Stream[T any] struct {
	// Sentinels ends the stream when a frame's payload equals one of these.
	// OpenAI-compatible APIs terminate with "data: [DONE]", which is not valid
	// JSON. Set before the first Next.
	Sentinels []string

	body       io.ReadCloser
	reader     *bufio.Reader
	framing    framing
	current    T
	event      Event
	err        error
	done       bool
	closed     bool
	bomChecked bool
}

// NewEventStream decodes resp as Server-Sent Events, one T per frame payload.
func NewEventStream[T any](resp *http.Response) *Stream[T] {
	return newStream[T](resp, framingSSE)
}

// NewLineStream decodes resp as line-delimited JSON, one T per line.
func NewLineStream[T any](resp *http.Response) *Stream[T] {
	return newStream[T](resp, framingLines)
}

// NewStream picks the framing from the response Content-Type.
func NewStream[T any](resp *http.Response) *Stream[T] {
	f := framingLines
	if resp != nil && baseMediaType(resp.Header.Get("Content-Type")) == "text/event-stream" {
		f = framingSSE
	}
	return newStream[T](resp, f)
}

func newStream[T any](resp *http.Response, f framing) *Stream[T] {
	s := &Stream[T]{framing: f}
	if resp == nil || resp.Body == nil {
		s.done = true
		return s
	}
	s.body = resp.Body
	s.reader = bufio.NewReader(resp.Body)
	return s
}

// Next reads and decodes the next frame. It returns false at the end of the
// stream, on a sentinel, or on the first error - check Err to tell them apart.
func (s *Stream[T]) Next() bool {
	if s.done {
		return false
	}

	event, ok := s.nextFrame()
	if !ok {
		return false
	}

	if s.isSentinel(event.Data) {
		s.done = true
		return false
	}

	var current T
	if err := decodeFrame(event.Data, &current); err != nil {
		s.fail(fmt.Errorf("error decoding stream frame: %w", err))
		return false
	}

	s.current = current
	s.event = event
	return true
}

// Current returns the value decoded by the most recent successful Next.
func (s *Stream[T]) Current() T {
	return s.current
}

// Event returns the raw frame Current was decoded from.
func (s *Stream[T]) Event() Event {
	return s.event
}

// Err returns the first read or decode error, nil once consumed cleanly.
func (s *Stream[T]) Err() error {
	return s.err
}

// Close releases the response body. Safe to call more than once. The body is
// not drained first, since an endless stream would never finish draining.
func (s *Stream[T]) Close() error {
	s.done = true
	if s.closed || s.body == nil {
		return nil
	}
	s.closed = true
	return s.body.Close()
}

// All iterates the remaining frames, yielding a terminal error as the last pair
// if one occurs. The caller still owns Close.
func (s *Stream[T]) All() iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for s.Next() {
			if !yield(s.current, nil) {
				return
			}
		}
		if s.err != nil {
			var zero T
			yield(zero, s.err)
		}
	}
}

// nextFrame returns the next dispatchable frame, skipping comments, keep-alives
// and frames with no payload.
func (s *Stream[T]) nextFrame() (Event, bool) {
	if s.framing == framingSSE {
		return s.nextEventFrame()
	}
	return s.nextLineFrame()
}

// nextEventFrame assembles one SSE frame per the WHATWG event stream spec.
func (s *Stream[T]) nextEventFrame() (Event, bool) {
	var (
		event   Event
		data    []byte
		hasData bool
	)

	for {
		line, err := s.readLine()
		if err != nil {
			// A frame the server never terminated with a blank line still
			// counts, as long as it carried a payload.
			if errors.Is(err, io.EOF) {
				s.done = true
				if hasData {
					event.Data = data
					return event, true
				}
				return Event{}, false
			}
			s.fail(err)
			return Event{}, false
		}

		// A blank line dispatches the frame. Without a payload it is a
		// keep-alive separator, which also resets the fields collected so far.
		if len(line) == 0 {
			if !hasData {
				event = Event{}
				continue
			}
			event.Data = data
			return event, true
		}

		// Lines starting with a colon are comments, commonly used as heartbeats.
		if line[0] == ':' {
			continue
		}

		field, value := splitEventField(line)
		switch field {
		case "event":
			event.Type = value
		case "data":
			if hasData {
				data = append(data, '\n')
			}
			data = append(data, value...)
			hasData = true
		case "id":
			// The specification requires ignoring an id containing a NUL.
			if !strings.ContainsRune(value, 0) {
				event.ID = value
			}
		case "retry":
			if ms, err := strconv.Atoi(value); err == nil && ms >= 0 {
				event.Retry = time.Duration(ms) * time.Millisecond
			}
		}
	}
}

// nextLineFrame returns the next non-blank line of a line-delimited body.
func (s *Stream[T]) nextLineFrame() (Event, bool) {
	for {
		line, err := s.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.done = true
				return Event{}, false
			}
			s.fail(err)
			return Event{}, false
		}

		if line = bytes.TrimSpace(line); len(line) == 0 {
			continue
		}
		return Event{Data: line}, true
	}
}

// readLine returns the next line without its terminator. "\n", "\r\n" and a lone
// "\r" are all accepted, as the SSE spec requires. A trailing line without a
// terminator is returned before io.EOF.
func (s *Stream[T]) readLine() ([]byte, error) {
	var line []byte

	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) > 0 {
				return s.trimBOM(line), nil
			}
			return nil, err
		}

		switch b {
		case '\n':
			return s.trimBOM(line), nil
		case '\r':
			// "\r\n" is a single terminator, so swallow a following "\n".
			if next, err := s.reader.ReadByte(); err == nil && next != '\n' {
				_ = s.reader.UnreadByte()
			}
			return s.trimBOM(line), nil
		default:
			line = append(line, b)
		}
	}
}

// trimBOM strips a byte-order mark from the first line of the stream.
func (s *Stream[T]) trimBOM(line []byte) []byte {
	if s.bomChecked {
		return line
	}
	s.bomChecked = true
	return bytes.TrimPrefix(line, []byte("\ufeff"))
}

// isSentinel reports whether data is one of the configured terminators.
func (s *Stream[T]) isSentinel(data []byte) bool {
	if len(s.Sentinels) == 0 {
		return false
	}

	trimmed := string(bytes.TrimSpace(data))
	for _, sentinel := range s.Sentinels {
		if trimmed == sentinel {
			return true
		}
	}
	return false
}

// fail records the first error and ends the stream.
func (s *Stream[T]) fail(err error) {
	if s.err == nil {
		s.err = err
	}
	s.done = true
}

// sequentialMediaTypes lists media types whose bodies frame a sequence of items.
// Buffering one with io.ReadAll blocks until the server closes the connection,
// which for an endless stream never happens.
var sequentialMediaTypes = map[string]bool{
	"text/event-stream":       true,
	"application/x-ndjson":    true,
	"application/ndjson":      true,
	"application/jsonl":       true,
	"application/x-jsonlines": true,
	"application/json-lines":  true,
}

// IsSequentialMediaType reports whether contentType frames its body as a
// sequence of items. Parameters such as "; charset=utf-8" are ignored.
func IsSequentialMediaType(contentType string) bool {
	return sequentialMediaTypes[baseMediaType(contentType)]
}

// baseMediaType strips any parameters from contentType and lowercases it.
func baseMediaType(contentType string) string {
	if contentType == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		return parsed
	}
	base, _, _ := strings.Cut(contentType, ";")
	return strings.ToLower(strings.TrimSpace(base))
}

// splitEventField splits an SSE line into its field name and value, dropping a
// single space after the colon. A line with no colon is a field with an empty
// value.
func splitEventField(line []byte) (string, string) {
	field, value, found := bytes.Cut(line, []byte(":"))
	if !found {
		return string(line), ""
	}
	return string(field), string(bytes.TrimPrefix(value, []byte(" ")))
}

// decodeFrame decodes a frame payload into out. Byte, string and
// json.RawMessage targets get it verbatim, so non-JSON frames stay usable.
func decodeFrame[T any](data []byte, out *T) error {
	switch target := any(out).(type) {
	case *[]byte:
		*target = bytes.Clone(data)
	case *string:
		*target = string(data)
	case *json.RawMessage:
		*target = bytes.Clone(data)
	default:
		return json.Unmarshal(data, out)
	}
	return nil
}
