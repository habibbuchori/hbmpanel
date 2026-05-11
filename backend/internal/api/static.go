package api

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"github.com/habibbuchori/hbmpanel/internal/web"
)

func mountStatic(app *fiber.App) {
	sub := web.FS()
	if sub == nil {
		return
	}

	app.Use("/", filesystem.New(filesystem.Config{
		Root:         http.FS(sub),
		Browse:       false,
		Index:        "index.html",
		NotFoundFile: "index.html",
	}))

	app.Use(func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Next()
		}
		return c.SendStatus(fiber.StatusNotFound)
	})
}
