package logging

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 日志：业务输出一律写 .log 文件，不再打屏。
//   LianT.log      当前日志
//   LianT.log.1..5 轮转后的历史日志（每个 8MB，共留 5 个）
// ============================================================================

const (
	LogMaxBytes = 8 << 20 // 单文件上限 8MB
	LogKeep     = 5       // 保留的历史文件个数
)

// RotatingWriter 是带大小轮转的文件写入器，可安全并发使用。
type RotatingWriter struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	keep    int
	file    *os.File
	size    int64
}

func NewRotatingWriter(path string, maxSize int64, keep int) (*RotatingWriter, error) {
	w := &RotatingWriter{path: path, maxSize: maxSize, keep: keep}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = st.Size()
	return nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.maxSize > 0 && w.size+int64(len(p)) > w.maxSize {
		w.rotateLocked()
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotateLocked 把 LianT.log 挪成 .1，旧的整体后移，然后重新开一个空文件。
func (w *RotatingWriter) rotateLocked() {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	for i := w.keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	_ = os.Rename(w.path, w.path+".1")
	_ = w.open()
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// Setup 把标准库 log 的输出接到 .log 文件。
// alsoConsole 只用于前台调试（LianT run），正式运行不要开，避免退回终端输出。
func Setup(path string, alsoConsole bool) (*RotatingWriter, error) {
	w, err := NewRotatingWriter(path, LogMaxBytes, LogKeep)
	if err != nil {
		return nil, err
	}
	var out io.Writer = w
	if alsoConsole {
		out = io.MultiWriter(w, os.Stderr)
	}
	log.SetOutput(out)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return w, nil
}

// Tail 输出文件最后 n 行（只读，不影响任务进程）。
func Tail(path string, n int, out io.Writer) error {
	if n <= 0 {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if size == 0 {
		return nil
	}
	const chunk = 32 * 1024
	var buf []byte
	pos := size
	for pos > 0 {
		read := int64(chunk)
		if pos < read {
			read = pos
		}
		pos -= read
		tmp := make([]byte, read)
		if _, err := f.ReadAt(tmp, pos); err != nil && err != io.EOF {
			return err
		}
		buf = append(tmp, buf...)
		if bytes.Count(buf, []byte{'\n'}) > n {
			break
		}
	}
	text := strings.TrimRight(string(buf), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, l := range lines {
		fmt.Fprintln(out, l)
	}
	return nil
}

// Follow 类似 tail -f：持续输出新写入的行，直到 stop 关闭。
func Follow(path string, out io.Writer, stop <-chan struct{}) error {
	offset := int64(-1)
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		f, err := os.Open(path)
		if err != nil {
			select {
			case <-stop:
				return nil
			case <-time.After(time.Second):
			}
			continue
		}
		if st, err := f.Stat(); err == nil {
			if offset < 0 {
				offset = st.Size()
			} else if st.Size() < offset {
				offset = 0 // 文件被轮转/清空，从头读
			}
			if _, err := f.Seek(offset, io.SeekStart); err == nil {
				for {
					n, rerr := f.Read(buf)
					if n > 0 {
						if _, werr := out.Write(buf[:n]); werr != nil {
							_ = f.Close()
							return werr
						}
						offset += int64(n)
					}
					if rerr != nil {
						break
					}
				}
			}
		}
		_ = f.Close()
		select {
		case <-stop:
			return nil
		case <-time.After(400 * time.Millisecond):
		}
	}
}
