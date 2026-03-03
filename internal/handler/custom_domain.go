package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type CustomDomainHandler struct {
	domains *repository.CustomDomainRepository
	gates   *repository.GateRepository
}

func NewCustomDomainHandler(
	domains *repository.CustomDomainRepository,
	gates *repository.GateRepository,
) *CustomDomainHandler {
	return &CustomDomainHandler{domains: domains, gates: gates}
}

// --- Path params ---

type domainPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

type domainItemPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	DomainID    uuid.UUID `path:"domain_id"`
}

// --- Views ---

type customDomainView struct {
	ID                uuid.UUID  `json:"id"`
	GateID            uuid.UUID  `json:"gate_id"`
	Domain            string     `json:"domain"`
	DNSChallengeToken string     `json:"dns_challenge_token"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func toDomainView(d *model.CustomDomain) customDomainView {
	return customDomainView{
		ID:                d.ID,
		GateID:            d.GateID,
		Domain:            d.Domain,
		DNSChallengeToken: d.DNSChallengeToken,
		VerifiedAt:        d.VerifiedAt,
		CreatedAt:         d.CreatedAt,
	}
}

// --- Helpers ---

// ensureGateInWorkspace checks the gate belongs to the workspace (prevents cross-workspace access).
func (h *CustomDomainHandler) ensureGateInWorkspace(ctx context.Context, gateID, wsID uuid.UUID) error {
	if _, err := h.gates.GetByID(ctx, gateID, wsID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return huma.Error404NotFound("gate not found")
		}
		return huma.Error500InternalServerError("internal error")
	}
	return nil
}

// --- POST /api/workspaces/{ws_id}/gates/{gate_id}/domains ---

type addDomainInput struct {
	domainPathParam
	Body struct {
		Domain string `json:"domain" minLength:"3" maxLength:"253"`
	}
}

type addDomainOutput struct {
	Body customDomainView
}

func (h *CustomDomainHandler) addDomain(ctx context.Context, in *addDomainInput) (*addDomainOutput, error) {
	if err := h.ensureGateInWorkspace(ctx, in.GateID, in.WorkspaceID); err != nil {
		return nil, err
	}

	domain := strings.ToLower(strings.TrimSpace(in.Body.Domain))
	if domain == "" {
		return nil, huma.Error400BadRequest("domain is required")
	}

	d, err := h.domains.Create(ctx, in.GateID, in.WorkspaceID, domain)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, huma.Error409Conflict("domain already registered")
		}
		slog.Error("create custom domain", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &addDomainOutput{Body: toDomainView(d)}, nil
}

// --- GET /api/workspaces/{ws_id}/gates/{gate_id}/domains ---

type listDomainsInput struct {
	domainPathParam
}

type listDomainsOutput struct {
	Body []customDomainView
}

func (h *CustomDomainHandler) listDomains(ctx context.Context, in *listDomainsInput) (*listDomainsOutput, error) {
	if err := h.ensureGateInWorkspace(ctx, in.GateID, in.WorkspaceID); err != nil {
		return nil, err
	}

	list, err := h.domains.ListByGate(ctx, in.GateID)
	if err != nil {
		slog.Error("list custom domains", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}

	views := make([]customDomainView, 0, len(list))
	for _, d := range list {
		views = append(views, toDomainView(d))
	}
	return &listDomainsOutput{Body: views}, nil
}

// --- DELETE /api/workspaces/{ws_id}/gates/{gate_id}/domains/{domain_id} ---

type deleteDomainInput struct {
	domainItemPathParam
}

func (h *CustomDomainHandler) deleteDomain(ctx context.Context, in *deleteDomainInput) (*struct{}, error) {
	if err := h.ensureGateInWorkspace(ctx, in.GateID, in.WorkspaceID); err != nil {
		return nil, err
	}

	if err := h.domains.Delete(ctx, in.DomainID, in.GateID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("domain not found")
		}
		slog.Error("delete custom domain", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

// --- POST /api/workspaces/{ws_id}/gates/{gate_id}/domains/{domain_id}/verify ---

type verifyDomainInput struct {
	domainItemPathParam
}

type verifyDomainOutput struct {
	Body struct {
		Verified bool   `json:"verified"`
		Message  string `json:"message,omitempty"`
	}
}

func (h *CustomDomainHandler) verifyDomain(ctx context.Context, in *verifyDomainInput) (*verifyDomainOutput, error) {
	if err := h.ensureGateInWorkspace(ctx, in.GateID, in.WorkspaceID); err != nil {
		return nil, err
	}

	d, err := h.domains.GetByID(ctx, in.DomainID, in.GateID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("domain not found")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}

	// DNS TXT lookup: _gaty.<domain> must contain the challenge token.
	txtHost := fmt.Sprintf("_gaty.%s", d.Domain)
	records, err := net.LookupTXT(txtHost)
	if err != nil {
		return &verifyDomainOutput{Body: struct {
			Verified bool   `json:"verified"`
			Message  string `json:"message,omitempty"`
		}{false, "DNS lookup failed — ensure _gaty." + d.Domain + " TXT record is set"}}, nil
	}

	found := false
	for _, r := range records {
		if r == d.DNSChallengeToken {
			found = true
			break
		}
	}
	if !found {
		return &verifyDomainOutput{Body: struct {
			Verified bool   `json:"verified"`
			Message  string `json:"message,omitempty"`
		}{false, "TXT record not found or token mismatch"}}, nil
	}

	now := time.Now().UTC()
	if err := h.domains.SetVerified(ctx, d.ID, now); err != nil {
		slog.Error("set domain verified", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &verifyDomainOutput{Body: struct {
		Verified bool   `json:"verified"`
		Message  string `json:"message,omitempty"`
	}{true, ""}}, nil
}

// --- GET /api/public/verify-domain?domain=xxx ---
// Caddy On-Demand TLS "ask" endpoint: returns 200 if domain is registered+verified, 403 otherwise.

type verifyDomainPublicInput struct {
	Domain string `query:"domain" required:"true"`
}

func (h *CustomDomainHandler) verifyDomainPublic(ctx context.Context, in *verifyDomainPublicInput) (*struct{}, error) {
	d, err := h.domains.GetByDomain(ctx, strings.ToLower(strings.TrimSpace(in.Domain)))
	if err != nil {
		// ErrNotFound or any error → deny TLS issuance
		return nil, huma.NewError(http.StatusForbidden, "domain not allowed")
	}
	if !d.IsVerified() {
		return nil, huma.NewError(http.StatusForbidden, "domain not verified")
	}
	return nil, nil
}

// --- GET /api/public/domains/list ---
// Returns all verified domains (for external proxy automation scripts).

type publicDomainsListOutput struct {
	Body struct {
		Domains []string `json:"domains"`
	}
}

func (h *CustomDomainHandler) publicDomainsList(ctx context.Context, _ *struct{}) (*publicDomainsListOutput, error) {
	list, err := h.domains.ListAllVerified(ctx)
	if err != nil {
		slog.Error("list verified domains", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}

	domains := make([]string, 0, len(list))
	for _, d := range list {
		domains = append(domains, d.Domain)
	}
	out := &publicDomainsListOutput{}
	out.Body.Domains = domains
	return out, nil
}

// --- RegisterRoutes ---

func (h *CustomDomainHandler) RegisterRoutes(api huma.API, wsAdmin func(ctx huma.Context, next func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID:   "add-custom-domain",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/gates/{gate_id}/domains",
		Summary:       "Add a custom domain to a gate",
		Tags:          []string{"Custom Domains"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.addDomain)

	huma.Register(api, huma.Operation{
		OperationID: "list-custom-domains",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/domains",
		Summary:     "List custom domains for a gate",
		Tags:        []string{"Custom Domains"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.listDomains)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-custom-domain",
		Method:        http.MethodDelete,
		Path:          "/api/workspaces/{ws_id}/gates/{gate_id}/domains/{domain_id}",
		Summary:       "Remove a custom domain",
		Tags:          []string{"Custom Domains"},
		DefaultStatus: http.StatusNoContent,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.deleteDomain)

	huma.Register(api, huma.Operation{
		OperationID: "verify-custom-domain",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/domains/{domain_id}/verify",
		Summary:     "Trigger DNS verification for a custom domain",
		Tags:        []string{"Custom Domains"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.verifyDomain)

	// Public endpoints (no auth)
	huma.Get(api, "/api/public/verify-domain", h.verifyDomainPublic)
	huma.Get(api, "/api/public/domains/list", h.publicDomainsList)
}
