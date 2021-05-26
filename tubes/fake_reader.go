package tubes

import (
	"errors"
	"io"
	"time"
)

var TubeReadTimeout = errors.New("tube read timeout")

type recvMode uint8

const (
	read recvMode = iota
	peek
)

type fakeReader struct {
	reader    io.Reader
	readLock  chan struct{}
	readErr   error
	unreadBuf []byte
}

func newFakeReader(r io.Reader) *fakeReader {
	f := &fakeReader{
		reader:   r,
		readLock: make(chan struct{}, 1),
	}
	f.readLock <- struct{}{}
	return f
}

func (f *fakeReader) unrecv(b []byte) {
	<-f.readLock
	f.unreadBuf = append(b, f.unreadBuf...)
	f.readLock <- struct{}{}
}

func (f *fakeReader) discard(len int) {
	<-f.readLock
	f.unreadBuf = f.unreadBuf[len:]
	f.readLock <- struct{}{}
}

func (f *fakeReader) Read(b []byte) (int, error) {
	<-f.readLock
	defer func() {
		f.readLock <- struct{}{}
	}()
	bytesRead := copy(b, f.unreadBuf)
	if bytesRead > 0 {
		f.unreadBuf = f.unreadBuf[:bytesRead]
		return bytesRead, nil
	}
	n, err := f.reader.Read(b)
	return n, err
}

func (f *fakeReader) recv(totalRead int, timeout time.Duration, mode recvMode) ([]byte, error) {
	var timer *time.Timer
	if timeout >= 0 {
		timer = time.NewTimer(timeout)
		select {
		case <-timer.C:
			// FIXME: do we want to return an error?
			return nil, TubeReadTimeout
		case <-f.readLock:
		}
	} else {
		<-f.readLock
	}
	releaseLock := func() {
		if timer != nil {
			timer.Stop()
		}
		f.readLock <- struct{}{}
	}

	buffed := len(f.unreadBuf)
	if buffed > 0 {
		// we have something to return without actual read
		if buffed < totalRead {
			totalRead = buffed
		}

		ret := f.unreadBuf[:totalRead]
		if mode == read {
			f.unreadBuf = f.unreadBuf[totalRead:]
		}
		releaseLock()
		return ret, f.readErr
	}

	buf := make([]byte, totalRead)

	actualRead := func(bytesRead int) ([]byte, error) {
		switch mode {
		case read:
			return buf[:bytesRead], f.readErr
		default:
			f.unreadBuf = buf[:bytesRead]
			return f.unreadBuf, f.readErr
		}
	}

	if timeout <= 0 {
		// short hand when no timeout
		defer releaseLock()
		bytesRead, err := f.reader.Read(buf)
		f.readErr = err
		return actualRead(bytesRead)
	}

	readComplete := make(chan int)
	go func() {
		defer releaseLock()
		bytesRead, err := f.reader.Read(buf)
		f.readErr = err
		if timer.Stop() {
			readComplete <- bytesRead
		} else {
			f.unreadBuf = buf[:bytesRead]
		}
	}()

	select {
	case <-timer.C:
		return nil, f.readErr
	case bytesRead := <-readComplete:
		return actualRead(bytesRead)
	}
}
