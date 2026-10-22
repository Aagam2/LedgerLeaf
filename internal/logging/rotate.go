// Package logging bounds local log storage without an external logging service.
package logging

import (
	"fmt"
	"os"
	"sync"
)

type Writer struct {
	mu   sync.Mutex
	path string
	max  int64
	keep int
	file *os.File
	size int64
}

func Open(path string, max int64, keep int) (*Writer, error) {
	if max < 1 || keep < 1 {
		return nil, fmt.Errorf("invalid log retention")
	}
	w := &Writer{path: path, max: max, keep: keep}
	if e := w.open(); e != nil {
		return nil, e
	}
	return w, nil
}
func (w *Writer) open() error {
	f, e := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	info, e := f.Stat()
	if e != nil {
		f.Close()
		return e
	}
	w.file = f
	w.size = info.Size()
	return nil
}
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size > 0 && w.size+int64(len(p)) > w.max {
		if e := w.file.Close(); e != nil {
			return 0, e
		}
		w.file = nil
		for i := w.keep; i >= 1; i-- {
			dst := fmt.Sprintf("%s.%d", w.path, i)
			src := w.path
			if i > 1 {
				src = fmt.Sprintf("%s.%d", w.path, i-1)
			}
			if e := os.Remove(dst); e != nil && !os.IsNotExist(e) {
				w.open()
				return 0, e
			}
			if e := os.Rename(src, dst); e != nil && !os.IsNotExist(e) {
				w.open()
				return 0, e
			}
		}
		if e := w.open(); e != nil {
			return 0, e
		}
	}
	n, e := w.file.Write(p)
	w.size += int64(n)
	return n, e
}
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	e := w.file.Close()
	w.file = nil
	return e
}
