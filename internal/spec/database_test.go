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

func TestRebindQuerySQLiteUnchanged(t *testing.T) {
	query := `SELECT id FROM sessions WHERE target = ?`
	got := rebindQuery(driverSQLite, query)
	if got != query {
		t.Fatalf("rebindQuery() = %q, want %q", got, query)
	}
}