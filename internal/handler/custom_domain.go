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

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type CustomDomainHandler struct {
	domains repository.CustomDomainRepository
	gates   repository.GateRepository
}

func NewCustomDomainHandler(
	domains repository.CustomDomainRepository,
	gates repository.GateRepository,
) *CustomDomainHandler {
	return &CustomDomainHandler{domains: domains, gates: gates}
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

// --- POST /api/gates/{gate_id}/domains ---

type addDomainInput struct {
	GateID uuid.UUID `path:"gate_id"`
	Body   struct {
		Domain string `json:"domain" minLength:"3" maxLength:"253"`
	}
}

type addDomainOutput struct {
	Body customDomainView
}

func (h *CustomDomainHandler) addDomain(ctx context.Context, in *addDomainInput) (*addDomainOutput, error) {
	domain := strings.ToLower(strings.TrimSpace(in.Body.Domain))
	if domain == "" {
		return nil, huma.Error400BadRequest("domain is required")
	}

	d, err := h.domains.Create(ctx, in.GateID, domain)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, huma.Error409Conflict("domain already registered")
		}
		slog.Error("create custom domain", "error", err)
		return nil, huma.Error500InternalServerError("internal error", err)
	}
	return &addDomainOutput{Body: toDomainView(d)}, nil
}

// --- GET /api/gates/{gate_id}/domains ---

type listDomainsInput struct {
	GateID uuid.UUID `path:"gate_id"`
	PaginationQuery
}

type listDomainsOutput struct {
	Body PaginatedBody[customDomainView]
}

func (h *CustomDomainHandler) listDomains(ctx context.Context, in *listDomainsInput) (*listDomainsOutput, error) {
	p := in.Params()
	list, total, err := h.domains.ListByGate(ctx, in.GateID, p)
	if err != nil {
		slog.Error("list custom domains", "error", err)
		return nil, huma.Error500InternalServerError("internal error", err)
	}

	views := make([]customDomainView, 0, len(list))
	for _, d := range list {
		views = append(views, toDomainView(d))
	}
	return &listDomainsOutput{Body: NewPaginatedBody(views, total, p)}, nil
}

// --- DELETE /api/gates/{gate_id}/domains/{domain_id} ---

type deleteDomainInput struct {
	GateID   uuid.UUID `path:"gate_id"`
	DomainID uuid.UUID `path:"domain_id"`
}

func (h *CustomDomainHandler) deleteDomain(ctx context.Context, in *deleteDomainInput) (*struct{}, error) {
	if err := h.domains.Delete(ctx, in.DomainID, in.GateID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("domain not found")
		}
		slog.Error("delete custom domain", "error", err)
		return nil, huma.Error500InternalServerError("internal error", err)
	}
	return nil, nil
}

// --- POST /api/gates/{gate_id}/domains/{domain_id}/verify ---

type verifyDomainInput struct {
	GateID   uuid.UUID `path:"gate_id"`
	DomainID uuid.UUID `path:"domain_id"`
}

type verifyDomainOutput struct {
	Body struct {
		Verified bool   `json:"verified"`
		Message  string `json:"message,omitempty"`
	}
}

func (h *CustomDomainHandler) verifyDomain(ctx context.Context, in *verifyDomainInput) (*verifyDomainOutput, error) {
	d, err := h.domains.GetByID(ctx, in.DomainID, in.GateID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("domain not found")
		}
		return nil, huma.Error500InternalServerError("internal error", err)
	}

	// DNS TXT lookup: _gatie.<domain> must contain the challenge token.
	txtHost := fmt.Sprintf("_gatie.%s", d.Domain)
	records, err := net.LookupTXT(txtHost)
	if err != nil {
		return &verifyDomainOutput{Body: struct {
			Verified bool   `json:"verified"`
			Message  string `json:"message,omitempty"`
		}{false, "DNS lookup failed — ensure _gatie." + d.Domain + " TXT record is set"}}, nil
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
		return nil, huma.Error500InternalServerError("internal error", err)
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

// --- GET /api/public/resolve?domain=xxx ---
// Returns gate context for a verified custom domain.
// Used by the frontend to know which gate page to render on a custom domain.

type resolveDomainInput struct {
	Domain string `query:"domain" required:"true"`
}

type resolveDomainOutput struct {
	Body struct {
		GateID         uuid.UUID         `json:"gate_id"`
		GateName       string            `json:"gate_name"`
		HasOpenAction  bool              `json:"has_open_action"`
		HasCloseAction bool              `json:"has_close_action"`
		Status         model.GateStatus  `json:"status"`
		MetaConfig     []model.MetaField `json:"meta_config,omitempty"`
		StatusMetadata map[string]any    `json:"status_metadata,omitempty"`
	}
}

func (h *CustomDomainHandler) resolveDomain(ctx context.Context, in *resolveDomainInput) (*resolveDomainOutput, error) {
	res, err := h.domains.ResolveByDomain(ctx, strings.ToLower(strings.TrimSpace(in.Domain)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("domain not found or not verified")
		}
		slog.Error("resolve domain", "error", err)
		return nil, huma.Error500InternalServerError("internal error", err)
	}
	out := &resolveDomainOutput{}
	out.Body.GateID = res.GateID
	out.Body.GateName = res.GateName
	out.Body.HasOpenAction = res.HasOpenAction
	out.Body.HasCloseAction = res.HasCloseAction
	out.Body.Status = res.Status
	out.Body.MetaConfig = res.MetaConfig
	out.Body.StatusMetadata = res.StatusMetadata
	return out, nil
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
		return nil, huma.Error500InternalServerError("internal error", err)
	}

	domains := make([]string, 0, len(list))
	for _, d := range list {
		domains = append(domains, d.Domain)
	}
	out := &publicDomainsListOutput{}
	out.Body.Domains = domains
	return out, nil
}

