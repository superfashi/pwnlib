package tubes

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type TubeDirection uint8

const bufferSize = 4096

const (
	Read TubeDirection = iota + 1
	Send
	Both
)

type TubeIf interface {
	io.ReadWriteCloser
	Shutdown(TubeDirection) error
}

type Tube struct {
	r    *fakeReader
	impl interface {
		io.WriteCloser
		Shutdown(TubeDirection) error
	}

	KeepLineEnding bool
	Newline        []byte
	RecvTimeout    time.Duration
}

func NewTube(impl TubeIf) *Tube {
	t := &Tube{
		r:    newFakeReader(impl),
		impl: impl,

		Newline:     linebreak,
		RecvTimeout: 10 * time.Second,
	}
	return t
}

func (t *Tube) Clean() {
	_, _ = t.RecvRepeat()
}

func (t *Tube) Recv(numBytes int) ([]byte, error) {
	return t.r.recv(numBytes, t.RecvTimeout, read)
}

func (t *Tube) RecvAll() ([]byte, error) {
	return io.ReadAll(t.r)
}

func (t *Tube) RecvAllString() (string, error) {
	all, err := io.ReadAll(t.r)
	return string(all), err
}

func (t *Tube) RecvLine() ([]byte, error) {
	bs, err := t.RecvUntil(!t.KeepLineEnding, '\n')
	l := len(bs)
	if !t.KeepLineEnding && l > 0 && bs[l-1] == '\r' {
		bs = bs[:l-1]
	}
	return bs, err
}

func (t *Tube) RecvLineString() (string, error) {
	line, err := t.RecvLine()
	return string(line), err
}

func (t *Tube) RecvLinePred(pred func(line []byte) bool) ([]byte, error) {
	for {
		line, err := t.RecvLine()
		if pred(line) {
			return line, err
		}
		if err != nil {
			return nil, err
		}
	}
}

