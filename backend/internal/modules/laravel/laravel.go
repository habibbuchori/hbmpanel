package laravel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/sites", h.listSites)
	r.Post("/:id/artisan", h.runArtisan)
	r.Post("/:id/preset/:name", h.runPreset)
}

type handler struct{ store *db.Store }

type laravelSite struct {
	ID         int64  `json:"id"`
	Domain     string `json:"domain"`
	RootPath   string `json:"root_path"`
	ProjectDir string `json:"project_dir"`
	Version    string `json:"version,omitempty"`
}

// listSites returns PHP sites where an `artisan` file exists either at
// root_path or at parent of root_path (PRD convention: root_path = .../public).
func (h *handler) listSites(c *fiber.Ctx) error {
	rows, err := h.store.DB().Query(`SELECT id, domain, root_path FROM sites
		WHERE type='php' AND status='active' ORDER BY id DESC`)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := []laravelSite{}
	for rows.Next() {
		var s laravelSite
		if err := rows.Scan(&s.ID, &s.Domain, &s.RootPath); err != nil {
			continue
		}
		s.ProjectDir = projectRoot(s.RootPath)
		if s.ProjectDir == "" {
			continue
		}
		out = append(out, s)
	}
	return c.JSON(out)
}

type artisanReq struct {
	Cmd string `json:"cmd"`
}

type artisanResp struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

func (h *handler) runArtisan(c *fiber.Ctx) error {
	site, err := h.loadSite(c.Params("id"))
	if err != nil {
		return err
	}
	var body artisanReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	args := splitArgs(body.Cmd)
	if len(args) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "cmd required")
	}
	out, code := execArtisan(site.ProjectDir, args, 60*time.Second)
	return c.JSON(artisanResp{Output: out, ExitCode: code})
}

var presets = map[string][][]string{
	"migrate":     {{"migrate", "--force"}},
	"migrate-fresh": {{"migrate:fresh", "--force"}},
	"seed":        {{"db:seed", "--force"}},
	"cache-clear": {{"cache:clear"}, {"config:clear"}, {"route:clear"}, {"view:clear"}},
	"optimize":    {{"optimize"}},
	"optimize-clear": {{"optimize:clear"}},
	"queue-restart": {{"queue:restart"}},
	"storage-link":  {{"storage:link"}},
	"version":       {{"--version"}},
}

func (h *handler) runPreset(c *fiber.Ctx) error {
	site, err := h.loadSite(c.Params("id"))
	if err != nil {
		return err
	}
	name := c.Params("name")
	cmds, ok := presets[name]
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "unknown preset")
	}
	var sb strings.Builder
	lastCode := 0
	for _, args := range cmds {
		fmt.Fprintf(&sb, "$ php artisan %s\n", strings.Join(args, " "))
		out, code := execArtisan(site.ProjectDir, args, 60*time.Second)
		sb.WriteString(out)
		if !strings.HasSuffix(out, "\n") {
			sb.WriteString("\n")
		}
		lastCode = code
		if code != 0 {
			break
		}
	}
	return c.JSON(artisanResp{Output: sb.String(), ExitCode: lastCode})
}

func (h *handler) loadSite(id string) (*laravelSite, error) {
	row := h.store.DB().QueryRow(`SELECT id, domain, root_path FROM sites WHERE id=? AND type='php'`, id)
	var s laravelSite
	if err := row.Scan(&s.ID, &s.Domain, &s.RootPath); err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "not a php site")
	}
	s.ProjectDir = projectRoot(s.RootPath)
	if s.ProjectDir == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "artisan not found at "+s.RootPath)
	}
	return &s, nil
}

func projectRoot(rootPath string) string {
	candidates := []string{rootPath, filepath.Dir(rootPath)}
	for _, p := range candidates {
		if p == "" || p == "/" {
			continue
		}
		if _, err := os.Stat(filepath.Join(p, "artisan")); err == nil {
			return p
		}
	}
	return ""
}

func execArtisan(dir string, args []string, timeout time.Duration) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "php", append([]string{"artisan"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return string(out), code
}

// splitArgs splits a command string on whitespace. Quotes are NOT honored —
// callers that need spaces in args should use the web terminal instead.
func splitArgs(s string) []string {
	out := []string{}
	for _, tok := range strings.Fields(s) {
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
}
