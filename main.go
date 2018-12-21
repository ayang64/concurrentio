package concurrentio

import (
	"context"
	"io"
)

type SimpleWriter struct {
	w []io.Writer
}

func SimpleWriterNew(w ...io.Writer) *SimpleWriter {
	wr := make([]io.Writer, len(w), len(w))
	copy(wr, w)
	return &SimpleWriter{w: wr}
}

func (w *SimpleWriter) Write(p []byte) (int, error) {
	errors := make(chan error, len(w.w))
	for _, writer := range w.w {
		go func(w io.Writer) {
			write := func() error {
				n, err := w.Write(p)
				if err != nil {
					return err
				}
				if n != len(p) {
					return io.ErrShortWrite
				}
				return nil
			}
			errors <- write()
		}(writer)
	}

	drain := func() error {
		err := error(nil)
		for i := 0; i < cap(errors); i++ {
			if e := <-errors; e != nil {
				err = e
			}
		}
		return err
	}

	if err := drain(); err != nil {
		return 0, err
	}

	return len(p), nil
}

type Writer struct {
	c int // level of concurrency
	w []io.Writer
}

func (w *Writer) Write(p []byte) (int, error) {
	errchan := make(chan error)

	concurrency := func() int {
		if w.c < 0 {
			return len(p)
		}
		return w.c
	}()

	ctx, cancel := context.WithCancel(context.Background())

	workers := func() (chan<- io.Writer, <-chan struct{}) {
		rc := make(chan io.Writer, concurrency)
		done := make(chan struct{}, concurrency)

		go func() {
			for i := 0; i < concurrency; i++ {
				go func() {
					defer func() { done <- struct{}{} }()
					for {
						select {
						case w := <-rc:
							wl, err := w.Write(p)
							if err != nil {
								errchan <- err
							}

							if wl != len(p) {
								errchan <- io.ErrShortWrite
							}
						case <-ctx.Done():
							return
						}
					}
				}()
			}
		}()

		return rc, done
	}

	work, done := workers()

	go func() {
		for i := range w.w {
			work <- w.w[i]
		}
		cancel()
	}()

	completed := 0
mainloop:
	for {
		select {
		case err := <-errchan:
			cancel() // cancel pending workers.
			return 0, err
		case <-done:
			completed++
			if completed == concurrency {
				break mainloop
			}
		}
	}

	return len(p), nil
}
func MultiWriterWithConcurrency(n int, w ...io.Writer) *Writer {
	rc := MultiWriter(w...)
	rc.c = n
	return rc
}

func MultiWriter(w ...io.Writer) *Writer {
	writers := make([]io.Writer, len(w), len(w))
	copy(writers, w)
	return &Writer{c: -1, w: writers}
}
