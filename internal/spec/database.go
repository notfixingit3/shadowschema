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
	sqlDriverPGX   = "pgx"
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
	db, err := sql.Open(sqlDriverPGX, dsn)
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
	path, err := sqliteDBPath()
	if err != nil {
		return nil, "", err
	}

	db, err := sql.Open(driverSQLite, path)
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("sqlite pragma: %w", err)
	}

	if err := initSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, "", err
	}

	log.Printf("[INFO] Using SQLite database at %q", path)
	return db, driverSQLite, nil
}

func sqliteDBPath() (string, error) {
	path := strings.TrimSpace(os.Getenv("SHADOWSCHEMA_DB_PATH"))
	if path == "" {
		return "./shadowschema.db", nil
	}
	if strings.ContainsAny(path, "\r\n\x00") {
		return "", fmt.Errorf("invalid sqlite path")
	}
	return path, nil
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
			host TEXT NOT NULL DEFAULT '',
			first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(session_id, host, header_name, token_value)
		)`,
		`CREATE TABLE IF NOT EXISTS discovered_domains (
			session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			host TEXT NOT NULL,
			first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(session_id, host)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("postgres schema: %w", err)
		}
	}
	if err := migrateAuthVaultHostPostgres(db); err != nil {
		return err
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
		host TEXT DEFAULT '',
		first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(session_id, host, header_name, token_value)
	)`)
	if err != nil {
		return fmt.Errorf("sqlite auth_vault table: %w", err)
	}

	if err := migrateAuthVaultHostSQLite(db); err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS discovered_domains (
		session_id INTEGER,
		host TEXT,
		first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(session_id, host)
	)`)
	if err != nil {
		return fmt.Errorf("sqlite discovered_domains table: %w", err)
	}
	return nil
}

func migrateAuthVaultHostPostgres(db *sql.DB) error {
	// Best-effort upgrade for pre-host-scoped vault tables.
	_, _ = db.Exec(`ALTER TABLE auth_vault ADD COLUMN IF NOT EXISTS host TEXT NOT NULL DEFAULT ''`)
	// Recreate unique constraint to include host (ignore failures if already correct).
	_, _ = db.Exec(`ALTER TABLE auth_vault DROP CONSTRAINT IF EXISTS auth_vault_session_id_header_name_token_value_key`)
	_, _ = db.Exec(`ALTER TABLE auth_vault DROP CONSTRAINT IF EXISTS auth_vault_session_id_host_header_name_token_value_key`)
	_, err := db.Exec(`
		DO $$ BEGIN
			ALTER TABLE auth_vault
			ADD CONSTRAINT auth_vault_session_host_header_token_key
			UNIQUE (session_id, host, header_name, token_value);
		EXCEPTION WHEN duplicate_table OR duplicate_object THEN NULL;
		END $$;
	`)
	if err != nil {
		// Non-fatal on older Postgres variants — UNIQUE may already exist via CREATE TABLE.
		log.Printf("[WARN] auth_vault unique constraint migration: %v", err)
	}
	return nil
}

func migrateAuthVaultHostSQLite(db *sql.DB) error {
	// Ensure host column exists on older DBs created without it.
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('auth_vault') WHERE name = 'host'`).Scan(&count)
	if err != nil {
		// pragma_table_info may fail on very old SQLite — try ALTER and rebuild.
		count = 0
	}
	if count == 0 {
		_, _ = db.Exec(`ALTER TABLE auth_vault ADD COLUMN host TEXT DEFAULT ''`)
	}

	// Rebuild table if unique index still lacks host (detect via index list / recreate always safe with OR IGNORE copy).
	var needsRebuild bool
	rows, err := db.Query(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'auth_vault'`)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			var createSQL string
			if err := rows.Scan(&createSQL); err == nil {
				// Old unique without host: UNIQUE(session_id, header_name, token_value)
				if strings.Contains(createSQL, "UNIQUE(session_id, header_name, token_value)") &&
					!strings.Contains(createSQL, "UNIQUE(session_id, host, header_name, token_value)") {
					needsRebuild = true
				}
			}
		}
	}
	if !needsRebuild {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite vault migrate begin: %w", err)
	}
	stmts := []string{
		`CREATE TABLE auth_vault_new (
			session_id INTEGER,
			header_name TEXT,
			token_value TEXT,
			host TEXT DEFAULT '',
			first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(session_id, host, header_name, token_value)
		)`,
		`INSERT OR IGNORE INTO auth_vault_new (session_id, header_name, token_value, host, first_seen)
			SELECT session_id, header_name, token_value, COALESCE(host, ''), first_seen FROM auth_vault`,
		`DROP TABLE auth_vault`,
		`ALTER TABLE auth_vault_new RENAME TO auth_vault`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("sqlite vault migrate: %w", err)
		}
	}
	return tx.Commit()
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

func (s *SpecManager) saveVaultCredential(headerName, tokenValue, host string) error {
	host = normalizeHost(host)
	if s.dbDriver == driverPostgres {
		_, err := s.dbExec(
			`INSERT INTO auth_vault (session_id, header_name, token_value, host) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			s.SessionID, headerName, tokenValue, host,
		)
		return err
	}

	_, err := s.dbExec(
		`INSERT OR IGNORE INTO auth_vault (session_id, header_name, token_value, host) VALUES (?, ?, ?, ?)`,
		s.SessionID, headerName, tokenValue, host,
	)
	return err
}