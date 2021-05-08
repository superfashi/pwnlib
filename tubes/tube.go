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

type tubeInner interface {
	io.WriteCloser
	shutdown(TubeDirection) error
	setReadTimeout(time.Duration)
}

type TubeIf interface {
	io.Reader
	tubeInner
}

type TubeDirection uint8

const (
	Read TubeDirection = iota + 1
	Send
	Both
)

type Tube struct {
	impl   tubeInner
	reader *bufio.Reader

	KeepEnds bool
	Newline  []byte
	//ReadTimeout time.Duration
}

func NewTube(impl TubeIf) *Tube {
	return &Tube{
		impl:    impl,
		reader:  bufio.NewReader(impl),
		Newline: linebreak,
	}
}

func (t *Tube) Clean() {
	_, _ = t.reader.Discard(t.reader.Buffered())
}

func (t *Tube) Read(numBytes int) ([]byte, error) {
	if numBytes <= 0 {
		numBytes = t.reader.Size()
	}
	ret := make([]byte, numBytes)
	n, err := t.reader.Read(ret)
	return ret[:n], err
}

func (t *Tube) ReadAll() ([]byte, error) {
	return io.ReadAll(t.reader)
}

func (t *Tube) ReadAllString() (string, error) {
	all, err := io.ReadAll(t.reader)
	return string(all), err
}

func (t *Tube) ReadLine() ([]byte, error) {
	bs, err := t.reader.ReadBytes('\n')
	l := len(bs)
	if !t.KeepEnds && l > 0 && bs[l-1] == '\n' {
		if l > 1 && bs[l-2] == '\r' {
			l--
		}
		bs = bs[:l-1]
	}
	return bs, err
}

