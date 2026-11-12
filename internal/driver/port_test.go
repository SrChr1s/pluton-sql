package driver

import (
	"strconv"
	"strings"
	"testing"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

// Un puerto sin indicar (0) usa el predeterminado del motor; uno explícito se respeta.
func TestDSNDefaultPort(t *testing.T) {
	for _, name := range []string{"postgres", "mysql", "mssql"} {
		d, err := Get(name)
		if err != nil {
			t.Fatal(err)
		}
		want := ":" + strconv.Itoa(d.DefaultPort())
		if dsn := d.DSN(config.ConnectionProfile{Driver: name, Host: "h", User: "u"}, "pw", ""); !strings.Contains(dsn, "h"+want) {
			t.Errorf("%s sin puerto: %q no contiene h%s", name, dsn, want)
		}
		if dsn := d.DSN(config.ConnectionProfile{Driver: name, Host: "h", Port: 4242, User: "u"}, "pw", ""); !strings.Contains(dsn, "h:4242") {
			t.Errorf("%s con puerto 4242: %q", name, dsn)
		}
	}
}
