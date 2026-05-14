// SSH key manager — mengelola ~/.ssh/authorized_keys untuk system user
// (root + SFTP users yang dibuat panel).
package sshkeys

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/users", h.users)
	r.Get("/", h.list)
	r.Post("/", h.add)
	r.Delete("/", h.remove)
}

type handler struct{ store *db.Store }

type Key struct {
	Type        string `json:"type"`
	Comment     string `json:"comment"`
	Fingerprint string `json:"fingerprint"` // SHA256:abc...
}

// users mengembalikan user OS yang boleh dikelola key-nya: root + sftp_users.
func (h *handler) users(c *fiber.Ctx) error {
	out := []string{"root"}
	rows, err := h.store.DB().Query(`SELECT username FROM sftp_users ORDER BY username`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err == nil {
				out = append(out, u)
			}
		}
	}
	return c.JSON(out)
}

func (h *handler) list(c *fiber.Ctx) error {
	user := c.Query("user", "root")
	if err := h.allowedUser(user); err != nil {
		return err
	}
	path, err := keysPath(user)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	keys, err := readKeys(path)
	if err != nil && !os.IsNotExist(err) {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(keys)
}

type addReq struct {
	User string `json:"user"`
	Key  string `json:"key"`
}

func (h *handler) add(c *fiber.Ctx) error {
	var body addReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if body.User == "" {
		body.User = "root"
	}
	if err := h.allowedUser(body.User); err != nil {
		return err
	}
	key := strings.TrimSpace(body.Key)
	if _, _, err := parseKey(key); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "key invalid: "+err.Error())
	}
	path, err := keysPath(body.User)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if err := appendKey(path, key, body.User); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) remove(c *fiber.Ctx) error {
	user := c.Query("user", "root")
	fp := c.Query("fingerprint")
	if fp == "" {
		return fiber.NewError(fiber.StatusBadRequest, "fingerprint required")
	}
	if err := h.allowedUser(user); err != nil {
		return err
	}
	path, err := keysPath(user)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if err := removeKey(path, fp); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) allowedUser(user string) error {
	if user == "root" {
		return nil
	}
	var n int
	if err := h.store.DB().QueryRow(`SELECT COUNT(*) FROM sftp_users WHERE username=?`, user).Scan(&n); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if n == 0 {
		return fiber.NewError(fiber.StatusForbidden, "user not managed by panel")
	}
	return nil
}

// ---------- file helpers ----------

func keysPath(user string) (string, error) {
	home := "/root"
	if user != "root" {
		home = filepath.Join("/home", user)
	}
	return filepath.Join(home, ".ssh", "authorized_keys"), nil
}

func readKeys(path string) ([]Key, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []Key{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		typ, fp, err := parseKey(line)
		if err != nil {
			continue
		}
		comment := keyComment(line)
		out = append(out, Key{Type: typ, Comment: comment, Fingerprint: fp})
	}
	return out, nil
}

func appendKey(path, key, user string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Dedup by fingerprint.
	if existing, err := readKeys(path); err == nil {
		_, newFp, _ := parseKey(key)
		for _, k := range existing {
			if k.Fingerprint == newFp {
				return fmt.Errorf("key already present")
			}
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(strings.TrimRight(key, "\n") + "\n"); err != nil {
		return err
	}
	// Permission hardening untuk SSH (server reject ACL longgar).
	_ = os.Chmod(path, 0o600)
	_ = os.Chmod(dir, 0o700)
	if user != "root" {
		_ = exec.Command("chown", "-R", user+":"+user, dir).Run()
	}
	return nil
}

func removeKey(path, fingerprint string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	removed := false
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			buf.WriteString(line + "\n")
			continue
		}
		_, fp, err := parseKey(trimmed)
		if err == nil && fp == fingerprint {
			removed = true
			continue
		}
		buf.WriteString(line + "\n")
	}
	_ = f.Close()
	if !removed {
		return fmt.Errorf("key not found")
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

// parseKey memvalidasi format & menghitung fingerprint SHA256.
//
// Format authorized_keys: "<type> <base64-blob> [comment]".
// Type yang diakui: ssh-rsa, ssh-ed25519, ecdsa-sha2-nistp256/384/521,
// sk-ssh-ed25519@openssh.com, sk-ecdsa-sha2-nistp256@openssh.com.
func parseKey(line string) (string, string, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", errors.New("expected '<type> <key>'")
	}
	typ := parts[0]
	if !validKeyType(typ) {
		return "", "", fmt.Errorf("unsupported key type %q", typ)
	}
	blob, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", errors.New("base64 invalid")
	}
	sum := sha256.Sum256(blob)
	fp := "SHA256:" + strings.TrimRight(base64.StdEncoding.EncodeToString(sum[:]), "=")
	return typ, fp, nil
}

func keyComment(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts[2:], " ")
}

func validKeyType(t string) bool {
	switch t {
	case "ssh-rsa", "ssh-ed25519",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com":
		return true
	}
	return false
}
