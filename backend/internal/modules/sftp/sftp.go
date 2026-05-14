// SFTP user management. Mengelola user OS untuk chroot-SFTP sesuai PRD 7.10.
//
// Asumsi dari installer:
//   - Group `sftponly` sudah dibuat.
//   - sshd_config sudah punya block:
//
//       Match Group sftponly
//           ChrootDirectory /home/%u
//           ForceCommand internal-sftp
//           AllowTcpForwarding no
//           X11Forwarding no
//
//   - User dibuat dengan shell /usr/sbin/nologin, home /home/<user> milik root:root
//     mode 755, dan subdir /home/<user>/data milik <user>:<user> sebagai write area.
package sftp

import (
	"database/sql"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

const (
	sftpGroup    = "sftponly"
	defaultShell = "/usr/sbin/nologin"
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)

func Register(r fiber.Router, store *db.Store) {
	h := &handler{store: store}
	r.Get("/users", h.list)
	r.Post("/users", h.create)
	r.Post("/users/:id/password", h.resetPassword)
	r.Delete("/users/:id", h.delete)
}

type handler struct{ store *db.Store }

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	SiteID    *int64    `json:"site_id,omitempty"`
	Home      string    `json:"home"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *handler) list(c *fiber.Ctx) error {
	rows, err := h.store.DB().Query(`SELECT id, username, site_id, home, created_at
		FROM sftp_users ORDER BY id DESC`)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		var sid sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &sid, &u.Home, &u.CreatedAt); err != nil {
			continue
		}
		if sid.Valid {
			u.SiteID = &sid.Int64
		}
		out = append(out, u)
	}
	return c.JSON(out)
}

type createReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	SiteID   int64  `json:"site_id,omitempty"`
}

func (h *handler) create(c *fiber.Ctx) error {
	var body createReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if !nameRe.MatchString(body.Username) {
		return fiber.NewError(fiber.StatusBadRequest, "username must be lowercase letters/digits/_/-, 2..32 chars")
	}
	if len(body.Password) < 8 {
		return fiber.NewError(fiber.StatusBadRequest, "password min 8 chars")
	}

	// Resolve target directory.
	target := ""
	if body.SiteID > 0 {
		if err := h.store.DB().QueryRow(`SELECT root_path FROM sites WHERE id=?`, body.SiteID).Scan(&target); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "site not found")
		}
	}

	home := filepath.Join("/home", body.Username)

	// 1. useradd dengan group sftponly, shell nologin.
	useradd := exec.Command("useradd",
		"-m",
		"-d", home,
		"-s", defaultShell,
		"-G", sftpGroup,
		body.Username,
	)
	if out, err := useradd.CombinedOutput(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "useradd: "+string(out))
	}

	// 2. Set password via chpasswd.
	if err := setPassword(body.Username, body.Password); err != nil {
		_ = exec.Command("userdel", "-r", body.Username).Run()
		return fiber.NewError(fiber.StatusInternalServerError, "chpasswd: "+err.Error())
	}

	// 3. Chroot prep: home harus root:root mode 755.
	_ = exec.Command("chown", "root:root", home).Run()
	_ = exec.Command("chmod", "755", home).Run()

	// 4. Subdir 'data' untuk write area.
	dataDir := filepath.Join(home, "data")
	_ = exec.Command("mkdir", "-p", dataDir).Run()
	_ = exec.Command("chown", body.Username+":"+body.Username, dataDir).Run()
	_ = exec.Command("chmod", "755", dataDir).Run()

	// 5. Bind-mount site root_path → /home/<user>/data/<site> kalau site ditentukan.
	// (Skip auto-mount; dokumentasikan saja sebagai langkah manual untuk v0.2.)
	_ = target // reserved for future bind-mount automation

	// 6. Simpan ke DB.
	var siteCol any
	if body.SiteID > 0 {
		siteCol = body.SiteID
	} else {
		siteCol = nil
	}
	res, err := h.store.DB().Exec(
		`INSERT INTO sftp_users (username, site_id, home) VALUES (?, ?, ?)`,
		body.Username, siteCol, home,
	)
	if err != nil {
		_ = exec.Command("userdel", "-r", body.Username).Run()
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	id, _ := res.LastInsertId()
	return c.JSON(fiber.Map{"id": id, "ok": true, "home": home, "data_dir": dataDir})
}

type passwordReq struct {
	Password string `json:"password"`
}

func (h *handler) resetPassword(c *fiber.Ctx) error {
	id := c.Params("id")
	var username string
	if err := h.store.DB().QueryRow(`SELECT username FROM sftp_users WHERE id=?`, id).Scan(&username); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	var body passwordReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if len(body.Password) < 8 {
		return fiber.NewError(fiber.StatusBadRequest, "password min 8 chars")
	}
	if err := setPassword(username, body.Password); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *handler) delete(c *fiber.Ctx) error {
	id := c.Params("id")
	var username string
	if err := h.store.DB().QueryRow(`SELECT username FROM sftp_users WHERE id=?`, id).Scan(&username); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	// userdel -r menghapus home directory. Idempotent kalau user sudah tiada.
	if out, err := exec.Command("userdel", "-r", username).CombinedOutput(); err != nil {
		// Code 6 = user does not exist — anggap ok.
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 6 {
			return fiber.NewError(fiber.StatusInternalServerError, "userdel: "+string(out))
		}
	}
	if _, err := h.store.DB().Exec(`DELETE FROM sftp_users WHERE id=?`, id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// setPassword mengeset password user via chpasswd stdin.
func setPassword(username, password string) error {
	cmd := exec.Command("chpasswd")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdin, "%s:%s\n", username, password); err != nil {
		_ = stdin.Close()
		return err
	}
	_ = stdin.Close()
	return cmd.Wait()
}
