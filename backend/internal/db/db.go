package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	d, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	d.SetMaxOpenConns(1) // SQLite single-writer
	if err := d.Ping(); err != nil {
		return nil, err
	}
	return &Store{db: d}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			username    TEXT    NOT NULL UNIQUE,
			password    TEXT    NOT NULL,
			role        TEXT    NOT NULL DEFAULT 'admin',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sites (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			domain      TEXT    NOT NULL UNIQUE,
			type        TEXT    NOT NULL,
			root_path   TEXT    NOT NULL,
			php_version TEXT,
			node_port   INTEGER,
			ssl         BOOLEAN NOT NULL DEFAULT 0,
			env_vars    TEXT,
			status      TEXT    NOT NULL DEFAULT 'pending',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pm_apps (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id     INTEGER REFERENCES sites(id) ON DELETE CASCADE,
			name        TEXT    NOT NULL UNIQUE,
			cwd         TEXT    NOT NULL,
			script      TEXT    NOT NULL,
			instances   INTEGER NOT NULL DEFAULT 1,
			env_vars    TEXT,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sftp_users (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			username    TEXT    NOT NULL UNIQUE,
			site_id     INTEGER REFERENCES sites(id) ON DELETE CASCADE,
			home        TEXT    NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pg_databases (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL UNIQUE,
			owner       TEXT    NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER,
			action      TEXT    NOT NULL,
			target      TEXT,
			detail      TEXT,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS licenses (
			id           INTEGER  PRIMARY KEY AUTOINCREMENT,
			token        TEXT     NOT NULL UNIQUE,
			machine_id   TEXT     NOT NULL,
			plan         TEXT     NOT NULL DEFAULT 'free',
			status       TEXT     NOT NULL DEFAULT 'inactive',
			features     TEXT     NOT NULL DEFAULT '[]',
			activated_at DATETIME,
			expires_at   DATETIME,
			last_checked DATETIME,
			last_ok      DATETIME,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, q[:40])
		}
	}
	return nil
}

// EnsureInitialAdmin membuat user "admin" bila belum ada.
// Mengembalikan plaintext password baru (hanya bila baru dibuat) atau "" bila sudah ada.
func (s *Store) EnsureInitialAdmin(username string) (string, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "", nil
	}

	plain, err := randomPassword(18)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(
		`INSERT INTO users (username, password, role, created_at) VALUES (?, ?, 'admin', ?)`,
		username, string(hash), time.Now().UTC(),
	); err != nil {
		return "", err
	}
	return plain, nil
}

// FindUser → mengambil user untuk login.
func (s *Store) FindUser(username string) (id int64, hash string, role string, err error) {
	row := s.db.QueryRow(`SELECT id, password, role FROM users WHERE username = ?`, username)
	err = row.Scan(&id, &hash, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", "", ErrNotFound
	}
	return
}

var ErrNotFound = errors.New("not found")

type License struct {
	ID          int64
	Token       string
	MachineID   string
	Plan        string
	Status      string
	Features    []string
	ActivatedAt *time.Time
	ExpiresAt   *time.Time
	LastChecked *time.Time
	LastOk      *time.Time
	CreatedAt   time.Time
}

func (s *Store) GetLicense() (*License, error) {
	row := s.db.QueryRow(`
		SELECT id, token, machine_id, plan, status, features, activated_at, expires_at, last_checked, last_ok, created_at
		FROM licenses ORDER BY created_at DESC LIMIT 1
	`)
	var lic License
	var featStr string
	err := row.Scan(&lic.ID, &lic.Token, &lic.MachineID, &lic.Plan, &lic.Status, &featStr,
		&lic.ActivatedAt, &lic.ExpiresAt, &lic.LastChecked, &lic.LastOk, &lic.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if featStr != "" && featStr != "[]" {
		_ = json.Unmarshal([]byte(featStr), &lic.Features)
	}
	return &lic, nil
}

func (s *Store) UpsertLicense(lic *License) error {
	featJSON, _ := json.Marshal(lic.Features)
	_, err := s.db.Exec(`
		INSERT INTO licenses (token, machine_id, plan, status, features, activated_at, expires_at, last_checked, last_ok, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET
			machine_id = excluded.machine_id,
			plan = excluded.plan,
			status = excluded.status,
			features = excluded.features,
			activated_at = excluded.activated_at,
			expires_at = excluded.expires_at,
			last_checked = excluded.last_checked,
			last_ok = excluded.last_ok
	`, lic.Token, lic.MachineID, lic.Plan, lic.Status, string(featJSON),
		lic.ActivatedAt, lic.ExpiresAt, lic.LastChecked, lic.LastOk, lic.CreatedAt)
	return err
}

func (s *Store) TouchLicenseChecked(token string) error {
	_, err := s.db.Exec(`UPDATE licenses SET last_checked = ? WHERE token = ?`, time.Now().UTC(), token)
	return err
}

func (s *Store) UpdateLicenseFromRemote(token, plan string, features []string, expiresAt *time.Time, isValid bool) error {
	status := "inactive"
	if isValid {
		status = "active"
	}
	featJSON, _ := json.Marshal(features)
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		UPDATE licenses
		SET plan = ?, status = ?, features = ?, expires_at = ?, last_checked = ?, last_ok = ?
		WHERE token = ?
	`, plan, status, string(featJSON), expiresAt, now, now, token)
	return err
}

func randomPassword(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

// DB exposes underlying *sql.DB untuk modul lain (read-only convention).
func (s *Store) DB() *sql.DB { return s.db }
