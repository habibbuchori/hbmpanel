package ws

import (
	"bufio"
	"os/exec"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func Register(r fiber.Router) {
	r.Use("/", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	r.Get("/logs/:target", websocket.New(handleLogs))
}

// handleLogs men-stream live log dari sumber yang aman (whitelist).
func handleLogs(c *websocket.Conn) {
	target := c.Params("target")
	var cmd *exec.Cmd
	switch target {
	case "caddy":
		cmd = exec.Command("journalctl", "-u", "caddy", "-f", "-n", "100")
	case "panel":
		cmd = exec.Command("journalctl", "-u", "hbmpanel", "-f", "-n", "100")
	case "php":
		cmd = exec.Command("journalctl", "-u", "php8.4-fpm", "-f", "-n", "100")
	case "postgres":
		cmd = exec.Command("journalctl", "-u", "postgresql", "-f", "-n", "100")
	default:
		_ = c.WriteMessage(websocket.TextMessage, []byte("unknown target"))
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer cmd.Process.Kill()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if err := c.WriteMessage(websocket.TextMessage, scanner.Bytes()); err != nil {
			return
		}
	}
}
