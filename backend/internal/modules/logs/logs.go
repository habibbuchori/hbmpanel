package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
	"github.com/habibbuchori/hbmpanel/internal/modules/caddy"
)

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/sources", h.sources)
	r.Get("/tail", h.tail)
	r.Get("/search", h.search)
	r.Get("/download", h.download)
}

type handler struct{ store *db.Store }

type Source struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // "journal" | "file" | "pm2"
}

var staticJournalUnits = []struct{ key, unit string }{
	{"caddy", "caddy"},
	{"panel", "hbmpanel"},
	{"php", "php8.4-fpm"},
	{"postgres", "postgresql"},
	{"redis", "redis-server"},
}

func (h *handler) sources(c *fiber.Ctx) error {
	out := []Source{}
	for _, s := range staticJournalUnits {
		out = append(out, Source{Key: s.key, Label: s.unit, Kind: "journal"})
	}
	// Per-site file logs
	rows, err := h.store.DB().Query(`SELECT id, domain FROM sites ORDER BY id`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var domain string
			if rows.Scan(&id, &domain) == nil {
				out = append(out, Source{
					Key:   fmt.Sprintf("site:%d", id),
					Label: "site · " + domain,
					Kind:  "file",
				})
			}
		}
	}
	// PM2 apps
	for _, name := range pm2AppNames() {
		out = append(out, Source{Key: "pm2:" + name, Label: "pm2 · " + name, Kind: "pm2"})
	}
	return c.JSON(out)
}

func (h *handler) tail(c *fiber.Ctx) error {
	src := c.Query("source")
	lines := parseInt(c.Query("lines"), 200, 1, 5000)
	out, err := readLog(h.store, src, lines, 0, "")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.SendString(out)
}

func (h *handler) search(c *fiber.Ctx) error {
	src := c.Query("source")
	q := c.Query("q")
	if q == "" {
		return fiber.NewError(fiber.StatusBadRequest, "q required")
	}
	maxMatches := parseInt(c.Query("max"), 500, 1, 5000)
	scanLines := parseInt(c.Query("lines"), 10000, 100, 100000)
	out, err := readLog(h.store, src, scanLines, maxMatches, q)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.SendString(out)
}

func (h *handler) download(c *fiber.Ctx) error {
	src := c.Query("source")
	// Untuk download kita ambil snapshot besar (10k lines) supaya browser tetap responsif.
	out, err := readLog(h.store, src, 10000, 0, "")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s.log"`, sanitizeFilename(src)))
	return c.SendString(out)
}

// readLog returns at most `lines` recent lines from `source`. If `query` is
// non-empty, only matching lines are returned (up to `maxMatches`).
func readLog(store *db.Store, source string, lines, maxMatches int, query string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("source required")
	}

	// Resolve to a content stream (reader).
	r, cleanup, err := openSource(store, source, lines)
	if err != nil {
		return "", err
	}
	defer cleanup()

	var b strings.Builder
	matches := 0
	q := strings.ToLower(query)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if q != "" && !strings.Contains(strings.ToLower(line), q) {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		matches++
		if maxMatches > 0 && matches >= maxMatches {
			break
		}
	}
	return b.String(), nil
}

func openSource(store *db.Store, source string, lines int) (io.Reader, func(), error) {
	// journalctl unit
	for _, s := range staticJournalUnits {
		if s.key == source {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			cmd := exec.CommandContext(ctx, "journalctl", "-u", s.unit, "-n", strconv.Itoa(lines), "--no-pager")
			r, err := cmd.StdoutPipe()
			if err != nil {
				cancel()
				return nil, nil, err
			}
			if err := cmd.Start(); err != nil {
				cancel()
				return nil, nil, err
			}
			cleanup := func() {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				cancel()
			}
			return r, cleanup, nil
		}
	}

	// site:<id> → file
	if strings.HasPrefix(source, "site:") {
		id := strings.TrimPrefix(source, "site:")
		var domain string
		if err := store.DB().QueryRow(`SELECT domain FROM sites WHERE id=?`, id).Scan(&domain); err != nil {
			return nil, nil, fmt.Errorf("site not found")
		}
		return openFileTail(caddy.SiteLogPath(domain), lines)
	}

	// pm2:<name> → pm2 logs cmd
	if strings.HasPrefix(source, "pm2:") {
		name := strings.TrimPrefix(source, "pm2:")
		if !isSafeName(name) {
			return nil, nil, fmt.Errorf("invalid pm2 name")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, "pm2", "logs", name, "--raw", "--nostream", "--lines", strconv.Itoa(lines))
		r, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			return nil, nil, err
		}
		if err := cmd.Start(); err != nil {
			cancel()
			return nil, nil, err
		}
		cleanup := func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			cancel()
		}
		return r, cleanup, nil
	}

	return nil, nil, fmt.Errorf("unknown source: %s", source)
}

// openFileTail opens a file and seeks to roughly the last N lines.
func openFileTail(path string, lines int) (io.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	// Heuristik: rata-rata 200 byte per baris.
	want := int64(lines) * 200
	if fi.Size() > want {
		_, _ = f.Seek(fi.Size()-want, io.SeekStart)
		// buang baris pertama (kemungkinan terpotong)
		_, _ = bufio.NewReader(f).ReadString('\n')
	}
	cleanup := func() { _ = f.Close() }
	return f, cleanup, nil
}

func pm2AppNames() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pm2", "jlist").Output()
	if err != nil {
		return nil
	}
	names := []string{}
	// hindari import json di sini — pakai pencarian sederhana "name":"<x>"
	s := string(out)
	for {
		i := strings.Index(s, `"name":"`)
		if i < 0 {
			break
		}
		s = s[i+8:]
		j := strings.Index(s, `"`)
		if j < 0 {
			break
		}
		name := s[:j]
		if isSafeName(name) {
			names = append(names, name)
		}
		s = s[j+1:]
	}
	return names
}

func isSafeName(n string) bool {
	if n == "" || len(n) > 64 {
		return false
	}
	for _, r := range n {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "log"
	}
	return b.String()
}

func parseInt(s string, def, min, max int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
