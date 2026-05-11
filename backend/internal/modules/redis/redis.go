package redis

import (
	"os/exec"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Register(r fiber.Router) {
	r.Get("/info", handleInfo)
	r.Post("/flush", handleFlush)
	r.Post("/restart", handleRestart)
}

func handleInfo(c *fiber.Ctx) error {
	out, err := exec.Command("redis-cli", "INFO").Output()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	info := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, ":"); i > 0 {
			info[line[:i]] = line[i+1:]
		}
	}
	return c.JSON(info)
}

func handleFlush(c *fiber.Ctx) error {
	if err := exec.Command("redis-cli", "FLUSHALL").Run(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func handleRestart(c *fiber.Ctx) error {
	if err := exec.Command("systemctl", "restart", "redis-server").Run(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
