package service

import (
	"reflect"
	"testing"
)

func TestSplitGoBatches(t *testing.T) {
	cases := []struct {
		name, in string
		want     []batch
	}{
		{"sin GO", "SELECT 1", []batch{{"SELECT 1", 1}}},
		{"USE y ALTER", "-- x\nUSE [A]  -- c\nGO\nALTER PROCEDURE p AS\nSELECT 1\n", []batch{
			{"-- x\nUSE [A]  -- c\n", 1}, {"ALTER PROCEDURE p AS\nSELECT 1\n", 1}}},
		{"GO con minúsculas, CRLF y comentario", "a\r\n  go -- fin\r\nb", []batch{{"a\r\n", 1}, {"b", 1}}},
		{"GO n", "INSERT x\nGO 3\n", []batch{{"INSERT x\n", 3}}},
		{"vacíos", "GO\n\nGO\nSELECT 1\nGO", []batch{{"SELECT 1\n", 1}}},
		{"GO en cadena", "SELECT 'a\nGO\nb'\nGO\nSELECT 2", []batch{{"SELECT 'a\nGO\nb'\n", 1}, {"SELECT 2", 1}}},
		{"GO en comentario de bloque", "/* x\nGO\n*/ SELECT 1", []batch{{"/* x\nGO\n*/ SELECT 1", 1}}},
		{"GO en corchetes", "SELECT [a\nGO\nb]", []batch{{"SELECT [a\nGO\nb]", 1}}},
		{"no es GO", "GOTO x\nGOSUB\nSELECT go FROM t", []batch{{"GOTO x\nGOSUB\nSELECT go FROM t", 1}}},
		{"comilla escapada", "SELECT 'it''s'\nGO\nSELECT 2", []batch{{"SELECT 'it''s'\n", 1}, {"SELECT 2", 1}}},
	}
	for _, c := range cases {
		if got := splitGoBatches(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}
