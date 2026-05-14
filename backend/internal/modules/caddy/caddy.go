package caddy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	siteDir = "/etc/caddy/Caddyfile.d"
)

func Register(r fiber.Router) {
	r.Get("/configs", handleList)
	r.Post("/reload", handleReload)
}

func handleList(c *fiber.Ctx) error {
	entries, err := os.ReadDir(siteDir)
	if err != nil {
		return c.JSON([]string{})
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".caddy") {
			out = append(out, e.Name())
		}
	}
	return c.JSON(out)
}

func handleReload(c *fiber.Ctx) error {
	if err := exec.Command("systemctl", "reload", "caddy").Run(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- Caddyfile generator (dipakai oleh modules/site) ----------

type SiteConfig struct {
	Domain     string
	Type       string // php | static | node | proxy
	Root       string
	PHPSocket  string // override manual; jika kosong dihitung dari PHPVersion
	PHPVersion string // e.g. "8.4" → /run/php/php8.4-fpm.sock
	NodePort   int
	SSL        bool
}

var domainRe = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

func WriteSite(s SiteConfig) error {
	if !domainRe.MatchString(s.Domain) {
		return fmt.Errorf("domain invalid")
	}
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(siteDir, s.Domain+".caddy")
	body, err := renderConfig(s)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func RemoveSite(domain string) error {
	if !domainRe.MatchString(domain) {
		return fmt.Errorf("domain invalid")
	}
	for _, suffix := range []string{".caddy", ".caddy.disabled"} {
		path := filepath.Join(siteDir, domain+suffix)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func DisableSite(domain string) error {
	if !domainRe.MatchString(domain) {
		return fmt.Errorf("domain invalid")
	}
	src := filepath.Join(siteDir, domain+".caddy")
	dst := filepath.Join(siteDir, domain+".caddy.disabled")
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func EnableSite(domain string) error {
	if !domainRe.MatchString(domain) {
		return fmt.Errorf("domain invalid")
	}
	src := filepath.Join(siteDir, domain+".caddy.disabled")
	dst := filepath.Join(siteDir, domain+".caddy")
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func ReadSite(domain string) (string, error) {
	for _, suffix := range []string{".caddy", ".caddy.disabled"} {
		path := filepath.Join(siteDir, domain+suffix)
		b, err := os.ReadFile(path)
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("caddyfile not found for %s", domain)
}

func WriteRawSite(domain, content string) error {
	if !domainRe.MatchString(domain) {
		return fmt.Errorf("domain invalid")
	}
	path := filepath.Join(siteDir, domain+".caddy")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func ReloadCaddy() error {
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func SiteLogPath(domain string) string {
	return filepath.Join("/var/log/caddy", domain+".log")
}

func renderConfig(s SiteConfig) (string, error) {
	var b strings.Builder
	host := s.Domain
	if !s.SSL {
		host = "http://" + s.Domain
	}
	b.WriteString(host + " {\n")
	switch s.Type {
	case "php":
		socket := s.PHPSocket
		if socket == "" && s.PHPVersion != "" {
			socket = "/run/php/php" + s.PHPVersion + "-fpm.sock"
		}
		if socket == "" {
			socket = "/run/php/php8.4-fpm.sock"
		}
		fmt.Fprintf(&b, "    root * %s\n", s.Root)
		fmt.Fprintf(&b, "    php_fastcgi unix/%s\n", socket)
		b.WriteString("    file_server\n")
	case "static":
		fmt.Fprintf(&b, "    root * %s\n", s.Root)
		b.WriteString("    file_server\n")
	case "node", "proxy":
		if s.NodePort == 0 {
			return "", fmt.Errorf("node_port required")
		}
		fmt.Fprintf(&b, "    reverse_proxy localhost:%d\n", s.NodePort)
	default:
		return "", fmt.Errorf("unknown site type: %s", s.Type)
	}
	b.WriteString("    encode gzip zstd\n")
	fmt.Fprintf(&b, "    log {\n        output file %s\n    }\n", SiteLogPath(s.Domain))
	b.WriteString("}\n")
	return b.String(), nil
}
