package license

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/habibbuchori/hbmpanel/internal/db"
)

func RequirePremium(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		lic, _ := store.GetLicense()

		if lic == nil || lic.Status != "active" {
			// Check grace period
			if lic != nil && lic.LastOk != nil && time.Since(*lic.LastOk) < 7*24*time.Hour {
				c.Locals("license", lic)
				return c.Next()
			}
			return fiber.NewError(fiber.StatusPaymentRequired,
				"premium license required — activate at /dashboard/settings")
		}

		c.Locals("license", lic)
		return c.Next()
	}
}
