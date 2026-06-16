package spec

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const (
	driverPostgres = "postgres"
	driverSQLite   = "sqlite"
)

func openDatabase() (*sql.DB, string, error) {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		return openPostgres(dsn)
	}
	return openSQLite()
}

func openPostgres(dsn string) (*sql.DB, string, error) {
	db, err := sql.Open(driverPostgres, dsn)
	if err != nil {
		return nil, "", fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	var pingErr error
	for attempt := 1; attempt <= 30; attempt++ {
		pingErr = db.Ping()
		if pingErr == nil {
			if err := initPostgresSchema(db); err != nil {
				_ = db.Close()
				return nil, "", err
			}
			log.Printf("[INFO] Connected to PostgreSQL")
			return db, driverPostgres, nil
		}
		time.Sleep(time.Second)
	}

	_ = db.Close()
	return nil, "", fmt.Errorf("postgres unavailable: %w", pingErr)
}

func openSQLite() (*sql.DB, string, error) {
	path := strings.TrimSpace(os.Getenv("SHADOWSCHEMA_DB_PATH"))
	if path == "" {
		path = "./shadowschema.db"
	}

	db, err := sql.Open(driverSQLite, path)
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite: %w", err)
	}

	if err := initSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, "", err
	}

	log.Printf("[INFO] Using SQLite database at %s", path)
	return db, driverSQLite, nil
}

func initPostgresSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			target TEXT NOT NULL,
			spec_json TEXT NOT NULL,
			ignore_rules TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS auth_vault (
			session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			header_name TEXT NOT NULL,
			token_value TEXT NOT NULL,
			first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(session_id, header_name, token_value)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("postgres schema: %w", err)
		}
	}
	return nil
}

func initSQLiteSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		target TEXT,
		spec_json TEXT,
		ignore_rules TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("sqlite sessions table: %w", err)
	}

	_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN ignore_rules TEXT DEFAULT ''`)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS auth_vault (
		session_id INTEGER,
		header_name TEXT,
		token_value TEXT,
		first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(session_id, header_name, token_value)
	)`)
	if err != nil {
		return fmt.Errorf("sqlite auth_vault table: %w", err)
	}
	return nil
}

func rebindQuery(driver, query string) string {
	if driver != driverPostgres {
		return query
	}

	var b strings.Builder
	arg := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(arg))
			arg++
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

func (s *SpecManager) dbExec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(rebindQuery(s.dbDriver, query), args...)
}

func (s *SpecManager) dbQuery(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(rebindQuery(s.dbDriver, query), args...)
}

func (s *SpecManager) dbQueryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(rebindQuery(s.dbDriver, query), args...)
}

func (s *SpecManager) insertSession(name, target, ignoreRules, specJSON string) (int, error) {
	if s.dbDriver == driverPostgres {
		var id int
		err := s.db.QueryRow(
			`INSERT INTO sessions (name, target, ignore_rules, spec_json) VALUES ($1, $2, $3, $4) RETURNING id`,
			name, target, ignoreRules, specJSON,
		).Scan(&id)
		return id, err
	}

	res, err := s.dbExec(
		`INSERT INTO sessions (name, target, ignore_rules, spec_json) VALUES (?, ?, ?, ?)`,
		name, target, ignoreRules, specJSON,
	)
	if err != nil {
		return 0, err
	}
	lastID, err := res.LastInsertId()
	return int(lastID), err
}

func (s *SpecManager) saveVaultCredential(headerName, tokenValue string) error {
	if s.dbDriver == driverPostgres {
		_, err := s.dbExec(
			`INSERT INTO auth_vault (session_id, header_name, token_value) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			s.SessionID, headerName, tokenValue,
		)
		return err
	}

	_, err := s.dbExec(
		`INSERT OR IGNORE INTO auth_vault (session_id, header_name, token_value) VALUES (?, ?, ?)`,
		s.SessionID, headerName, tokenValue,
	)
	return err
}