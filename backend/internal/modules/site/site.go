package site

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
	"github.com/habibbuchori/hbmpanel/internal/modules/caddy"
)

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/:id", h.get)
	r.Delete("/:id", h.delete)
	r.Post("/:id/restart", h.restart)
	r.Post("/:id/enable", h.enable)
	r.Post("/:id/disable", h.disable)
	r.Get("/:id/env", h.getEnv)
	r.Put("/:id/env", h.putEnv)
}

type handler struct{ store *db.Store }

type Site struct {
	ID         int64             `json:"id"`
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	RootPath   string            `json:"root_path"`
	PHPVersion string            `json:"php_version,omitempty"`
	NodePort   int               `json:"node_port,omitempty"`
	SSL        bool              `json:"ssl"`
	Status     string            `json:"status"`
	EnvVars    map[string]string `json:"env_vars,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

func (h *handler) list(c *fiber.Ctx) error {
	rows, err := h.store.DB().Query(`SELECT id, domain, type, root_path, COALESCE(php_version,''),
		COALESCE(node_port,0), ssl, status, COALESCE(env_vars,''), created_at FROM sites ORDER BY id DESC`)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := []Site{}
	for rows.Next() {
		var s Site
		var envStr string
		if err := rows.Scan(&s.ID, &s.Domain, &s.Type, &s.RootPath,
			&s.PHPVersion, &s.NodePort, &s.SSL, &s.Status, &envStr, &s.CreatedAt); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if envStr != "" {
			_ = json.Unmarshal([]byte(envStr), &s.EnvVars)
		}
		out = append(out, s)
	}
	return c.JSON(out)
}

type createReq struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	RootPath   string            `json:"root_path"`
	PHPVersion string            `json:"php_version,omitempty"`
	NodePort   int               `json:"node_port,omitempty"`
	SSL        bool              `json:"ssl"`
	EnvVars    map[string]string `json:"env_vars,omitempty"`
}

func (h *handler) create(c *fiber.Ctx) error {
	var body createReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if body.Domain == "" || body.Type == "" {
		return fiber.NewError(fiber.StatusBadRequest, "domain & type required")
	}
	if body.Type == "php" || body.Type == "static" {
		if body.RootPath == "" {
			return fiber.NewError(fiber.StatusBadRequest, "root_path required")
		}
		if err := os.MkdirAll(body.RootPath, 0o755); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	}

	envJSON := ""
	if len(body.EnvVars) > 0 {
		b, _ := json.Marshal(body.EnvVars)
		envJSON = string(b)
	}

	res, err := h.store.DB().Exec(`INSERT INTO sites
		(domain, type, root_path, php_version, node_port, ssl, env_vars, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active')`,
		body.Domain, body.Type, body.RootPath,
		nullStr(body.PHPVersion), nullInt(body.NodePort), body.SSL, nullStr(envJSON))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	id, _ := res.LastInsertId()

	if err := caddy.WriteSite(caddy.SiteConfig{
		Domain:   body.Domain,
		Type:     body.Type,
		Root:     body.RootPath,
		NodePort: body.NodePort,
		SSL:      body.SSL,
	}); err != nil {
		// rollback DB
		_, _ = h.store.DB().Exec(`DELETE FROM sites WHERE id = ?`, id)
		return fiber.NewError(fiber.StatusInternalServerError, "caddy: "+err.Error())
	}
	return c.JSON(fiber.Map{"id": id, "ok": true})
}

func (h *handler) get(c *fiber.Ctx) error {
	id := c.Params("id")
	row := h.store.DB().QueryRow(`SELECT id, domain, type, root_path, COALESCE(php_version,''),
		COALESCE(node_port,0), ssl, status, COALESCE(env_vars,''), created_at FROM sites WHERE id = ?`, id)
	var s Site
	var envStr string
	if err := row.Scan(&s.ID, &s.Domain, &s.Type, &s.RootPath,
		&s.PHPVersion, &s.NodePort, &s.SSL, &s.Status, &envStr, &s.CreatedAt); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	if envStr != "" {
		_ = json.Unmarshal([]byte(envStr), &s.EnvVars)
	}
	return c.JSON(s)
}

func (h *handler) delete(c *fiber.Ctx) error {
	id := c.Params("id")
	var domain string
	err := h.store.DB().QueryRow(`SELECT domain FROM sites WHERE id = ?`, id).Scan(&domain)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	if err := caddy.RemoveSite(domain); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "caddy: "+err.Error())
	}
	if _, err := h.store.DB().Exec(`DELETE FROM sites WHERE id = ?`, id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) restart(c *fiber.Ctx) error {
	s, err := h.loadSite(c.Params("id"))
	if err != nil {
		return err
	}
	// Rewrite caddyfile (idempotent) + reload caddy.
	if err := caddy.WriteSite(caddy.SiteConfig{
		Domain:   s.Domain,
		Type:     s.Type,
		Root:     s.RootPath,
		NodePort: s.NodePort,
		SSL:      s.SSL,
	}); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "caddy: "+err.Error())
	}
	switch s.Type {
	case "php":
		_ = exec.Command("systemctl", "reload", "php8.4-fpm").Run()
	case "node":
		// Convention: PM2 app name = site domain. Silent best-effort.
		_ = exec.Command("pm2", "restart", s.Domain).Run()
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) enable(c *fiber.Ctx) error {
	s, err := h.loadSite(c.Params("id"))
	if err != nil {
		return err
	}
	if err := caddy.EnableSite(s.Domain); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "caddy: "+err.Error())
	}
	if _, err := h.store.DB().Exec(`UPDATE sites SET status='active', updated_at=? WHERE id=?`,
		time.Now().UTC(), s.ID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) disable(c *fiber.Ctx) error {
	s, err := h.loadSite(c.Params("id"))
	if err != nil {
		return err
	}
	if err := caddy.DisableSite(s.Domain); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "caddy: "+err.Error())
	}
	if _, err := h.store.DB().Exec(`UPDATE sites SET status='disabled', updated_at=? WHERE id=?`,
		time.Now().UTC(), s.ID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) getEnv(c *fiber.Ctx) error {
	id := c.Params("id")
	var envStr string
	err := h.store.DB().QueryRow(`SELECT COALESCE(env_vars,'') FROM sites WHERE id = ?`, id).Scan(&envStr)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	out := map[string]string{}
	if envStr != "" {
		_ = json.Unmarshal([]byte(envStr), &out)
	}
	return c.JSON(out)
}

func (h *handler) putEnv(c *fiber.Ctx) error {
	id := c.Params("id")
	var body map[string]string
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	b, _ := json.Marshal(body)
	res, err := h.store.DB().Exec(`UPDATE sites SET env_vars=?, updated_at=? WHERE id=?`,
		string(b), time.Now().UTC(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) loadSite(id string) (*Site, error) {
	row := h.store.DB().QueryRow(`SELECT id, domain, type, root_path, COALESCE(php_version,''),
		COALESCE(node_port,0), ssl, status FROM sites WHERE id = ?`, id)
	var s Site
	if err := row.Scan(&s.ID, &s.Domain, &s.Type, &s.RootPath,
		&s.PHPVersion, &s.NodePort, &s.SSL, &s.Status); err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "not found")
	}
	return &s, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return sql.NullString{}
	}
	return s
}
func nullInt(n int) interface{} {
	if n == 0 {
		return sql.NullInt64{}
	}
	return n
}
