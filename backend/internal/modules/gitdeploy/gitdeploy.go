// Git deployment: clone, pull, branch switch, optional post-deploy command.
// Konfigurasi disimpan di tabel git_deployments (1 row per site).
package gitdeploy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/sites/:id", h.status)
	r.Post("/sites/:id/setup", h.setup)
	r.Post("/sites/:id/deploy", h.deploy)
	r.Delete("/sites/:id", h.unlink)
}

type handler struct{ store *db.Store }

type Deployment struct {
	SiteID         int64     `json:"site_id"`
	Repo           string    `json:"repo"`
	Branch         string    `json:"branch"`
	DeployCmd      string    `json:"deploy_cmd,omitempty"`
	LastCommit     string    `json:"last_commit,omitempty"`
	LastDeployedAt *time.Time `json:"last_deployed_at,omitempty"`
	LastOutput     string    `json:"last_output,omitempty"`
	RootPath       string    `json:"root_path,omitempty"`
}

func (h *handler) status(c *fiber.Ctx) error {
	id := c.Params("id")
	var rootPath string
	if err := h.store.DB().QueryRow(`SELECT root_path FROM sites WHERE id=?`, id).Scan(&rootPath); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "site not found")
	}

	row := h.store.DB().QueryRow(`SELECT site_id, repo, branch, COALESCE(deploy_cmd,''),
		COALESCE(last_commit,''), last_deployed_at, COALESCE(last_output,'')
		FROM git_deployments WHERE site_id=?`, id)
	var d Deployment
	var lastDeployed sql.NullTime
	err := row.Scan(&d.SiteID, &d.Repo, &d.Branch, &d.DeployCmd,
		&d.LastCommit, &lastDeployed, &d.LastOutput)
	if errors.Is(err, sql.ErrNoRows) {
		return c.JSON(fiber.Map{"linked": false, "root_path": rootPath})
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if lastDeployed.Valid {
		d.LastDeployedAt = &lastDeployed.Time
	}
	d.RootPath = rootPath
	return c.JSON(fiber.Map{"linked": true, "deployment": d})
}

type setupReq struct {
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	DeployCmd string `json:"deploy_cmd"`
}

func (h *handler) setup(c *fiber.Ctx) error {
	id := c.Params("id")
	var rootPath string
	if err := h.store.DB().QueryRow(`SELECT root_path FROM sites WHERE id=?`, id).Scan(&rootPath); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "site not found")
	}
	if rootPath == "" {
		return fiber.NewError(fiber.StatusBadRequest, "site has no root_path")
	}
	var body setupReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if !isSafeRepo(body.Repo) {
		return fiber.NewError(fiber.StatusBadRequest, "repo URL invalid")
	}
	if body.Branch == "" {
		body.Branch = "main"
	}

	// Bersihkan root_path agar git clone berhasil. Hati-hati: ini menghapus
	// file existing — UI harus konfirmasi.
	if err := emptyDir(rootPath); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "clean target: "+err.Error())
	}

	out, code := runIn(rootPath, 5*time.Minute, "git", "clone",
		"--branch", body.Branch, "--single-branch", body.Repo, ".")
	if code != 0 {
		return fiber.NewError(fiber.StatusInternalServerError, "git clone failed:\n"+out)
	}
	commit, _ := gitHEAD(rootPath)
	now := time.Now().UTC()

	_, err := h.store.DB().Exec(`
		INSERT INTO git_deployments (site_id, repo, branch, deploy_cmd, last_commit, last_deployed_at, last_output, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id) DO UPDATE SET
			repo=excluded.repo,
			branch=excluded.branch,
			deploy_cmd=excluded.deploy_cmd,
			last_commit=excluded.last_commit,
			last_deployed_at=excluded.last_deployed_at,
			last_output=excluded.last_output,
			updated_at=excluded.updated_at
	`, id, body.Repo, body.Branch, body.DeployCmd, commit, now, out, now)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "commit": commit, "output": out})
}

func (h *handler) deploy(c *fiber.Ctx) error {
	id := c.Params("id")
	var rootPath, repo, branch, deployCmd string
	row := h.store.DB().QueryRow(`
		SELECT s.root_path, g.repo, g.branch, COALESCE(g.deploy_cmd,'')
		FROM sites s
		INNER JOIN git_deployments g ON g.site_id = s.id
		WHERE s.id = ?`, id)
	if err := row.Scan(&rootPath, &repo, &branch, &deployCmd); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "site not linked to a git repo")
	}
	_ = repo // reserved for future remote-url updates

	var sb strings.Builder
	step := func(args ...string) bool {
		fmt.Fprintf(&sb, "$ %s\n", strings.Join(args, " "))
		out, code := runIn(rootPath, 5*time.Minute, args[0], args[1:]...)
		sb.WriteString(out)
		if !strings.HasSuffix(out, "\n") {
			sb.WriteString("\n")
		}
		return code == 0
	}

	commit := ""
	if step("git", "fetch", "--all", "--prune") &&
		step("git", "reset", "--hard", "origin/"+branch) {
		if deployCmd != "" {
			fmt.Fprintf(&sb, "$ %s\n", deployCmd)
			out, _ := runIn(rootPath, 10*time.Minute, "sh", "-c", deployCmd)
			sb.WriteString(out)
			if !strings.HasSuffix(out, "\n") {
				sb.WriteString("\n")
			}
		}
		commit, _ = gitHEAD(rootPath)
	}

	output := sb.String()
	if err := persistOutput(h.store, id, output, commit); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"output": output, "last_commit": commit})
}

func (h *handler) unlink(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, err := h.store.DB().Exec(`DELETE FROM git_deployments WHERE site_id=?`, id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- helpers ----------

func persistOutput(store *db.Store, siteID, output, commit string) error {
	now := time.Now().UTC()
	if commit != "" {
		_, err := store.DB().Exec(
			`UPDATE git_deployments SET last_output=?, last_deployed_at=?, last_commit=?, updated_at=? WHERE site_id=?`,
			output, now, commit, now, siteID,
		)
		return err
	}
	_, err := store.DB().Exec(
		`UPDATE git_deployments SET last_output=?, last_deployed_at=?, updated_at=? WHERE site_id=?`,
		output, now, now, siteID,
	)
	return err
}

func runIn(dir string, timeout time.Duration, name string, args ...string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new -o BatchMode=yes",
	)
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

func gitHEAD(dir string) (string, error) {
	out, code := runIn(dir, 10*time.Second, "git", "rev-parse", "HEAD")
	if code != 0 {
		return "", fmt.Errorf("rev-parse: %s", out)
	}
	return strings.TrimSpace(out), nil
}

// emptyDir removes everything inside dir (but keeps dir itself).
func emptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0o755)
		}
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(fmtJoin(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func fmtJoin(a, b string) string {
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

func isSafeRepo(s string) bool {
	if s == "" || len(s) > 1024 {
		return false
	}
	// Allow https://, http://, git@host:path, ssh://, git://
	prefixes := []string{"https://", "http://", "ssh://", "git://", "git@"}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
