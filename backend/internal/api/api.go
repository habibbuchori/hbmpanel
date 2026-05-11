package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/habibbuchori/hbmpanel/internal/auth"
	"github.com/habibbuchori/hbmpanel/internal/config"
	"github.com/habibbuchori/hbmpanel/internal/db"
	"github.com/habibbuchori/hbmpanel/internal/modules/caddy"
	"github.com/habibbuchori/hbmpanel/internal/modules/pm2"
	"github.com/habibbuchori/hbmpanel/internal/modules/postgres"
	"github.com/habibbuchori/hbmpanel/internal/modules/redis"
	"github.com/habibbuchori/hbmpanel/internal/modules/site"
	"github.com/habibbuchori/hbmpanel/internal/modules/system"
	"github.com/habibbuchori/hbmpanel/internal/ws"
)

func New(cfg *config.Config, store *db.Store) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "hbmpanel",
		DisableStartupMessage: true,
		ErrorHandler:          errorHandler,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} (${latency})\n",
	}))

	// public
	app.Post("/api/auth/login", auth.HandleLogin(store))
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "version": config.Version})
	})

	// protected
	api := app.Group("/api", auth.Middleware)
	api.Post("/auth/logout", auth.HandleLogout)
	api.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"uid":  c.Locals("uid"),
			"role": c.Locals("role"),
		})
	})

	system.Register(api.Group("/system"))
	site.Register(api.Group("/sites"), store)
	caddy.Register(api.Group("/caddy"))
	pm2.Register(api.Group("/pm2"))
	postgres.Register(api.Group("/postgres"))
	redis.Register(api.Group("/redis"))
	ws.Register(api.Group("/ws"))

	// embedded frontend (static export)
	mountStatic(app)
	return app
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := err.Error()
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		msg = e.Message
	}
	return c.Status(code).JSON(fiber.Map{"error": msg})
}
