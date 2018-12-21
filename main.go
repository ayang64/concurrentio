package concurrentio

import (
	"context"
	"io"
	"runtime"
)

type Writer struct {
	c int // level of concurrency
	w []io.Writer
}

func (w *Writer) Write(p []byte) (int, error) {
	errchan := make(chan error)

	concurrency := func() int {
		if w.c < 0 {
			return 4
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

func MultiWriter(w ...io.Writer) *Writer {
	writers := make([]io.Writer, len(w), len(w))
	copy(writers, w)
	return &Writer{c: runtime.NumCPU(), w: writers}
}
