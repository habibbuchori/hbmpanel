package ws

import (
	"bufio"
	"os/exec"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
	"github.com/habibbuchori/hbmpanel/internal/modules/caddy"
)

func Register(r fiber.Router, store *db.Store) {
	r.Use("/", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	r.Get("/logs/:target", websocket.New(handleLogs))
	r.Get("/sitelog/:id", websocket.New(handleSiteLog(store)))
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
	stream(c, cmd)
}

func handleSiteLog(store *db.Store) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		id := c.Params("id")
		var domain, typ string
		err := store.DB().QueryRow(`SELECT domain, type FROM sites WHERE id = ?`, id).Scan(&domain, &typ)
		if err != nil {
			_ = c.WriteMessage(websocket.TextMessage, []byte("site not found"))
			return
		}
		var cmd *exec.Cmd
		switch typ {
		case "node", "proxy":
			cmd = exec.Command("pm2", "logs", domain, "--raw", "--lines", "100")
		default:
			cmd = exec.Command("tail", "-F", "-n", "100", caddy.SiteLogPath(domain))
		}
		stream(c, cmd)
	}
}

func stream(c *websocket.Conn, cmd *exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := c.WriteMessage(websocket.TextMessage, scanner.Bytes()); err != nil {
			return
		}
	}
}
