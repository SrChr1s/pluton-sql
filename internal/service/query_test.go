package service

import (
	"strings"
	"testing"
)

func TestReturnsRows(t *testing.T) {
	cases := map[string]bool{
		"select 1":                true,
		"  \n WITH x AS (...)":    true,
		"-- hola\nSELECT 1":       true,
		"/* c */ show tables":     true,
		"(SELECT 1) UNION (...)":  true,
		"INSERT INTO t VALUES(1)": false,
		"update t set a=1":        false,
		"-- solo comentario":      false,
	}
	for q, want := range cases {
		if got := returnsRows(q); got != want {
			t.Errorf("returnsRows(%q)=%v, want %v", q, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if normalize([]byte("hi")) != "hi" {
		t.Error("bytes utf8")
	}
	if normalize([]byte{0xff, 0x00}) != "0xff00" {
		t.Error("bytes binarios")
	}
	big := make([]byte, 1000)
	big[0] = 0xff
	if got := normalize(big).(string); len(got) > 2+2*blobPreview+30 || !strings.HasSuffix(got, "(1000 bytes)") {
		t.Errorf("blob grande sin recortar: %d caracteres", len(got))
	}
	if normalize(int64(1)<<60) != "1152921504606846976" {
		t.Error("int64 grande")
	}
	if normalize(int64(5)) != int64(5) {
		t.Error("int64 pequeño")
	}
}
