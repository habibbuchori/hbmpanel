// Backup module — site (tar.gz of root_path) & PostgreSQL (pg_dump).
// Lokal storage di BackupDir; row metadata di tabel `backups`.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

const BackupDir = "/var/lib/hbmpanel/backups"

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/", h.list)
	r.Post("/site/:id", h.backupSite)
	r.Post("/db/:name", h.backupDB)
	r.Get("/:id/download", h.download)
	r.Post("/:id/restore", h.restore)
	r.Delete("/:id", h.delete)
}

type handler struct{ store *db.Store }

type Backup struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Target    string    `json:"target"`
	Label     string    `json:"label"`
	FilePath  string    `json:"file_path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *handler) list(c *fiber.Ctx) error {
	rows, err := h.store.DB().Query(`SELECT id, kind, target, label, file_path, size_bytes, created_at
		FROM backups ORDER BY id DESC`)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := []Backup{}
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.Kind, &b.Target, &b.Label, &b.FilePath, &b.SizeBytes, &b.CreatedAt); err != nil {
			continue
		}
		out = append(out, b)
	}
	return c.JSON(out)
}

func (h *handler) backupSite(c *fiber.Ctx) error {
	id := c.Params("id")
	var domain, rootPath string
	err := h.store.DB().QueryRow(`SELECT domain, root_path FROM sites WHERE id=?`, id).Scan(&domain, &rootPath)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "site not found")
	}
	if rootPath == "" {
		return fiber.NewError(fiber.StatusBadRequest, "site has no root_path")
	}

	if err := os.MkdirAll(BackupDir, 0o750); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	fname := fmt.Sprintf("site-%s-%s.tar.gz", sanitize(domain), stamp)
	filePath := filepath.Join(BackupDir, fname)

	if err := tarGz(rootPath, filePath); err != nil {
		_ = os.Remove(filePath)
		return fiber.NewError(fiber.StatusInternalServerError, "tar: "+err.Error())
	}
	size := fileSize(filePath)
	res, err := h.store.DB().Exec(
		`INSERT INTO backups (kind, target, label, file_path, size_bytes) VALUES ('site', ?, ?, ?, ?)`,
		id, "site · "+domain, filePath, size,
	)
	if err != nil {
		_ = os.Remove(filePath)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	bid, _ := res.LastInsertId()
	return c.JSON(fiber.Map{"id": bid, "file_path": filePath, "size_bytes": size})
}

func (h *handler) backupDB(c *fiber.Ctx) error {
	name := c.Params("name")
	if !isSafeDBName(name) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid db name")
	}
	if err := os.MkdirAll(BackupDir, 0o750); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	fname := fmt.Sprintf("db-%s-%s.sql.gz", sanitize(name), stamp)
	filePath := filepath.Join(BackupDir, fname)

	// pg_dump <name> | gzip > file
	if err := pgDump(name, filePath); err != nil {
		_ = os.Remove(filePath)
		return fiber.NewError(fiber.StatusInternalServerError, "pg_dump: "+err.Error())
	}
	size := fileSize(filePath)
	res, err := h.store.DB().Exec(
		`INSERT INTO backups (kind, target, label, file_path, size_bytes) VALUES ('db', ?, ?, ?, ?)`,
		name, "db · "+name, filePath, size,
	)
	if err != nil {
		_ = os.Remove(filePath)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	bid, _ := res.LastInsertId()
	return c.JSON(fiber.Map{"id": bid, "file_path": filePath, "size_bytes": size})
}

func (h *handler) download(c *fiber.Ctx) error {
	id := c.Params("id")
	var filePath string
	if err := h.store.DB().QueryRow(`SELECT file_path FROM backups WHERE id=?`, id).Scan(&filePath); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	return c.Download(filePath, filepath.Base(filePath))
}

func (h *handler) restore(c *fiber.Ctx) error {
	id := c.Params("id")
	var kind, target, filePath string
	err := h.store.DB().QueryRow(`SELECT kind, target, file_path FROM backups WHERE id=?`, id).
		Scan(&kind, &target, &filePath)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	switch kind {
	case "site":
		var rootPath string
		if err := h.store.DB().QueryRow(`SELECT root_path FROM sites WHERE id=?`, target).Scan(&rootPath); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "original site not found; cannot restore")
		}
		if err := untarGz(filePath, rootPath); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "untar: "+err.Error())
		}
	case "db":
		if err := pgRestore(target, filePath); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "psql: "+err.Error())
		}
	default:
		return fiber.NewError(fiber.StatusBadRequest, "unknown backup kind")
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) delete(c *fiber.Ctx) error {
	id := c.Params("id")
	var filePath string
	if err := h.store.DB().QueryRow(`SELECT file_path FROM backups WHERE id=?`, id).Scan(&filePath); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	_ = os.Remove(filePath)
	if _, err := h.store.DB().Exec(`DELETE FROM backups WHERE id=?`, id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- helpers ----------

func tarGz(src, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	srcAbs, _ := filepath.Abs(src)
	return filepath.Walk(srcAbs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcAbs, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func untarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Restore at the original root_path. Header.Name is relative to src root.
		target := filepath.Join(dst, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) && target != filepath.Clean(dst) {
			return fmt.Errorf("unsafe path: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			_ = out.Close()
		}
	}
}

func pgDump(name, filePath string) error {
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-u", "postgres", "pg_dump", "--clean", "--if-exists", name)
	cmd.Stdout = gz
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	errOut, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: %s", err, string(errOut))
	}
	return nil
}

func pgRestore(name, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-u", "postgres", "psql", "-d", name)
	cmd.Stdin = gz
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func fileSize(p string) int64 {
	info, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return info.Size()
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

func isSafeDBName(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		if i == 0 && !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
			return false
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}
