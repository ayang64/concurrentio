package concurrentio

import "io"

type SimpleMultiWriter struct {
	w []io.Writer
}

func SimpleMultiWriterNew(w ...io.Writer) *SimpleMultiWriter {
	wr := make([]io.Writer, len(w), len(w))
	copy(wr, w)
	return &SimpleMultiWriter{w: wr}
}

func (w *SimpleMultiWriter) Write(p []byte) (int, error) {
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
