package tubes

import (
	"bytes"
	"io"
	"testing"
	"time"
)

type NullTube struct {
	data []byte
}

func (n *NullTube) Read(v []byte) (int, error) {
	if len(n.data) <= 0 {
		return 0, io.EOF
	}
	b := copy(v, n.data)
	n.data = n.data[b:]
	return b, nil
}

func (NullTube) Write([]byte) (int, error) {
	return 0, nil
}

func (NullTube) Close() error {
	return nil
}

func (NullTube) Shutdown(TubeDirection) error {
	return nil
}

func (NullTube) SetReadDeadline(time.Time) error {
	return nil
}

func TestRecv(t *testing.T) {
	h := []byte("Hello, world")
	null := &NullTube{data: h}
	tube := NewTube(null)
	recv, _ := tube.Read(4096)
	if !bytes.Equal(h, recv) {
		t.Fatal("data mismatch")
	}

	woohoo := []byte("Woohoo")
	null.data = append(null.data, woohoo...)
	recv, _ = tube.Read(4096)
	if !bytes.Equal(woohoo, recv) {
		t.Fatal("data mismatch")
	}
}
