package tubes

import (
	"net"
)

type sock struct {
	net.Conn
}

func FromConn(conn net.Conn) *Tube {
	return NewTube(sock{conn})
}

func (s sock) Shutdown(direction TubeDirection) (err error) {
	switch direction {
	case Read:
		if closeReader, ok := s.Conn.(interface{ CloseRead() error }); ok {
			return closeReader.CloseRead()
		}
	case Send:
		if closeWriter, ok := s.Conn.(interface{ CloseWrite() error }); ok {
			return closeWriter.CloseWrite()
		}
	}
	return nil
}
