package spec

import "testing"

func TestRebindQueryPostgres(t *testing.T) {
	query := `SELECT id FROM sessions WHERE target = ? AND id = ?`
	got := rebindQuery(driverPostgres, query)
	want := `SELECT id FROM sessions WHERE target = $1 AND id = $2`
	if got != want {
		t.Fatalf("rebindQuery() = %q, want %q", got, want)
	}
}

func TestSQLiteDBPathRejectsUnsafeInput(t *testing.T) {
	t.Setenv("SHADOWSCHEMA_DB_PATH", "/tmp/evil\ninjected")
	if _, err := sqliteDBPath(); err == nil {
		t.Fatal("expected error for unsafe sqlite path")
	}
}

func TestRebindQuerySQLiteUnchanged(t *testing.T) {
	query := `SELECT id FROM sessions WHERE target = ?`
	got := rebindQuery(driverSQLite, query)
	if got != query {
		t.Fatalf("rebindQuery() = %q, want %q", got, query)
	}
}