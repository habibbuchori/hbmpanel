package api

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/habibbuchori/hbmpanel/internal/auth"
	"github.com/habibbuchori/hbmpanel/internal/config"
	"github.com/habibbuchori/hbmpanel/internal/db"
	"github.com/habibbuchori/hbmpanel/internal/modules/audit"
	"github.com/habibbuchori/hbmpanel/internal/modules/backup"
	"github.com/habibbuchori/hbmpanel/internal/modules/caddy"
	"github.com/habibbuchori/hbmpanel/internal/modules/filemanager"
	"github.com/habibbuchori/hbmpanel/internal/modules/gitdeploy"
	"github.com/habibbuchori/hbmpanel/internal/modules/ipwhitelist"
	"github.com/habibbuchori/hbmpanel/internal/modules/laravel"
	"github.com/habibbuchori/hbmpanel/internal/modules/license"
	"github.com/habibbuchori/hbmpanel/internal/modules/logs"
	"github.com/habibbuchori/hbmpanel/internal/modules/pm2"
	"github.com/habibbuchori/hbmpanel/internal/modules/postgres"
	"github.com/habibbuchori/hbmpanel/internal/modules/redis"
	"github.com/habibbuchori/hbmpanel/internal/modules/sftp"
	"github.com/habibbuchori/hbmpanel/internal/modules/site"
	"github.com/habibbuchori/hbmpanel/internal/modules/sshkeys"
	"github.com/habibbuchori/hbmpanel/internal/modules/system"
	"github.com/habibbuchori/hbmpanel/internal/modules/terminal"
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

	// IP whitelist (kalau di-set di settings) — di-evaluasi sebelum apa-apa
	// supaya bahkan login endpoint kena saring.
	ipState := ipwhitelist.New(store)
	app.Use(ipState.Middleware)

	app.Use(csrfHeaderGuard)
	app.Use(audit.Middleware(store))

	// public
	app.Post("/api/auth/login", loginRateLimit(), auth.HandleLogin(store))
	app.Post("/api/auth/2fa/verify", loginRateLimit(), auth.HandleTOTPVerify(store))
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "version": config.Version})
	})

	// license endpoints (public, no auth)
	license.Register(app.Group("/api/license"), store)

	// protected
	api := app.Group("/api", auth.Middleware)
	api.Post("/auth/logout", auth.HandleLogout)
	api.Post("/auth/password", auth.HandlePasswordChange(store))
	api.Post("/auth/2fa/setup", auth.HandleTOTPSetup(store))
	api.Post("/auth/2fa/enable", auth.HandleTOTPEnable(store))
	api.Post("/auth/2fa/disable", auth.HandleTOTPDisable(store))
	api.Get("/me", func(c *fiber.Ctx) error {
		var totpEnabled bool
		if uid, ok := c.Locals("uid").(int64); ok {
			_, en, _ := store.GetUserTOTP(uid)
			totpEnabled = en
		}
		return c.JSON(fiber.Map{
			"uid":          c.Locals("uid"),
			"role":         c.Locals("role"),
			"totp_enabled": totpEnabled,
		})
	})

	system.Register(api.Group("/system"))
	site.Register(api.Group("/sites"), store)
	caddy.Register(api.Group("/caddy"))
	pm2.Register(api.Group("/pm2"))
	postgres.Register(api.Group("/postgres"))
	redis.Register(api.Group("/redis"))
	laravel.Register(api.Group("/laravel"), store)
	logs.Register(api.Group("/logs"), store)
	terminal.Register(api.Group("/terminal"))
	ws.Register(api.Group("/ws"), store)
	audit.Register(api.Group("/audit"), store)
	sshkeys.Register(api.Group("/sshkeys"), store)
	ipState.Register(api.Group("/settings/ip-whitelist"))

	// v0.2+ premium modules — gated by RequirePremium.
	premium := api.Group("", license.RequirePremium(store))
	sftp.Register(premium.Group("/sftp"), store)
	filemanager.Register(premium.Group("/files"))
	backup.Register(premium.Group("/backup"), store)
	gitdeploy.Register(premium.Group("/git"), store)

	// embedded frontend (static export)
	mountStatic(app)
	return app
}

// csrfHeaderGuard menolak request mutasi yang tidak menyertakan X-Requested-With.
// Browser tidak bisa men-set custom header lintas-origin tanpa CORS preflight,
// dan panel tidak whitelist origin manapun → cross-site request mati di sini.
// WebSocket (GET upgrade) dan request safe (GET/HEAD/OPTIONS) tidak diperiksa.
func csrfHeaderGuard(c *fiber.Ctx) error {
	method := c.Method()
	if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
		return c.Next()
	}
	// Hanya endpoint /api/* yang dijaga; static SPA bebas.
	if !strings.HasPrefix(c.Path(), "/api/") {
		return c.Next()
	}
	if c.Get("X-Requested-With") != "hbmpanel" {
		return fiber.NewError(fiber.StatusForbidden, "csrf: missing X-Requested-With header")
	}
	return c.Next()
}

// loginRateLimit membatasi brute-force di endpoint login.
func loginRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many login attempts; try again later")
		},
	})
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
