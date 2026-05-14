package auth

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

const (
	cookieName       = "hbm_session"
	phaseTOTPPending = "totp_pending"
)

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
	Phase  string `json:"phase,omitempty"` // "" = full session, "totp_pending" = needs TOTP
	jwt.RegisteredClaims
}

func issueToken(uid int64, role, phase string, ttl time.Duration) (string, error) {
	c := Claims{
		UserID: uid,
		Role:   role,
		Phase:  phase,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret())
}

// IssueToken kompatibilitas dengan caller existing (full session 12 jam).
func IssueToken(uid int64, role string) (string, error) {
	return issueToken(uid, role, "", 12*time.Hour)
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
// Token dengan Phase != "" (mis. totp_pending) ditolak untuk endpoint protected.
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
	if claims.Phase != "" {
		return fiber.NewError(fiber.StatusUnauthorized, "2FA required; complete /api/auth/2fa/verify first")
	}
	c.Locals("uid", claims.UserID)
	c.Locals("role", claims.Role)
	return c.Next()
}

func setSession(c *fiber.Ctx, tok string, ttl time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name:     cookieName,
		Value:    tok,
		HTTPOnly: true,
		Secure:   c.Secure(),
		SameSite: "Strict",
		Path:     "/",
		Expires:  time.Now().Add(ttl),
	})
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
		id, hash, role, totpEnabled, err := store.FindUser(body.Username)
		if err != nil {
			logFailedLogin(c, body.Username, "unknown_user")
			return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
			logFailedLogin(c, body.Username, "bad_password")
			store.WriteAudit(id, "auth.login_failed", body.Username, "bad_password ip="+c.IP())
			return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
		}

		if totpEnabled {
			tok, err := issueToken(id, role, phaseTOTPPending, 5*time.Minute)
			if err != nil {
				return err
			}
			setSession(c, tok, 5*time.Minute)
			return c.JSON(fiber.Map{"require_2fa": true})
		}

		tok, err := IssueToken(id, role)
		if err != nil {
			return err
		}
		setSession(c, tok, 12*time.Hour)
		store.WriteAudit(id, "auth.login", body.Username, "ip="+c.IP())
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

// ---------------- 2FA ----------------

// HandleTOTPVerify menyelesaikan login: client udah punya cookie phase=totp_pending,
// sekarang submit kode TOTP. Kalau valid, ganti cookie jadi full session.
func HandleTOTPVerify(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Code string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "bad body")
		}
		tok := c.Cookies(cookieName)
		claims, err := parse(tok)
		if err != nil || claims.Phase != phaseTOTPPending {
			return fiber.NewError(fiber.StatusUnauthorized, "no pending 2FA challenge")
		}
		sec, enabled, err := store.GetUserTOTP(claims.UserID)
		if err != nil || !enabled || sec == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "2FA not configured")
		}
		if !totp.Validate(body.Code, sec) {
			logFailedLogin(c, "", "bad_totp")
			store.WriteAudit(claims.UserID, "auth.2fa_failed", "", "ip="+c.IP())
			return fiber.NewError(fiber.StatusUnauthorized, "invalid code")
		}
		full, err := IssueToken(claims.UserID, claims.Role)
		if err != nil {
			return err
		}
		setSession(c, full, 12*time.Hour)
		store.WriteAudit(claims.UserID, "auth.login_2fa", "", "ip="+c.IP())
		return c.JSON(fiber.Map{"ok": true, "role": claims.Role})
	}
}

// HandleTOTPSetup membuat secret baru (belum aktif). Client dapat secret + otpauth URL
// untuk di-scan oleh authenticator. Belum disimpan sampai user verify lewat enable.
func HandleTOTPSetup(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(int64)
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "HBMPanel",
			AccountName: hostnameOr("server") + ":uid" + itoa(uid),
		})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(fiber.Map{
			"secret":      key.Secret(),
			"otpauth_url": key.URL(),
		})
	}
}

// HandleTOTPEnable verifikasi kode pertama dari authenticator + simpan secret.
func HandleTOTPEnable(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(int64)
		var body struct {
			Secret string `json:"secret"`
			Code   string `json:"code"`
		}
		if err := c.BodyParser(&body); err != nil || body.Secret == "" || body.Code == "" {
			return fiber.NewError(fiber.StatusBadRequest, "secret & code required")
		}
		if !totp.Validate(body.Code, body.Secret) {
			return fiber.NewError(fiber.StatusBadRequest, "invalid code; check the authenticator")
		}
		if err := store.SetUserTOTP(uid, body.Secret, true); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		store.WriteAudit(uid, "auth.2fa_enabled", "", "")
		return c.JSON(fiber.Map{"ok": true})
	}
}

// HandleTOTPDisable matikan 2FA — butuh konfirmasi password.
func HandleTOTPDisable(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(int64)
		var body struct {
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "bad body")
		}
		hash, err := userPasswordHash(store, uid)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "wrong password")
		}
		if err := store.SetUserTOTP(uid, "", false); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		store.WriteAudit(uid, "auth.2fa_disabled", "", "")
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ---------------- Password change ----------------

func HandlePasswordChange(store *db.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(int64)
		var body struct {
			Current string `json:"current"`
			New     string `json:"new"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "bad body")
		}
		if len(body.New) < 10 {
			return fiber.NewError(fiber.StatusBadRequest, "new password must be at least 10 chars")
		}
		hash, err := userPasswordHash(store, uid)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Current)) != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "current password wrong")
		}
		newHash, err := bcrypt.GenerateFromPassword([]byte(body.New), 12)
		if err != nil {
			return err
		}
		if err := store.UpdatePassword(uid, string(newHash)); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		store.WriteAudit(uid, "auth.password_changed", "", "")
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ---------------- helpers ----------------

func userPasswordHash(store *db.Store, uid int64) (string, error) {
	var hash string
	err := store.DB().QueryRow(`SELECT password FROM users WHERE id=?`, uid).Scan(&hash)
	return hash, err
}

func hostnameOr(def string) string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return def
	}
	return h
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// logFailedLogin emit baris log terstruktur supaya gampang di-pickup fail2ban.
// Pattern: "[hbmpanel-auth] FAILED ip=<ip> user=<x> reason=<y>"
func logFailedLogin(c *fiber.Ctx, username, reason string) {
	if username == "" {
		username = "-"
	}
	log.Printf("[hbmpanel-auth] FAILED ip=%s user=%s reason=%s", c.IP(), username, reason)
}
