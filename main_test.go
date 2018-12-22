package concurrentio_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/ayang64/concurrentio"
)

func TestMultiWriter(t *testing.T) {
	w := concurrentio.MultiWriter(os.Stdout, os.Stderr)
	t.Logf("%#v", w)
}

type SlowWriter struct {
	latency time.Duration
	w       io.Writer
}

func (s *SlowWriter) Write(p []byte) (int, error) {
	time.Sleep(s.latency)
	return s.w.Write(p)
}

func NewSlowWriter(latency time.Duration, w io.Writer) *SlowWriter {
	return &SlowWriter{latency: latency, w: w}
}

func BenchmarkWriters(b *testing.B) {

	randomdata := func(s int) []byte {
		rc := make([]byte, s, s)
		rand.Read(rc)
		return rc
	}

	randompaths := func(n int) []string {
		rc := make([]string, n, n)
		for i := 0; i < n; i++ {
			rc[i] = fmt.Sprintf("test-data/test-file-%05d.dat", i)
		}
		return rc
	}

	open := func(paths ...string) ([]*os.File, error) {
		rc := []*os.File{}
		for _, path := range paths {
			fh, err := os.Create(path)

			if err != nil {
				// should probably close open files here?
				return nil, err
			}
			rc = append(rc, fh)
		}
		return rc, nil
	}

	towriter := func(fh []*os.File) []io.Writer {
		rc := make([]io.Writer, len(fh), len(fh))
		for i := range fh {
			rc[i] = NewSlowWriter(250*time.Millisecond, fh[i])
		}

		return rc
	}

	data := randomdata(1024 * 1024) // generage a megabyte of random data

	benches := []struct {
		Name string
		F    func(...io.Writer) io.Writer
	}{
		{Name: "Stdlib Multi-Writer", F: io.MultiWriter},
		{Name: "Simple Multi-Writer", F: concurrentio.SimpleMultiWriterNew},
		{Name: "Concurrent Multi-Writer", F: concurrentio.MultiWriter},
	}

	for exp := uint(1); exp < 13; exp++ {
		numfiles := 1 << exp
		for _, bench := range benches {
			paths := randompaths(numfiles)
			handles, err := open(paths...)

			if err != nil {
				b.Fatal(err)
				b.FailNow()
			}

			writers := towriter(handles)
			writer := bench.F(writers...)

			countlabel := fmt.Sprintf("Files-%05d", numfiles)

			b.Run(countlabel, func(b *testing.B) {
				b.Run(bench.Name, func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						writer.Write(data)
					}
				})
			})

			for i := range handles {
				handles[i].Close()
			}

			for i := range paths {
				if err := os.Remove(paths[i]); err != nil {
					b.Fatal(err)
					b.FailNow()
				}
			}
		}
	}
}

func TestMultiWriterWriting(t *testing.T) {
	o1, err := os.Create("o1.dat")

	if err != nil {
		t.Fatal(err)
		t.FailNow()
	}
	defer o1.Close()

	o2, err := os.Create("o2.dat")
	if err != nil {
		t.Fatal(err)
		t.FailNow()
	}

	defer o2.Close()

	w := concurrentio.MultiWriter(o1, o2)

	fmt.Fprintf(w, "HELLO WORLD!\n")
}
