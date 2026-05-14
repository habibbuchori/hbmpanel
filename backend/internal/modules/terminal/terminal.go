//go:build linux

package terminal

import (
	"encoding/json"
	"os/exec"

	"github.com/creack/pty"
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
	r.Get("/shell", websocket.New(handleShell))
}

type wsMsg struct {
	Data   string `json:"data,omitempty"`
	Resize *struct {
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	} `json:"resize,omitempty"`
}

func handleShell(c *websocket.Conn) {
	cmd := exec.Command("bash", "-l")
	cmd.Env = append(cmd.Env,
		"TERM=xterm-256color",
		"LANG=C.UTF-8",
		"HOME="+envOr("HOME", "/root"),
		"PATH="+envOr("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"),
	)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		_ = c.WriteMessage(websocket.TextMessage, []byte("pty error: "+err.Error()))
		return
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_, _ = cmd.Process.Wait()
	}()

	done := make(chan struct{})

	// pty → ws
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if werr := c.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// ws → pty
	for {
		select {
		case <-done:
			return
		default:
		}
		_, raw, err := c.ReadMessage()
		if err != nil {
			return
		}
		var msg wsMsg
		if json.Unmarshal(raw, &msg) != nil {
			// Fallback: treat as raw stdin.
			if _, err := ptmx.Write(raw); err != nil {
				return
			}
			continue
		}
		if msg.Resize != nil {
			_ = pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Resize.Cols, Rows: msg.Resize.Rows})
		}
		if msg.Data != "" {
			if _, err := ptmx.Write([]byte(msg.Data)); err != nil {
				return
			}
		}
	}
}
