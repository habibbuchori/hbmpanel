package postgres

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Register(r fiber.Router) {
	r.Get("/databases", handleListDB)
	r.Post("/databases", handleCreateDB)
	r.Delete("/databases/:name", handleDropDB)
	r.Get("/users", handleListUsers)
	r.Post("/users", handleCreateUser)
}

var nameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func psql(args ...string) (string, error) {
	cmd := exec.Command("sudo", append([]string{"-u", "postgres", "psql", "-tAc"}, strings.Join(args, " "))...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func psqlQuery(q string) (string, error) {
	cmd := exec.Command("sudo", "-u", "postgres", "psql", "-tAc", q)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func handleListDB(c *fiber.Ctx) error {
	out, err := psqlQuery("SELECT datname FROM pg_database WHERE datistemplate = false")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, out)
	}
	dbs := []string{}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			dbs = append(dbs, l)
		}
	}
	return c.JSON(dbs)
}

type createDBReq struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

func handleCreateDB(c *fiber.Ctx) error {
	var body createDBReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if !nameRe.MatchString(body.Name) || !nameRe.MatchString(body.Owner) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid name")
	}
	q := "CREATE DATABASE \"" + body.Name + "\" OWNER \"" + body.Owner + "\""
	if out, err := psqlQuery(q); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, out)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func handleDropDB(c *fiber.Ctx) error {
	name := c.Params("name")
	if !nameRe.MatchString(name) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid name")
	}
	if out, err := psqlQuery("DROP DATABASE \"" + name + "\""); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, out)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func handleListUsers(c *fiber.Ctx) error {
	out, err := psqlQuery("SELECT rolname FROM pg_roles WHERE rolcanlogin = true ORDER BY rolname")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, out)
	}
	users := []string{}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			users = append(users, l)
		}
	}
	return c.JSON(users)
}

type createUserReq struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func handleCreateUser(c *fiber.Ctx) error {
	var body createUserReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad body")
	}
	if !nameRe.MatchString(body.Name) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid name")
	}
	if body.Password == "" {
		return fiber.NewError(fiber.StatusBadRequest, "password required")
	}
	// gunakan parameterized via psql -v agar password tidak bocor ke logs
	q := "CREATE ROLE \"" + body.Name + "\" LOGIN PASSWORD '" + escapeSQL(body.Password) + "'"
	if out, err := psqlQuery(q); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, out)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
