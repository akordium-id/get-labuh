package logging

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type RotatingWriter struct {
	mu        sync.Mutex
	path      string
	maxSize   int64
	maxFiles  int
	current   *os.File
	size      int64
}

func NewRotatingWriter(path string, maxSize int64, maxFiles int) (*RotatingWriter, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	if maxFiles <= 0 {
		maxFiles = 5
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &RotatingWriter{
		path:     path,
		maxSize:  maxSize,
		maxFiles: maxFiles,
		current:  f,
		size:     info.Size(),
	}, nil
}

func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.current == nil {
		return 0, errors.New("writer is closed")
	}

	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = w.current.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) rotate() error {
	if err := w.current.Close(); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	base := w.path
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	rotated := name + "-" + timestamp + ext

	if err := os.Rename(w.path, rotated); err != nil {
		return err
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	w.current = f
	w.size = 0

	return w.cleanupOldFiles()
}

func (w *RotatingWriter) cleanupOldFiles() error {
	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext) + "-"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var rotated []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) {
			rotated = append(rotated, filepath.Join(dir, name))
		}
	}

	if len(rotated) <= w.maxFiles {
		return nil
	}

	sort.Slice(rotated, func(i, j int) bool {
		infoI, _ := os.Stat(rotated[i])
		infoJ, _ := os.Stat(rotated[j])
		if infoI == nil || infoJ == nil {
			return rotated[i] < rotated[j]
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	for i := 0; i < len(rotated)-w.maxFiles; i++ {
		os.Remove(rotated[i])
	}

	return nil
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		err := w.current.Close()
		w.current = nil
		return err
	}
	return nil
}

func (w *RotatingWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return w.current.Sync()
	}
	return nil
}

var _ io.WriteCloser = (*RotatingWriter)(nil)

type LogWriter interface {
	Write(p []byte) (n int, err error)
	Sync() error
	Close() error
}

type teeLogWriter struct {
	primary   LogWriter
	secondary io.Writer
}

func (t *teeLogWriter) Write(p []byte) (n int, err error) {
	n, err = t.primary.Write(p)
	if t.secondary != nil {
		_, _ = t.secondary.Write(p)
	}
	return n, err
}

func (t *teeLogWriter) Sync() error {
	return t.primary.Sync()
}

func (t *teeLogWriter) Close() error {
	return t.primary.Close()
}

func NewTeeLogWriter(primary LogWriter, secondary io.Writer) LogWriter {
	if secondary == nil {
		return primary
	}
	return &teeLogWriter{primary: primary, secondary: secondary}
}
