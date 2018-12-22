package concurrentio

import (
	"context"
	"io"
)

type multiWriter struct {
	c int // level of concurrency
	w []io.Writer
}

func (w *multiWriter) Write(p []byte) (int, error) {
	errchan := make(chan error)

	concurrency := func() int {
		if w.c < 0 {
			return len(w.w)
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

func newMultiWriter(w ...io.Writer) *multiWriter {
	writers := make([]io.Writer, len(w), len(w))
	copy(writers, w)
	return &multiWriter{c: -1, w: writers}
}

func MultiWriterWithConcurrency(n int, w ...io.Writer) io.Writer {
	rc := newMultiWriter(w...)
	rc.c = n
	return rc
}

func MultiWriter(w ...io.Writer) io.Writer {
	return newMultiWriter(w...)
}
