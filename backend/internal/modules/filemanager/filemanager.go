// File Manager module — direct implementation tanpa FileBrowser bridge.
// Semua endpoint butuh path absolut; tidak ada virtual root.
// Admin trust: panel diasumsikan dijalankan oleh admin yang sudah punya akses
// shell penuh, jadi tidak ada path whitelist.
package filemanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func Register(r fiber.Router) {
	r.Get("/list", handleList)
	r.Get("/read", handleRead)
	r.Post("/write", handleWrite)
	r.Post("/upload", handleUpload)
	r.Get("/download", handleDownload)
	r.Post("/mkdir", handleMkdir)
	r.Post("/rename", handleRename)
	r.Post("/chmod", handleChmod)
	r.Post("/extract", handleExtract)
	r.Delete("/", handleDelete)
}

type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mtime"`
}

func handleList(c *fiber.Ctx) error {
	path := cleanPath(c.Query("path", "/"))
	entries, err := os.ReadDir(path)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Entry{
			Name:    e.Name(),
			Path:    filepath.Join(path, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			Mode:    fmt.Sprintf("%#o", info.Mode().Perm()),
			ModTime: info.ModTime(),
		})
	}
	// Folders dulu, lalu alfabetis case-insensitive.
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return c.JSON(fiber.Map{
		"path":    path,
		"entries": out,
	})
}

const maxReadBytes = 2 * 1024 * 1024 // 2 MiB; UI tidak boleh open file lebih besar

func handleRead(c *fiber.Ctx) error {
	path := cleanPath(c.Query("path"))
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}
	if info.IsDir() {
		return fiber.NewError(fiber.StatusBadRequest, "is a directory")
	}
	if info.Size() > maxReadBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large to edit in browser (%d bytes); use Download", info.Size()))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.Send(data)
}

type writeReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func handleWrite(c *fiber.Ctx) error {
	var body writeReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	path := cleanPath(body.Path)
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path required")
	}
	if err := os.WriteFile(path, []byte(body.Content), 0o644); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func handleUpload(c *fiber.Ctx) error {
	dest := cleanPath(c.Query("path"))
	if dest == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path required")
	}
	if info, err := os.Stat(dest); err != nil || !info.IsDir() {
		return fiber.NewError(fiber.StatusBadRequest, "path is not a directory")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	files := form.File["files"]
	if len(files) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "no files")
	}
	saved := []string{}
	for _, fh := range files {
		// Strip path traversal (browser kadang kirim full path di filename).
		safe := filepath.Base(fh.Filename)
		target := filepath.Join(dest, safe)
		if err := c.SaveFile(fh, target); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		saved = append(saved, target)
	}
	return c.JSON(fiber.Map{"ok": true, "saved": saved})
}

func handleDownload(c *fiber.Ctx) error {
	path := cleanPath(c.Query("path"))
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}
	if info.IsDir() {
		return fiber.NewError(fiber.StatusBadRequest, "use a zip endpoint for directories")
	}
	return c.Download(path, filepath.Base(path))
}

type mkdirReq struct {
	Path string `json:"path"`
}

func handleMkdir(c *fiber.Ctx) error {
	var body mkdirReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	path := cleanPath(body.Path)
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

type renameReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func handleRename(c *fiber.Ctx) error {
	var body renameReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	from := cleanPath(body.From)
	to := cleanPath(body.To)
	if from == "" || to == "" {
		return fiber.NewError(fiber.StatusBadRequest, "from & to required")
	}
	if err := os.Rename(from, to); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

type chmodReq struct {
	Path string `json:"path"`
	Mode string `json:"mode"` // octal string e.g. "755"
}

func handleChmod(c *fiber.Ctx) error {
	var body chmodReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	path := cleanPath(body.Path)
	mode, err := strconv.ParseUint(body.Mode, 8, 32)
	if err != nil || mode > 0o7777 {
		return fiber.NewError(fiber.StatusBadRequest, "mode must be octal 0..7777")
	}
	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func handleDelete(c *fiber.Ctx) error {
	path := cleanPath(c.Query("path"))
	if path == "" || path == "/" {
		return fiber.NewError(fiber.StatusBadRequest, "refusing to delete root")
	}
	if err := os.RemoveAll(path); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

type extractReq struct {
	Path string `json:"path"` // archive file
	Dest string `json:"dest"` // optional, default = parent of Path
}

func handleExtract(c *fiber.Ctx) error {
	var body extractReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	src := cleanPath(body.Path)
	if src == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path required")
	}
	dest := cleanPath(body.Dest)
	if dest == "" {
		dest = filepath.Dir(src)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	low := strings.ToLower(src)
	switch {
	case strings.HasSuffix(low, ".zip"):
		if err := extractZip(src, dest); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	case strings.HasSuffix(low, ".tar.gz") || strings.HasSuffix(low, ".tgz"):
		if err := extractTarGz(src, dest); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	case strings.HasSuffix(low, ".tar"):
		if err := extractTar(src, dest); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	default:
		return fiber.NewError(fiber.StatusBadRequest, "unsupported archive (need .zip, .tar, .tar.gz)")
	}
	return c.JSON(fiber.Map{"ok": true, "dest": dest})
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := safeJoin(dest, f.Name)
		if target == "" {
			return fmt.Errorf("zip: unsafe path %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			_ = dst.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		_ = src.Close()
		_ = dst.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
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
	return extractTarStream(tar.NewReader(gz), dest)
}

func extractTar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarStream(tar.NewReader(f), dest)
}

func extractTarStream(tr *tar.Reader, dest string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := safeJoin(dest, hdr.Name)
		if target == "" {
			return fmt.Errorf("tar: unsafe path %s", hdr.Name)
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

// safeJoin mencegah path traversal di archive (zip-slip).
func safeJoin(base, name string) string {
	cleaned := filepath.Clean(name)
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return ""
	}
	joined := filepath.Join(base, cleaned)
	rel, err := filepath.Rel(base, joined)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return joined
}

func cleanPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}
