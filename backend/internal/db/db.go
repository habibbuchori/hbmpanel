package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password      TEXT    NOT NULL,
			role          TEXT    NOT NULL DEFAULT 'admin',
			totp_secret   TEXT,
			totp_enabled  INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
		`CREATE TABLE IF NOT EXISTS backups (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			kind        TEXT    NOT NULL,         -- 'site' | 'db'
			target      TEXT    NOT NULL,         -- site id (as string) | db name
			label       TEXT    NOT NULL,         -- human readable
			file_path   TEXT    NOT NULL UNIQUE,
			size_bytes  INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS git_deployments (
			site_id           INTEGER PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
			repo              TEXT    NOT NULL,
			branch            TEXT    NOT NULL DEFAULT 'main',
			deploy_cmd        TEXT,
			last_commit       TEXT,
			last_deployed_at  DATETIME,
			last_output       TEXT,
			updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, q[:40])
		}
	}
	// Idempotent column additions for existing DBs (pre-P2).
	for _, alter := range []string{
		`ALTER TABLE users ADD COLUMN totp_secret TEXT`,
		`ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := s.db.Exec(alter); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate alter: %w (%s)", err, alter)
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

// FindUser → mengambil user untuk login (termasuk status 2FA).
func (s *Store) FindUser(username string) (id int64, hash, role string, totpEnabled bool, err error) {
	row := s.db.QueryRow(`SELECT id, password, role, COALESCE(totp_enabled,0) FROM users WHERE username = ?`, username)
	var en int
	err = row.Scan(&id, &hash, &role, &en)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", "", false, ErrNotFound
	}
	totpEnabled = en != 0
	return
}

func (s *Store) GetUserTOTP(id int64) (secret string, enabled bool, err error) {
	var sec sql.NullString
	var en int
	err = s.db.QueryRow(`SELECT totp_secret, COALESCE(totp_enabled,0) FROM users WHERE id=?`, id).Scan(&sec, &en)
	return sec.String, en != 0, err
}

func (s *Store) SetUserTOTP(id int64, secret string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	var sec any = secret
	if secret == "" {
		sec = nil
	}
	_, err := s.db.Exec(`UPDATE users SET totp_secret=?, totp_enabled=? WHERE id=?`, sec, en, id)
	return err
}

func (s *Store) UpdatePassword(id int64, newHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password=? WHERE id=?`, newHash, id)
	return err
}

func (s *Store) UpdatePasswordByName(username, newHash string) (int64, error) {
	res, err := s.db.Exec(`UPDATE users SET password=? WHERE username=?`, newHash, username)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---------- settings ----------

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value)
	return err
}

// ---------- audit log ----------

type AuditRow struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Action    string    `json:"action"`
	Target    string    `json:"target,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) WriteAudit(userID int64, action, target, detail string) {
	var uid any = userID
	if userID == 0 {
		uid = nil
	}
	// Fire-and-forget; tidak boleh fatal jika DB sibuk.
	_, _ = s.db.Exec(
		`INSERT INTO audit_log (user_id, action, target, detail) VALUES (?, ?, ?, ?)`,
		uid, action, target, detail,
	)
}

func (s *Store) ListAudit(limit, offset int) ([]AuditRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.Query(`SELECT id, user_id, action, COALESCE(target,''), COALESCE(detail,''), created_at
		FROM audit_log ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var a AuditRow
		var uid sql.NullInt64
		if err := rows.Scan(&a.ID, &uid, &a.Action, &a.Target, &a.Detail, &a.CreatedAt); err != nil {
			continue
		}
		if uid.Valid {
			a.UserID = &uid.Int64
		}
		out = append(out, a)
	}
	return out, nil
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
