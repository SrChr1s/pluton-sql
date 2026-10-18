package driver

import (
	"crypto/tls"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// countingConn cuenta las escrituras TCP (una por registro TLS).
type countingConn struct {
	net.Conn
	mu     sync.Mutex
	writes []int
}

func (c *countingConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	c.writes = append(c.writes, len(b))
	c.mu.Unlock()
	return c.Conn.Write(b)
}

// Con TLS 1.0 + CBC, Go parte el primer envío en un registro de 1 byte que
// SQL Server 2008 no tolera. `make test` compila con el parche y exige un único registro.
func TestTLS10NoRecordSplit(t *testing.T) {
	t.Setenv("GODEBUG", "tls10server=1")
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{selfSigned(t)}, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS10,
		CipherSuites: allCipherSuites(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		buf := make([]byte, 256)
		c.Read(buf)
		c.Close()
	}()

	raw, err := net.DialTimeout("tcp", ln.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cc := &countingConn{Conn: raw}
	cfg := legacyTLSConfig(tls.VersionTLS10)
	cfg.CipherSuites = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA}
	conn := tls.Client(cc, cfg)
	if err := conn.Handshake(); err != nil {
		t.Fatal(err)
	}
	cc.mu.Lock()
	before := len(cc.writes)
	cc.mu.Unlock()
	conn.Write(make([]byte, 100))
	cc.mu.Lock()
	records := len(cc.writes) - before
	cc.mu.Unlock()
	conn.Close()

	if os.Getenv("PLUTON_EXPECT_NOSPLIT") == "" {
		t.Skipf("sin overlay: %d registro(s); usa `make test` para exigir 1", records)
	}
	if records != 1 {
		t.Fatalf("la primera escritura salió en %d registros TLS, esperaba 1 (¿falta el overlay?)", records)
	}
}
