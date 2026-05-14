package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type createTokenReq struct {
	Token     string   `json:"token"`
	Plan      string   `json:"plan"`
	Features  []string `json:"features"`
	ExpiresAt string   `json:"expires_at,omitempty"`
}

// RegisterAdmin mounts POST /token on the given router (must already be behind auth.Middleware).
func RegisterAdmin(r fiber.Router) {
	r.Post("/token", handleCreateToken)
}

func handleCreateToken(c *fiber.Ctx) error {
	if role, _ := c.Locals("role").(string); role != "admin" {
		return fiber.NewError(fiber.StatusForbidden, "admin role required")
	}

	adminKey := os.Getenv("HBMPANEL_LICENSE_ADMIN_KEY")
	if adminKey == "" {
		return fiber.NewError(fiber.StatusServiceUnavailable, "HBMPANEL_LICENSE_ADMIN_KEY not configured")
	}

	var req createTokenReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Token == "" || req.Plan == "" {
		return fiber.NewError(fiber.StatusBadRequest, "token and plan required")
	}

	apiURL := os.Getenv("HBMPANEL_LICENSE_API")
	if apiURL == "" {
		apiURL = defaultLicenseAPI
	}
	targetURL := strings.TrimSuffix(apiURL, "/validate") + "/admin/tokens"

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Admin-Key", adminKey)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, fmt.Sprintf("worker unreachable: %v", err))
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{"error": string(raw)})
	}

	return c.JSON(fiber.Map{"ok": true, "token": req.Token})
}
