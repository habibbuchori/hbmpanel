package site

import (
	"database/sql"
	"os"
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
}

type handler struct{ store *db.Store }

type Site struct {
	ID         int64     `json:"id"`
	Domain     string    `json:"domain"`
	Type       string    `json:"type"`
	RootPath   string    `json:"root_path"`
	PHPVersion string    `json:"php_version,omitempty"`
	NodePort   int       `json:"node_port,omitempty"`
	SSL        bool      `json:"ssl"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *handler) list(c *fiber.Ctx) error {
	rows, err := h.store.DB().Query(`SELECT id, domain, type, root_path, COALESCE(php_version,''),
		COALESCE(node_port,0), ssl, status, created_at FROM sites ORDER BY id DESC`)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := []Site{}
	for rows.Next() {
		var s Site
		if err := rows.Scan(&s.ID, &s.Domain, &s.Type, &s.RootPath,
			&s.PHPVersion, &s.NodePort, &s.SSL, &s.Status, &s.CreatedAt); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		out = append(out, s)
	}
	return c.JSON(out)
}

type createReq struct {
	Domain     string `json:"domain"`
	Type       string `json:"type"`
	RootPath   string `json:"root_path"`
	PHPVersion string `json:"php_version,omitempty"`
	NodePort   int    `json:"node_port,omitempty"`
	SSL        bool   `json:"ssl"`
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

	res, err := h.store.DB().Exec(`INSERT INTO sites
		(domain, type, root_path, php_version, node_port, ssl, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')`,
		body.Domain, body.Type, body.RootPath,
		nullStr(body.PHPVersion), nullInt(body.NodePort), body.SSL)
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
		COALESCE(node_port,0), ssl, status, created_at FROM sites WHERE id = ?`, id)
	var s Site
	if err := row.Scan(&s.ID, &s.Domain, &s.Type, &s.RootPath,
		&s.PHPVersion, &s.NodePort, &s.SSL, &s.Status, &s.CreatedAt); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
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