func (t *Tube) RecvLinePredString(pred func(line string) bool) (string, error) {
	for {
		line, err := t.RecvLineString()
		if pred(line) {
			return line, err
		}
		if err != nil {
			return "", err
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

func foldFuncItemsStr(f func(string, string) bool, items ...string) func(line string) bool {
	return func(line string) bool {
		for _, i := range items {
			if f(line, i) {
				return true
			}
		}
		return false
	}
}

func (t *Tube) RecvLineContains(items ...[]byte) ([]byte, error) {
	return t.RecvLinePred(foldFuncItems(bytes.Contains, items...))
}

func (t *Tube) RecvLineContainsString(items ...string) (string, error) {
	return t.RecvLinePredString(foldFuncItemsStr(strings.Contains, items...))
}

func (t *Tube) RecvLineStartsWith(items ...[]byte) ([]byte, error) {
	return t.RecvLinePred(foldFuncItems(bytes.HasPrefix, items...))
}

func (t *Tube) RecvLineStartsWithString(items ...string) (string, error) {
	return t.RecvLinePredString(foldFuncItemsStr(strings.HasPrefix, items...))
}

func (t *Tube) RecvLineEndsWith(items ...[]byte) ([]byte, error) {
	return t.RecvLinePred(foldFuncItems(bytes.HasSuffix, items...))
}

func (t *Tube) RecvLineEndsWithString(items ...string) (string, error) {
	return t.RecvLinePredString(foldFuncItemsStr(strings.HasSuffix, items...))
}

func (t *Tube) RecvLineRegex(regex *regexp.Regexp, exact bool) ([]byte, error) {
	if exact {
		return t.RecvLinePred(regex.Match)
	}
	return t.RecvLinePred(func(line []byte) bool {
		return regex.Find(line) != nil
	})
}

func (t *Tube) RecvLineRegexString(regex *regexp.Regexp, exact bool) (string, error) {
	if exact {
		return t.RecvLinePredString(regex.MatchString)
	}
	return t.RecvLinePredString(func(line string) bool {
		return regex.FindStringIndex(line) != nil
	})
}

func (t *Tube) RecvLines(numLines int) ([][]byte, error) {
	ret := make([][]byte, 0, numLines)
	for i := 0; i < numLines; i++ {
		line, err := t.RecvLine()
		if err == nil || len(line) > 0 {
			ret = append(ret, line)
		}
		if err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (t *Tube) RecvLinesString(numLines int) ([]string, error) {
	ret := make([]string, 0, numLines)
	for i := 0; i < numLines; i++ {
		line, err := t.RecvLineString()
		if err == nil || len(line) > 0 {
			ret = append(ret, line)
		}
		if err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (t *Tube) RecvN(numBytes int) ([]byte, error) {
	ret := make([]byte, numBytes)
	_, err := io.ReadFull(t.r, ret)
	return ret, err
}

func (t *Tube) RecvPred(pred func([]byte) bool) ([]byte, error) {
	var ret []byte
	for {
		b, err := t.Recv(1)
		if err != nil {
			return ret, err
		}
		ret = append(ret, b...)
		if pred(ret) {
			return ret, nil
		}
	}
}

func (t *Tube) RecvRegex(regex *regexp.Regexp, exact bool) ([]byte, error) {
	if exact {
		return t.RecvPred(regex.Match)
	}
	return t.RecvPred(func(line []byte) bool {
		return regex.Find(line) != nil
	})
}

func (t *Tube) recvRepeat(timeout time.Duration) ([]byte, error) {
	const bufferSize = 4096
	var ret []byte
	end := time.Now().Add(timeout)
	remaining := timeout

	for remaining > 0 {
		recv, err := t.r.recv(bufferSize, remaining, read)
		ret = append(ret, recv...)
		if err != nil {
			return ret, err
		}
		remaining = end.Sub(time.Now())
	}
	return ret, nil
}

func (t *Tube) RecvRepeat() ([]byte, error) {
	if t.RecvTimeout <= 0 {
		return t.RecvAll()
	}

	var ret []byte
	remaining := t.RecvTimeout
	end := time.Now().Add(t.RecvTimeout)

	for remaining > 0 {
		recv, err := t.r.recv(bufferSize, remaining, read)
		ret = append(ret, recv...)
		if err != nil {
			return ret, err
		}
		remaining = end.Sub(time.Now())
	}
	return ret, nil
}

func (t *Tube) RecvUntil(dropEnding bool, delims ...byte) ([]byte, error) {
	var ret []byte
	start := time.Now()
	end := start.Add(t.RecvTimeout)

	matched := false
	for end.Before(start) || time.Now().Before(end) {
		recv, err := t.r.recv(bufferSize, end.Sub(start), peek)
		for _, d := range delims {
			index := bytes.IndexByte(recv, d)
			if index != -1 {
				recv = recv[:index+1]
				matched = true
				break
			}
		}
		ret = append(ret, recv...)
		t.r.discard(len(recv))
		if matched {
			if dropEnding {
				ret = ret[:len(ret)-1]
			}
			return ret, err
		}
		if err != nil {
			return ret, err
		}
	}

	return ret, nil
}

func (t *Tube) Send(data []byte) (int, error) {
	return t.impl.Write(data)
}

func (t *Tube) SendAfter(data []byte, delims ...byte) (int, error) {
	_, err := t.RecvUntil(false, delims...)
	if err != nil {
		return 0, err
	}
	return t.Send(data)
}

func (t *Tube) SendLine(data []byte) (int, error) {
	return t.Send(append(data, t.Newline...))
}

func (t *Tube) SendLineAfter(data []byte, delims ...byte) (int, error) {
	_, err := t.RecvUntil(false, delims...)
	if err != nil {
		return 0, err
	}
	return t.SendLine(data)
}

func (t *Tube) SendThen(data []byte, delims ...byte) (int, error) {
	s, err := t.Send(data)
	if err == nil {
		_, err = t.RecvUntil(false, delims...)
	}
	return s, err
}

func (t *Tube) SendLineThen(data []byte, delims ...byte) (int, error) {
	s, err := t.SendLine(data)
	if err == nil {
		_, err = t.RecvUntil(false, delims...)
	}
	return s, err
}

func (t *Tube) Close() error {
	return t.impl.Close()
}

func (t *Tube) SpawnProcess(cmd *exec.Cmd) error {
	cmd.Stdin = t.r
	cmd.Stdout = t.impl
	cmd.Stderr = t.impl
	return cmd.Start()
}

func (t *Tube) Stream() (int64, error) {
	return io.Copy(os.Stdout, t.r)
}

func (t *Tube) ConnectInput(other *Tube) {
	go func() {
		_, _ = io.Copy(t.impl, other.r)
	}()
}

func (t *Tube) ConnectOutput(other *Tube) {
	go func() {
		_, _ = io.Copy(other.impl, t.r)
	}()
}

func (t *Tube) ConnectBoth(other *Tube) {
	t.ConnectInput(other)
	t.ConnectOutput(other)
}

func (t *Tube) Interactive(prompt []byte) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		_, _ = io.Copy(os.Stdout, t.r)
	}()

	_, _ = os.Stderr.Write(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	for ctx.Err() == nil && scanner.Scan() {
		_, _ = t.SendLine(scanner.Bytes())
		_, _ = os.Stderr.Write(prompt)
	}
}

func (t *Tube) Unrecv(data []byte) {
	t.r.unrecv(data)
}

func (t *Tube) Shutdown(direction TubeDirection) error {
	return t.impl.Shutdown(direction)
}

// CanRecv returns true if there is data available within RecvTimeout.
func (t *Tube) CanRecv() bool {
	_, err := t.r.recv(1, t.RecvTimeout, peek)
	return err == nil
}
