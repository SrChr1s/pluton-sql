package driver

import (
	"context"
	"log"
	"net"
	"time"
)

// spyDialer registra cada lectura/escritura TCP de una conexión SQL Server
// (solo tamaños y primeros bytes; los datos TLS ya viajan cifrados). Se activa con PLUTON_DEBUG.
type spyDialer struct{}

func (spyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	c, err := (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	return &spyConn{Conn: c, start: time.Now()}, nil
}

type spyConn struct {
	net.Conn
	start time.Time
	n     int
}

func (s *spyConn) logf(dir string, b []byte, n int, err error) {
	s.n++
	head := b[:min(n, 10)]
	log.Printf("[tcp] +%4dms #%02d %s n=%-5d err=%v first=% x", time.Since(s.start).Milliseconds(), s.n, dir, n, err, head)
}

func (s *spyConn) Write(b []byte) (int, error) {
	n, err := s.Conn.Write(b)
	s.logf("→ envía  ", b, n, err)
	return n, err
}

func (s *spyConn) Read(b []byte) (int, error) {
	n, err := s.Conn.Read(b)
	s.logf("← recibe ", b, n, err)
	return n, err
}
