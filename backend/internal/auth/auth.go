package auth

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

const cookieName = "hbm_session"

func secret() []byte {
	if v := os.Getenv("HBMPANEL_JWT_SECRET"); v != "" {
		return []byte(v)
	}
	log.Fatal("HBMPANEL_JWT_SECRET env var is required")
	return nil
}

type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func IssueToken(uid int64, role string) (string, error) {
	c := Claims{
		UserID: uid,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret())
}

func parse(tok string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tok, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("bad signing method")
		}
		return secret(), nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// Middleware validasi JWT dari cookie atau header Authorization.
func Middleware(c *fiber.Ctx) error {
	tok := c.Cookies(cookieName)
	if tok == "" {
		h := c.Get("Authorization")
		if len(h) > 7 && h[:7] == "Bearer " {
			tok = h[7:]
		}
	}
	if tok == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing token")
	}
	claims, err := parse(tok)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
	}
	c.Locals("uid", claims.UserID)
	c.Locals("role", claims.Role)
	return c.Next()
}

// HandleLogin dipanggil dari router.
func HandleLogin(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "bad body")
		}
		id, hash, role, err := store.FindUser(body.Username)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
		}
		tok, err := IssueToken(id, role)
		if err != nil {
			return err
		}
		c.Cookie(&fiber.Cookie{
			Name:     cookieName,
			Value:    tok,
			HTTPOnly: true,
			Secure:   c.Secure(), // hanya Secure bila request via HTTPS
			SameSite: "Lax",
			Path:     "/",
			Expires:  time.Now().Add(12 * time.Hour),
		})
		return c.JSON(fiber.Map{"ok": true, "role": role})
	}
}

func HandleLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:    cookieName,
		Value:   "",
		Path:    "/",
		Expires: time.Now().Add(-1 * time.Hour),
	})
	return c.JSON(fiber.Map{"ok": true})
}
