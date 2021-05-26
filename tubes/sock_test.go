package tubes

import (
	"bytes"
	"crypto/tls"
	"testing"
)

func TestRemoteApple(t *testing.T) {
	conn, err := tls.Dial("tcp", "apple.com:443", nil)
	if err != nil {
		t.Fatal(err)
	}
	tube := FromConn(conn)
	_, err = tube.Send([]byte("GET \r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := tube.RecvN(4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte("HTTP")) {
		t.Fatal("HTTP header mismatch")
	}
}
