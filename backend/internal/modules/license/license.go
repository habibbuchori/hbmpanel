package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/habibbuchori/hbmpanel/internal/db"
)

const defaultLicenseAPI = "https://license.hbm.my.id/validate"

var httpClient = &http.Client{Timeout: 10 * time.Second}

type handler struct {
	store     *db.Store
	machineID string
}

type validateRequest struct {
	Token     string `json:"token"`
	MachineID string `json:"machine_id"`
	Version   string `json:"version"`
}

type validateResponse struct {
	Valid     bool     `json:"valid"`
	Plan      string   `json:"plan"`
	Features  []string `json:"features"`
	ExpiresAt string   `json:"expires_at"`
	Message   string   `json:"message"`
}

type statusResponse struct {
	Status     string   `json:"status"`
	Plan       string   `json:"plan"`
	MachineID  string   `json:"machine_id"`
	Features   []string `json:"features"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
	GraceUntil *string  `json:"grace_until,omitempty"`
}

type activateRequest struct {
	Token string `json:"token"`
}

func Register(r fiber.Router, store *db.Store) {
	h := &handler{
		store:     store,
		machineID: getMachineID(),
	}
	r.Get("/", h.status)
	r.Post("/activate", h.activate)
	r.Post("/refresh", h.refresh)
}

func getMachineID() string {
	// Priority 1: /etc/machine-id
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		id := strings.TrimSpace(string(data))
		if len(id) >= 16 {
			return id
		}
	}
	// Priority 2: hostname
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "unknown"
}

func callRemoteValidation(token, machineID, version string) (*validateResponse, error) {
	apiURL := os.Getenv("HBMPANEL_LICENSE_API")
	if apiURL == "" {
		apiURL = defaultLicenseAPI
	}

	req := validateRequest{
		Token:     token,
		MachineID: machineID,
		Version:   version,
	}
	reqBody, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", apiURL, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api call failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var vResp validateResponse
	if err := json.Unmarshal(body, &vResp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	return &vResp, nil
}

func (h *handler) status(c *fiber.Ctx) error {
	lic, _ := h.store.GetLicense()
	resp := statusResponse{
		Status:    "inactive",
		Plan:      "free",
		MachineID: h.machineID,
		Features:  []string{},
	}

	if lic != nil {
		resp.Status = lic.Status
		resp.Plan = lic.Plan
		resp.Features = lic.Features
		if lic.ExpiresAt != nil {
			expStr := lic.ExpiresAt.Format(time.RFC3339)
			resp.ExpiresAt = &expStr
		}
		// Trigger background refresh jika > 24 jam
		if lic.LastChecked == nil || time.Since(*lic.LastChecked) > 24*time.Hour {
			go func(token string) {
				h.refreshBackground(token)
			}(lic.Token)
		}
		// Cek grace period
		if lic.LastOk != nil && time.Since(*lic.LastOk) < 7*24*time.Hour {
			graceEnd := lic.LastOk.Add(7 * 24 * time.Hour)
			graceStr := graceEnd.Format(time.RFC3339)
			resp.GraceUntil = &graceStr
		}
	}

	return c.JSON(resp)
}

func (h *handler) activate(c *fiber.Ctx) error {
	var req activateRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(400, "invalid body")
	}
	if req.Token == "" {
		return fiber.NewError(400, "token required")
	}

	vResp, err := callRemoteValidation(req.Token, h.machineID, "")
	if err != nil {
		return fiber.NewError(503, "validation server unreachable")
	}

	if !vResp.Valid {
		return fiber.NewError(402, vResp.Message)
	}

	now := time.Now().UTC()
	var expiresAt *time.Time
	if vResp.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, vResp.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}

	lic := &db.License{
		Token:       req.Token,
		MachineID:   h.machineID,
		Plan:        vResp.Plan,
		Status:      "active",
		Features:    vResp.Features,
		ActivatedAt: &now,
		ExpiresAt:   expiresAt,
		LastChecked: &now,
		LastOk:      &now,
		CreatedAt:   now,
	}

	if err := h.store.UpsertLicense(lic); err != nil {
		return fiber.NewError(500, "failed to save license")
	}

	return c.JSON(fiber.Map{
		"status":    "active",
		"plan":      vResp.Plan,
		"features":  vResp.Features,
		"machine_id": h.machineID,
	})
}

func (h *handler) refresh(c *fiber.Ctx) error {
	lic, _ := h.store.GetLicense()
	if lic == nil {
		return fiber.NewError(404, "no license found")
	}

	vResp, err := callRemoteValidation(lic.Token, h.machineID, "")
	if err != nil {
		// Network error — check grace period
		if lic.LastOk != nil && time.Since(*lic.LastOk) < 7*24*time.Hour {
			graceEnd := lic.LastOk.Add(7 * 24 * time.Hour)
			graceStr := graceEnd.Format(time.RFC3339)
			h.store.TouchLicenseChecked(lic.Token)
			return c.Status(200).JSON(fiber.Map{
				"status":      lic.Status,
				"grace_until": graceStr,
				"message":     "using cached license (offline grace period)",
			})
		}
		return fiber.NewError(503, "validation server unreachable and grace period expired")
	}

	if !vResp.Valid {
		return fiber.NewError(402, vResp.Message)
	}

	var expiresAt *time.Time
	if vResp.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, vResp.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}

	if err := h.store.UpdateLicenseFromRemote(lic.Token, vResp.Plan, vResp.Features, expiresAt, true); err != nil {
		return fiber.NewError(500, "failed to update license")
	}

	return c.JSON(fiber.Map{
		"status":    "active",
		"plan":      vResp.Plan,
		"features":  vResp.Features,
		"message":   "license validated successfully",
	})
}

func (h *handler) refreshBackground(token string) {
	_, _ = callRemoteValidation(token, h.machineID, "")
	_ = h.store.TouchLicenseChecked(token)
}
