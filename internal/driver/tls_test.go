package driver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func selfSigned(t *testing.T) tls.Certificate {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "x"}, NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k}
}

// Servidor que solo habla TLS 1.0: el cliente por defecto debe fallar y el legacy conectar.
func TestLegacyTLSHandshake(t *testing.T) {
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
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { c.(*tls.Conn).Handshake(); c.Close() }()
		}
	}()
	dial := func(cfg *tls.Config) error {
		c, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", ln.Addr().String(), cfg)
		if err == nil {
			c.Close()
		}
		return err
	}
	if err := dial(&tls.Config{InsecureSkipVerify: true}); err == nil {
		t.Error("el cliente por defecto no debería negociar TLS 1.0")
	}
	if err := dial(legacyTLSConfig(tls.VersionTLS10)); err != nil {
		t.Errorf("el cliente legacy debería conectar: %v", err)
	}
}
