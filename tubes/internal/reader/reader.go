package reader

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

type Reader struct {
	reader     *bufio.Reader
	setReadDdl func(time.Time) error

	ReadTimeout time.Duration
}

func NewReader(reader interface {
	io.Reader
	SetReadDeadline(time.Time) error
}) *Reader {
	return &Reader{
		reader:     bufio.NewReader(reader),
		setReadDdl: reader.SetReadDeadline,
	}
}

func (r *Reader) setReadDeadline() error {
	if r.ReadTimeout > 0 {
		return r.setReadDdl(time.Now().Add(r.ReadTimeout))
	}
	return r.setReadDdl(time.Time{})
}

func (r *Reader) Read(numBytes int) ([]byte, error) {
	if err := r.setReadDeadline(); err != nil {
		return nil, err
	}

	b := make([]byte, numBytes)
	n, err := r.reader.Read(b)
	return b[:n], err
}

func (r *Reader) ReadN(numBytes int) ([]byte, error) {
	if err := r.setReadDeadline(); err != nil {
		return nil, err
	}

	ret := make([]byte, numBytes)
	n, err := io.ReadFull(r.reader, ret)
	return ret[:n], err
}

func (r *Reader) ReadRepeat() ([]byte, error) {
	if err := r.setReadDdl(time.Time{}); err != nil {
		return nil, err
	}

	return io.ReadAll(r.reader)
}

func (r *Reader) ReadToWithContext(ctx context.Context, w io.Writer) (int64, error) {
	if err := r.setReadDdl(time.Time{}); err != nil {
		return 0, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		_ = r.setReadDdl(time.Now())
	}()

	return io.Copy(w, r.reader)
}

func (r *Reader) ReadUntil(dropEnding bool, delims ...[]byte) ([]byte, error) {
	if err := r.setReadDeadline(); err != nil {
		return nil, err
	}

	var ret []byte
	for {
		b, err := r.reader.ReadByte()
		if err != nil {
			return ret, err
		}

		ret = append(ret, b)
		for _, delim := range delims {
			if bytes.HasSuffix(ret, delim) {
				if dropEnding {
					return ret[:len(ret)-len(delim)], nil
				} else {
					return ret, nil
				}
			}
		}
	}
}

func (r *Reader) ReadPred(pred func([]byte) bool) ([]byte, error) {
	if err := r.setReadDeadline(); err != nil {
		return nil, err
	}

	var ret []byte
	for {
		b, err := r.reader.ReadByte()
		if err != nil {
			return ret, err
		}

		ret = append(ret, b)
		if pred(ret) {
			return ret, nil
		}
	}
}

func (r *Reader) CanRead() error {
	if err := r.setReadDeadline(); err != nil {
		return err
	}

	if _, err := r.reader.ReadByte(); err != nil {
		return err
	}
	return r.reader.UnreadByte()
}

func (r *Reader) Stream(lineByLine bool) error {
	if err := r.setReadDdl(time.Time{}); err != nil {
		return err
	}

	if !lineByLine {
		_, err := io.Copy(os.Stdout, r.reader)
		return err
	}

	scanner := bufio.NewScanner(r.reader)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	return scanner.Err()
}
