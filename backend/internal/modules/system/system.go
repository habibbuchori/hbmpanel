package system

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func Register(r fiber.Router) {
	r.Get("/stats", handleStats)
	r.Get("/services", handleServices)
	r.Post("/services/:name/:action", handleServiceAction)
}

type stats struct {
	CPUCount  int       `json:"cpu_count"`
	LoadAvg   string    `json:"load_avg"`
	MemTotal  uint64    `json:"mem_total"`
	MemFree   uint64    `json:"mem_free"`
	DiskTotal uint64    `json:"disk_total"`
	DiskFree  uint64    `json:"disk_free"`
	SiteCount int       `json:"site_count"`
	PM2Count  int       `json:"pm2_count"`
	RedisUsed string    `json:"redis_used"`
	Postgres  string    `json:"postgres_status"`
	Uptime    string    `json:"uptime"`
	GoVersion string    `json:"go_version"`
	Hostname  string    `json:"hostname"`
	Now       time.Time `json:"now"`
}

func handleStats(c *fiber.Ctx) error {
	s := stats{
		CPUCount:  runtime.NumCPU(),
		GoVersion: runtime.Version(),
		Now:       time.Now().UTC(),
	}
	if h, err := exec.Command("hostname").Output(); err == nil {
		s.Hostname = strings.TrimSpace(string(h))
	}
	if out, err := exec.Command("uptime", "-p").Output(); err == nil {
		s.Uptime = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("cat", "/proc/loadavg").Output(); err == nil {
		s.LoadAvg = strings.TrimSpace(string(out))
	}
	total, free := readMem()
	s.MemTotal = total
	s.MemFree = free
	dt, df := readDisk("/")
	s.DiskTotal = dt
	s.DiskFree = df
	s.SiteCount = countLines(exec.Command("find", "/etc/caddy/Caddyfile.d", "-maxdepth", "1", "-name", "*.caddy", "-type", "f"))
	s.PM2Count = pm2Count()
	s.RedisUsed = redisInfoValue("used_memory_human")
	s.Postgres = sysctlProp("postgresql", "ActiveState")
	return c.JSON(s)
}

func countLines(cmd *exec.Cmd) int {
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n
}

func pm2Count() int {
	out, err := exec.Command("pm2", "jlist").Output()
	if err != nil {
		return 0
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return 0
	}
	return len(raw)
}

func redisInfoValue(key string) string {
	out, err := exec.Command("redis-cli", "INFO", "memory").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}


var knownServices = []string{
	"caddy", "php8.4-fpm", "postgresql", "redis-server",
	"supervisor", "ssh", "hbmpanel",
}

type svc struct {
	Name   string `json:"name"`
	Active string `json:"active"`
	Sub    string `json:"sub"`
}

func handleServices(c *fiber.Ctx) error {
	out := make([]svc, 0, len(knownServices))
	for _, n := range knownServices {
		out = append(out, svc{
			Name:   n,
			Active: sysctlProp(n, "ActiveState"),
			Sub:    sysctlProp(n, "SubState"),
		})
	}
	return c.JSON(out)
}

func sysctlProp(unit, prop string) string {
	o, err := exec.Command("systemctl", "show", unit, "-p", prop, "--value").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(o))
}

func handleServiceAction(c *fiber.Ctx) error {
	name := c.Params("name")
	action := c.Params("action")
	allowed := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
	if !allowed[action] {
		return fiber.NewError(fiber.StatusBadRequest, "invalid action")
	}
	ok := false
	for _, n := range knownServices {
		if n == name {
			ok = true
			break
		}
	}
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "unknown service")
	}
	if err := exec.Command("systemctl", action, name).Run(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
