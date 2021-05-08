package tubes

import (
	"net"
	"time"
)

type sock struct {
	net.Conn
}

func FromConn(conn net.Conn) *Tube {
	return NewTube(&sock{conn})
}

func (s *sock) shutdown(TubeDirection) error {
	return s.Close()
}

func (s *sock) setReadTimeout(timeout time.Duration) {
	var t time.Time

	if timeout > 0 {
		t = time.Now().Add(timeout)
	}

	_ = s.SetReadDeadline(t)
}
