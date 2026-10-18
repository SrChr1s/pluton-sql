package driver

import "crypto/tls"

var tlsVersions = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
}

// legacyTLS devuelve v si es una versión mínima inferior a la de Go por defecto
// (1.2) y por tanto requiere configuración especial; si no, "".
func legacyTLS(v string) string {
	if v == "1.0" || v == "1.1" {
		return v
	}
	return ""
}

// allCipherSuites incluye las suites que Go considera inseguras (RSA kex,
// CBC-SHA, 3DES...), necesarias para servidores antiguos.
func allCipherSuites() []uint16 {
	var ids []uint16
	for _, s := range tls.CipherSuites() {
		ids = append(ids, s.ID)
	}
	for _, s := range tls.InsecureCipherSuites() {
		ids = append(ids, s.ID)
	}
	return ids
}

// legacyTLSConfig para servidores antiguos, que suelen usar certificados
// autofirmados o caducados, por lo que no se verifica la cadena.
func legacyTLSConfig(min uint16) *tls.Config {
	return &tls.Config{MinVersion: min, CipherSuites: allCipherSuites(), InsecureSkipVerify: true}
}
