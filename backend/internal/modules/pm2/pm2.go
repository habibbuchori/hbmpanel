package pm2

import (
	"encoding/json"
	"os/exec"

	"github.com/gofiber/fiber/v2"
)

func Register(r fiber.Router) {
	r.Get("/list", handleList)
	r.Post("/start", handleStart)
	r.Post("/:name/stop", handleAction("stop"))
	r.Post("/:name/restart", handleAction("restart"))
	r.Post("/:name/delete", handleAction("delete"))
}

type App struct {
	Name   string `json:"name"`
	PMID   int    `json:"pm_id"`
	Status string `json:"status"`
	CPU    float64 `json:"cpu"`
	Memory uint64  `json:"memory"`
}

func handleList(c *fiber.Ctx) error {
	out, err := exec.Command("pm2", "jlist").Output()
	if err != nil {
		return c.JSON([]App{})
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	apps := make([]App, 0, len(raw))
	for _, r := range raw {
		a := App{Name: toStr(r["name"])}
		if id, ok := r["pm_id"].(float64); ok {
			a.PMID = int(id)
		}
		if mon, ok := r["monit"].(map[string]interface{}); ok {
			if cpu, ok := mon["cpu"].(float64); ok {
				a.CPU = cpu
			}
			if mem, ok := mon["memory"].(float64); ok {
				a.Memory = uint64(mem)
			}
		}
		if env, ok := r["pm2_env"].(map[string]interface{}); ok {
			a.Status = toStr(env["status"])
		}
		apps = append(apps, a)
	}
	return c.JSON(apps)
}

type startReq struct {
	Name      string `json:"name"`
	Script    string `json:"script"`
	Cwd       string `json:"cwd"`
	Instances int    `json:"instances"`
}

func handleStart(c *fiber.Ctx) error {
	var body startReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if body.Name == "" || body.Script == "" || body.Cwd == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name, script, cwd required")
	}
	args := []string{"start", body.Script, "--name", body.Name, "--cwd", body.Cwd}
	if body.Instances > 1 {
		args = append(args, "-i", itoa(body.Instances))
	}
	if err := exec.Command("pm2", args...).Run(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	_ = exec.Command("pm2", "save").Run()
	return c.JSON(fiber.Map{"ok": true})
}

func handleAction(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		name := c.Params("name")
		if name == "" {
			return fiber.NewError(fiber.StatusBadRequest, "name required")
		}
		if err := exec.Command("pm2", action, name).Run(); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		_ = exec.Command("pm2", "save").Run()
		return c.JSON(fiber.Map{"ok": true})
	}
}

func toStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
