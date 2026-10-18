package service

import (
	"testing"
)

type rec struct {
	profile, database, sql, err string
	ms, rows                    int64
	isQuery                     bool
}

type fakeRecorder struct{ got []rec }

func (f *fakeRecorder) Record(profile, database, sql string, ms, rows int64, isQuery bool, errMsg string) {
	f.got = append(f.got, rec{profile, database, sql, errMsg, ms, rows, isQuery})
}

func TestQueryServiceRecordsHistory(t *testing.T) {
	_, db := newData(t)
	r := &fakeRecorder{}
	q := NewQueryService(fakePools{db, mustDialect(t)}, r)

	if _, err := q.Execute("q1", "p", "main", "SELECT * FROM items WHERE id <= 3"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Execute("q2", "p", "main", "UPDATE items SET qty = 9 WHERE id <= 4"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Execute("q3", "p", "main", "SELEC mal"); err == nil {
		t.Fatal("el SQL inválido debía fallar")
	}
	if len(r.got) != 3 {
		t.Fatalf("se esperaban 3 registros (éxito, éxito y error), hay %d: %+v", len(r.got), r.got)
	}
	if g := r.got[0]; g.rows != 3 || !g.isQuery || g.err != "" || g.profile != "p" || g.database != "main" {
		t.Errorf("SELECT: %+v", g)
	}
	if g := r.got[1]; g.rows != 4 || g.isQuery || g.err != "" {
		t.Errorf("UPDATE (filas afectadas): %+v", g)
	}
	if g := r.got[2]; g.err == "" || g.sql != "SELEC mal" {
		t.Errorf("los errores también se registran: %+v", g)
	}
}

func TestQueryServiceWorksWithoutRecorder(t *testing.T) {
	_, db := newData(t)
	q := NewQueryService(fakePools{db, mustDialect(t)}, nil)
	res, err := q.Execute("q", "p", "", "SELECT 1")
	if err != nil || len(res.Rows) != 1 {
		t.Errorf("sin Recorder debe funcionar igual: %v %+v", err, res)
	}
}
