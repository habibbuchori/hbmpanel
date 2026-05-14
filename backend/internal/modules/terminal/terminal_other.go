//go:build !linux

package terminal

import "github.com/gofiber/fiber/v2"

// Register stub untuk dev di non-Linux. Terminal hanya tersedia di produksi (Linux).
func Register(r fiber.Router) {
	r.Get("/shell", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotImplemented, "terminal only available on linux")
	})
}
