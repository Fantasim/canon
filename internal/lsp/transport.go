package lsp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"sync"
)

// readFrame reads one base-protocol message: headers, a blank line, then Content-Length bytes.
func readFrame(r *bufio.Reader) ([]byte, error) {
	header, err := textproto.NewReader(r).ReadMIMEHeader()
	if err != nil {
		if len(header) == 0 && errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf(fmtWrap, errHeader, err)
	}
	n, err := strconv.Atoi(header.Get(headerLength))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("%w: %s %q", errHeader, headerLength, header.Get(headerLength))
	}
	if n > maxMessage {
		if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
			return nil, fmt.Errorf(fmtWrap, errHeader, err)
		}
		return nil, fmt.Errorf("%w: %d bytes", errTooLarge, n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf(fmtWrap, errHeader, err)
	}
	return body, nil
}

// frame is one message read, or the error that ended the input.
type frame struct {
	body []byte
	err  error
}

// readFrames sends every message of in to frames, a skipped oversize one as its error, and
// returns the error that ended in.
func readFrames(ctx context.Context, in io.Reader, frames chan<- frame) error {
	r := bufio.NewReader(in)
	for {
		body, err := readFrame(r)
		if err != nil && !errors.Is(err, errTooLarge) {
			return err
		}
		select {
		case frames <- frame{body: body, err: err}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// writer frames messages onto out, one at a time; the first failure sticks.
type writer struct {
	mu  sync.Mutex
	out io.Writer
	err error
}

// write sends body as one message, unless an earlier write failed.
func (w *writer) write(body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	msg := append(fmt.Appendf(nil, headerFormat, len(body)), body...)
	if _, err := w.out.Write(msg); err != nil {
		w.err = fmt.Errorf(fmtWrap, errWrite, err)
	}
	return w.err
}

// failed is the error that stopped the writer, nil while it works.
func (w *writer) failed() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}
