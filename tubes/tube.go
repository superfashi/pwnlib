package tubes

import (
	"bytes"
	"io"
	"regexp"
	"time"

	"github.com/superfashi/pwnlib/tubes/internal/reader"
)

type TubeDirection uint8

const (
	Read TubeDirection = iota
	Send
)

type TubeIf interface {
	io.ReadWriteCloser
	Shutdown(TubeDirection) error
	SetReadDeadline(time.Time) error
}

type r = reader.Reader

type Tube struct {
	*r
	impl interface {
		io.WriteCloser
		Shutdown(TubeDirection) error
	}

	KeepLineEnding bool
}

func NewTube(impl TubeIf) *Tube {
	t := &Tube{
		r:    reader.NewReader(impl),
		impl: impl,
	}
	return t
}

func (t *Tube) ReadLine() ([]byte, error) {
	return t.ReadUntil(!t.KeepLineEnding, []byte{'\r', '\n'}, []byte{'\n'})
}

func (t *Tube) ReadLinePred(pred func(line []byte) bool) ([]byte, error) {
	for {
		line, err := t.ReadLine()
		if pred(line) {
			return line, err
		}
		if err != nil {
			return nil, err
		}
	}
}

func foldFuncItems(f func([]byte, []byte) bool, items ...[]byte) func(line []byte) bool {
	return func(line []byte) bool {
		for _, i := range items {
			if f(line, i) {
				return true
			}
		}
		return false
	}
}

func (t *Tube) ReadLineContains(items ...[]byte) ([]byte, error) {
	return t.ReadLinePred(foldFuncItems(bytes.Contains, items...))
}

func (t *Tube) ReadLineStartsWith(items ...[]byte) ([]byte, error) {
	return t.ReadLinePred(foldFuncItems(bytes.HasPrefix, items...))
}

func (t *Tube) ReadLineEndsWith(items ...[]byte) ([]byte, error) {
	return t.ReadLinePred(foldFuncItems(bytes.HasSuffix, items...))
}

func (t *Tube) ReadLineRegex(regex *regexp.Regexp, exact bool) ([]byte, error) {
	if exact {
		return t.ReadLinePred(regex.Match)
	}
	return t.ReadLinePred(func(line []byte) bool {
		return regex.Find(line) != nil
	})
}

func (t *Tube) ReadLines(numLines int) ([][]byte, error) {
	ret := make([][]byte, 0, numLines)
	for i := 0; i < numLines; i++ {
		line, err := t.ReadLine()
		if err == nil || len(line) > 0 {
			ret = append(ret, line)
		}
		if err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (t *Tube) ReadRegex(regex *regexp.Regexp, exact bool) ([]byte, error) {
	if exact {
		return t.ReadPred(regex.Match)
	}
	return t.ReadPred(func(line []byte) bool {
		return regex.Find(line) != nil
	})
}

func (t *Tube) Send(data []byte) (int, error) {
	return t.impl.Write(data)
}

func (t *Tube) SendAfter(data []byte, delims ...[]byte) (int, error) {
	_, err := t.ReadUntil(false, delims...)
	if err != nil {
		return 0, err
	}
	return t.Send(data)
}

func (t *Tube) SendLine(data []byte) (int, error) {
	return t.Send(append(data, '\n'))
}

func (t *Tube) SendLineAfter(data []byte, delims ...[]byte) (int, error) {
	_, err := t.ReadUntil(false, delims...)
	if err != nil {
		return 0, err
	}
	return t.SendLine(data)
}

func (t *Tube) SendThen(data []byte, delims ...[]byte) (int, error) {
	s, err := t.Send(data)
	if err == nil {
		_, err = t.ReadUntil(false, delims...)
	}
	return s, err
}

func (t *Tube) SendLineThen(data []byte, delims ...[]byte) (int, error) {
	s, err := t.SendLine(data)
	if err == nil {
		_, err = t.ReadUntil(false, delims...)
	}
	return s, err
}

func (t *Tube) Close() error {
	return t.impl.Close()
}

func (t *Tube) Shutdown(direction TubeDirection) error {
	return t.impl.Shutdown(direction)
}
