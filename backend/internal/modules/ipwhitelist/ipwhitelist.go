// IP whitelist — kalau diset, request ke /api/* di-block kalau IP-nya tidak masuk
// daftar. Whitelist disimpan di settings table key "allowed_ips" sebagai list
// IP / CIDR yang dipisahkan koma.
package ipwhitelist

import (
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v2"

	"github.com/habibbuchori/hbmpanel/internal/db"
)

const SettingKey = "allowed_ips"

// State menyimpan parsed CIDR list dengan cache atomic supaya tidak hit DB tiap request.
type State struct {
	store *db.Store
	mu    sync.RWMutex
	nets  atomic.Pointer[[]*net.IPNet]
	ips   atomic.Pointer[[]net.IP]
	raw   atomic.Value // string
}

func New(store *db.Store) *State {
	s := &State{store: store}
	_ = s.Reload()
	return s
}

func (s *State) Reload() error {
	raw, err := s.store.GetSetting(SettingKey)
	if err != nil {
		return err
	}
	nets := []*net.IPNet{}
	ips := []net.IP{}
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.Contains(tok, "/") {
			_, n, err := net.ParseCIDR(tok)
			if err == nil {
				nets = append(nets, n)
			}
			continue
		}
		if ip := net.ParseIP(tok); ip != nil {
			ips = append(ips, ip)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nets.Store(&nets)
	s.ips.Store(&ips)
	s.raw.Store(raw)
	return nil
}

func (s *State) Current() string {
	v, _ := s.raw.Load().(string)
	return v
}

func (s *State) Set(raw string) error {
	if err := s.store.SetSetting(SettingKey, raw); err != nil {
		return err
	}
	return s.Reload()
}

// Middleware menerapkan whitelist. Kalau raw kosong, semua diizinkan.
// Hanya path /api/* diperiksa supaya static SPA tetap accessible (untuk redirect/login).
func (s *State) Middleware(c *fiber.Ctx) error {
	if !strings.HasPrefix(c.Path(), "/api/") {
		return c.Next()
	}
	netsP := s.nets.Load()
	ipsP := s.ips.Load()
	if netsP == nil || ipsP == nil {
		return c.Next()
	}
	if len(*netsP) == 0 && len(*ipsP) == 0 {
		return c.Next()
	}
	clientIP := net.ParseIP(c.IP())
	if clientIP == nil {
		return fiber.NewError(fiber.StatusForbidden, "ip-whitelist: unable to parse client IP")
	}
	for _, ip := range *ipsP {
		if ip.Equal(clientIP) {
			return c.Next()
		}
	}
	for _, n := range *netsP {
		if n.Contains(clientIP) {
			return c.Next()
		}
	}
	return fiber.NewError(fiber.StatusForbidden, "ip not in whitelist")
}

// Register memasang endpoint settings get/put.
func (s *State) Register(r fiber.Router) {
	r.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"allowed_ips": s.Current()})
	})
	r.Put("/", func(c *fiber.Ctx) error {
		var body struct {
			AllowedIPs string `json:"allowed_ips"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "bad body")
		}
		if err := s.Set(body.AllowedIPs); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(fiber.Map{"ok": true, "allowed_ips": s.Current()})
	})
}