func (t *Tube) ReadLineString() (string, error) {
	line, err := t.ReadLine()
	return string(line), err
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

func (t *Tube) ReadLinePredString(pred func(line string) bool) (string, error) {
	for {
		line, err := t.ReadLineString()
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

func (t *Tube) ReadLineContains(items ...[]byte) ([]byte, error) {
	return t.ReadLinePred(foldFuncItems(bytes.Contains, items...))
}

func (t *Tube) ReadLineContainsString(items ...string) (string, error) {
	return t.ReadLinePredString(foldFuncItemsStr(strings.Contains, items...))
}

func (t *Tube) ReadLineStartsWith(items ...[]byte) ([]byte, error) {
	return t.ReadLinePred(foldFuncItems(bytes.HasPrefix, items...))
}

func (t *Tube) ReadLineStartsWithString(items ...string) (string, error) {
	return t.ReadLinePredString(foldFuncItemsStr(strings.HasPrefix, items...))
}

func (t *Tube) ReadLineEndsWith(items ...[]byte) ([]byte, error) {
	return t.ReadLinePred(foldFuncItems(bytes.HasSuffix, items...))
}

func (t *Tube) ReadLineEndsWithString(items ...string) (string, error) {
	return t.ReadLinePredString(foldFuncItemsStr(strings.HasSuffix, items...))
}

func (t *Tube) ReadLineRegex(regex *regexp.Regexp, exact bool) ([]byte, error) {
	if exact {
		return t.ReadLinePred(regex.Match)
	}
	return t.ReadLinePred(func(line []byte) bool {
		return regex.Find(line) != nil
	})
}

func (t *Tube) ReadLineRegexString(regex *regexp.Regexp, exact bool) (string, error) {
	if exact {
		return t.ReadLinePredString(regex.MatchString)
	}
	return t.ReadLinePredString(func(line string) bool {
		return regex.FindStringIndex(line) != nil
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

func (t *Tube) ReadLinesString(numLines int) ([]string, error) {
	ret := make([]string, 0, numLines)
	for i := 0; i < numLines; i++ {
		line, err := t.ReadLineString()
		if err == nil || len(line) > 0 {
			ret = append(ret, line)
		}
		if err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (t *Tube) ReadN(numBytes int) ([]byte, error) {
	ret := make([]byte, numBytes)
	_, err := io.ReadFull(t.reader, ret)
	return ret, err
}

func (t *Tube) ReadStringN(numRunes int) (string, error) {
	var r []rune
	for i := 0; i < numRunes; i++ {
		ru, _, err := t.reader.ReadRune()
		if err != nil {
			return string(r), err
		}
		r = append(r, ru)
	}
	return string(r), nil
}

func (t *Tube) ReadPred(pred func([]byte) bool) ([]byte, error) {
	var ret []byte
	for {
		b, err := t.reader.ReadByte()
		if err != nil {
			return ret, err
		}
		ret = append(ret, b)
		if pred(ret) {
			return ret, nil
		}
	}
}

func (t *Tube) ReadPredString(pred func(string) bool) (string, error) {
	var ret []rune
	for {
		r, _, err := t.reader.ReadRune()
		if err != nil {
			return string(ret), err
		}
		ret = append(ret, r)
		str := string(ret)
		if pred(str) {
			return str, nil
		}
	}
}

func (t *Tube) ReadRegex(regex *regexp.Regexp, exact bool) ([]byte, error) {
	if exact {
		return t.ReadPred(regex.Match)
	}
	return t.ReadPred(func(line []byte) bool {
		return regex.Find(line) != nil
	})
}

func (t *Tube) ReadRegexString(regex *regexp.Regexp, exact bool) (string, error) {
	if exact {
		return t.ReadPredString(regex.MatchString)
	}
	return t.ReadPredString(func(line string) bool {
		return regex.FindStringIndex(line) != nil
	})
}

// TODO: recvrepeat

func (t *Tube) ReadUntil(dropEnding bool, delims ...byte) ([]byte, error) {
	var ret []byte
	matched := false
	for {
		needRead := t.reader.Buffered()
		if needRead <= 0 {
			needRead = t.reader.Size()
		}
		peek, err := t.reader.Peek(needRead)
		for _, d := range delims {
			index := bytes.IndexByte(peek, d)
			if index != -1 {
				peek = peek[:index+1]
				matched = true
				break
			}
		}
		ret = append(ret, peek...)
		if _, e := t.reader.Discard(len(peek)); e != nil {
			panic(e)
		}
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
}

func (t *Tube) ReadUntilString(dropEnding bool, delims ...rune) (string, error) {
	var ret []rune
	for {
		r, _, err := t.reader.ReadRune()
		if err != nil {
			return string(ret), err
		}
		for _, d := range delims {
			if r == d {
				if !dropEnding {
					ret = append(ret, r)
				}
				return string(ret), nil
			}
		}
		ret = append(ret, r)
	}
}

func (t *Tube) Send(data []byte) (int, error) {
	return t.impl.Write(data)
}

func (t *Tube) SendAfter(data []byte, delims ...byte) (int, error) {
	_, err := t.ReadUntil(false, delims...)
	if err != nil {
		return 0, err
	}
	return t.Send(data)
}

func (t *Tube) SendLine(data []byte) (int, error) {
	return t.Send(append(data, t.Newline...))
}

func (t *Tube) SendLineAfter(data []byte, delims ...byte) (int, error) {
	_, err := t.ReadUntil(false, delims...)
	if err != nil {
		return 0, err
	}
	return t.SendLine(data)
}

func (t *Tube) SendThen(data []byte, delims ...byte) (int, error) {
	s, err := t.Send(data)
	if err == nil {
		_, err = t.ReadUntil(false, delims...)
	}
	return s, err
}

func (t *Tube) SendLineThen(data []byte, delims ...byte) (int, error) {
	s, err := t.SendLine(data)
	if err == nil {
		_, err = t.ReadUntil(false, delims...)
	}
	return s, err
}

func (t *Tube) Close() error {
	return t.impl.Close()
}

func (t *Tube) SpawnProcess(cmd *exec.Cmd) error {
	cmd.Stdin = t.reader
	cmd.Stdout = t.impl
	cmd.Stderr = t.impl
	return cmd.Start()
}

func (t *Tube) Stream() (int64, error) {
	return io.Copy(os.Stdout, t.reader)
}

func (t *Tube) ConnectInput(other *Tube) {
	go func() {
		_, _ = io.Copy(t.impl, other.reader)
	}()
}

func (t *Tube) ConnectOutput(other *Tube) {
	go func() {
		_, _ = io.Copy(other.impl, t.reader)
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
		_, _ = t.reader.WriteTo(os.Stdout)
	}()

	_, _ = os.Stderr.Write(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	for ctx.Err() == nil && scanner.Scan() {
		_, _ = t.SendLine(scanner.Bytes())
		_, _ = os.Stderr.Write(prompt)
	}
}

// TODO unread

func (t *Tube) Shutdown(direction TubeDirection) error {
	return t.impl.shutdown(direction)
}

func (t *Tube) CanRead() bool {
	_, err := t.reader.ReadByte()
	return err == nil
}
