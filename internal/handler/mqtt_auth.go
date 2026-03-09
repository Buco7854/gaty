package handler

import (
	"context"
	"net/http"

	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// MQTTAuthHandler handles authentication requests from the EMQX HTTP auth webhook.
// EMQX sends JSON POST requests with username/password for each MQTT CONNECT.
type MQTTAuthHandler struct {
	gates      *service.GateService
	serverUser string // username used by the gatie server (e.g. "gatie-server")
}

func NewMQTTAuthHandler(gates *service.GateService, serverUser string) *MQTTAuthHandler {
	return &MQTTAuthHandler{gates: gates, serverUser: serverUser}
}

// --- POST /api/mqtt/auth ---

type MQTTAuthInput struct {
	Username string `json:"username" required:"true"`
	Password string `json:"password,omitempty"`
}

type MQTTAuthOutput struct {
	Body struct {
		Result      string `json:"result"`
		IsSuperuser bool   `json:"is_superuser"`
	}
}

func (h *MQTTAuthHandler) Auth(ctx context.Context, input *struct{ Body MQTTAuthInput }) (*MQTTAuthOutput, error) {
	// The gatie server itself: superuser with full topic access.
	if input.Body.Username == h.serverUser {
		return &MQTTAuthOutput{Body: struct {
			Result      string `json:"result"`
			IsSuperuser bool   `json:"is_superuser"`
		}{Result: "allow", IsSuperuser: true}}, nil
	}

	// Gate device: username = gate_id (UUID), password = gate JWT token.
	if _, err := uuid.Parse(input.Body.Username); err != nil {
		return &MQTTAuthOutput{Body: struct {
			Result      string `json:"result"`
			IsSuperuser bool   `json:"is_superuser"`
		}{Result: "deny"}}, nil
	}

	if _, err := h.gates.AuthenticateToken(ctx, input.Body.Password); err != nil {
		return &MQTTAuthOutput{Body: struct {
			Result      string `json:"result"`
			IsSuperuser bool   `json:"is_superuser"`
		}{Result: "deny"}}, nil
	}

	return &MQTTAuthOutput{Body: struct {
		Result      string `json:"result"`
		IsSuperuser bool   `json:"is_superuser"`
	}{Result: "allow"}}, nil
}

func (h *MQTTAuthHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "mqtt-auth",
		Method:      http.MethodPost,
		Path:        "/api/mqtt/auth",
		Summary:     "MQTT broker authentication (called by EMQX HTTP auth webhook)",
		Tags:        []string{"MQTT Auth"},
	}, h.Auth)
}
