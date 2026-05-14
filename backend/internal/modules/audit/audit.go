// Audit log: middleware mencatat tiap request mutasi ke audit_log + endpoint
// list buat UI.
package audit

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

// Middleware mencatat aksi mutasi (POST/PUT/PATCH/DELETE) ke audit_log SETELAH
// handler selesai. Hanya log kalau status < 400 (sukses), kecuali login (sudah
// dicatat sendiri di auth.go).
func Middleware(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		m := c.Method()
		if m == fiber.MethodGet || m == fiber.MethodHead || m == fiber.MethodOptions {
			return c.Next()
		}
		path := c.Path()
		// Login/2FA punya audit log internal.
		if strings.HasPrefix(path, "/api/auth/") {
			return c.Next()
		}
		// Static assets/non-API: skip.
		if !strings.HasPrefix(path, "/api/") {
			return c.Next()
		}
		err := c.Next()
		status := c.Response().StatusCode()
		if status >= 400 {
			return err
		}
		uid, _ := c.Locals("uid").(int64)
		store.WriteAudit(uid, m+" "+path, "", "ip="+c.IP())
		return err
	}
}

// Register memasang endpoint GET /api/audit untuk listing.
func Register(r fiber.Router, store *db.Store) {
	r.Get("/", func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "100"))
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		rows, err := store.ListAudit(limit, offset)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(rows)
	})
}