// --- GET /api/public/gates/:gate_id ---
// Returns gate context by gate ID (for the PIN pad on direct /unlock/:gateId URLs).

type resolveGateInput struct {
	GateID uuid.UUID `path:"gate_id"`
}

func (h *CustomDomainHandler) resolveGate(ctx context.Context, in *resolveGateInput) (*resolveDomainOutput, error) {
	res, err := h.gates.GetPublicInfo(ctx, in.GateID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("gate not found")
		}
		return nil, huma.Error500InternalServerError("internal error", err)
	}
	out := &resolveDomainOutput{}
	out.Body.GateID = res.GateID
	out.Body.GateName = res.GateName
	out.Body.HasOpenAction = res.HasOpenAction
	out.Body.HasCloseAction = res.HasCloseAction
	out.Body.Status = res.Status
	out.Body.MetaConfig = res.MetaConfig
	out.Body.StatusMetadata = res.StatusMetadata
	return out, nil
}

// --- RegisterRoutes ---

func (h *CustomDomainHandler) RegisterRoutes(
	api huma.API,
	requireAuth func(huma.Context, func(huma.Context)),
	gateManager func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID:   "add-custom-domain",
		Method:        http.MethodPost,
		Path:          "/api/gates/{gate_id}/domains",
		Summary:       "Add a custom domain to a gate",
		Tags:          []string{"Custom Domains"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{requireAuth, gateManager},
	}, h.addDomain)

	huma.Register(api, huma.Operation{
		OperationID: "list-custom-domains",
		Method:      http.MethodGet,
		Path:        "/api/gates/{gate_id}/domains",
		Summary:     "List custom domains for a gate",
		Tags:        []string{"Custom Domains"},
		Middlewares: huma.Middlewares{requireAuth, gateManager},
	}, h.listDomains)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-custom-domain",
		Method:        http.MethodDelete,
		Path:          "/api/gates/{gate_id}/domains/{domain_id}",
		Summary:       "Remove a custom domain",
		Tags:          []string{"Custom Domains"},
		DefaultStatus: http.StatusNoContent,
		Middlewares:   huma.Middlewares{requireAuth, gateManager},
	}, h.deleteDomain)

	huma.Register(api, huma.Operation{
		OperationID: "verify-custom-domain",
		Method:      http.MethodPost,
		Path:        "/api/gates/{gate_id}/domains/{domain_id}/verify",
		Summary:     "Trigger DNS verification for a custom domain",
		Tags:        []string{"Custom Domains"},
		Middlewares: huma.Middlewares{requireAuth, gateManager},
	}, h.verifyDomain)

	// Public endpoints (no auth)
	huma.Get(api, "/api/public/verify-domain", h.verifyDomainPublic)
	huma.Get(api, "/api/public/resolve", h.resolveDomain)
	huma.Get(api, "/api/public/domains/list", h.publicDomainsList)
	huma.Register(api, huma.Operation{
		OperationID: "public-resolve-gate",
		Method:      http.MethodGet,
		Path:        "/api/public/gates/{gate_id}",
		Summary:     "Get gate info by gate ID (for direct /unlock/:gateId URLs)",
		Tags:        []string{"Public"},
	}, h.resolveGate)
}
