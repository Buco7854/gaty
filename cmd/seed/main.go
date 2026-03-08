// seed crée des données de démo dans une instance Gatie fraîche.
//
// Il provisionne :
//   - Un utilisateur admin (demo@gatie.local / Demo1234!)
//   - Un workspace "Demo"
//   - Gate 1 "Portail Principal" — HTTP_INBOUND, métadonnée batterie, règle low_battery
//   - Gate 2 "Garage" — NONE (statut manuel)
//
// Idempotent : se connecte avec les identifiants démo si le compte existe déjà.
//
// Usage :
//
//	go run ./cmd/seed [--api=http://localhost:8080]
//	# ou via task :
//	task demo-seed
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
)

const (
	demoEmail    = "demo@gatie.local"
	demoPassword = "Demo1234!"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8080", "URL de base de l'API Gatie")
	flag.Parse()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	c := &seedClient{base: *apiURL, http: client}

	// ── 1. Santé ──────────────────────────────────────────────────────────────
	health := c.must(http.MethodGet, "/api/health", nil, http.StatusOK)
	if db, _ := health["database"].(string); db != "ok" {
		slog.Error("base de données non disponible — lance `task dev-infra` et `task dev-api`")
		os.Exit(1)
	}
	slog.Info("infra ok", "db", health["database"], "redis", health["redis"])

	// ── 2. Authentification ───────────────────────────────────────────────────
	setupStatus := c.must(http.MethodGet, "/api/setup/status", nil, http.StatusOK)
	if setupStatus["setup_required"] == true {
		slog.Info("premier démarrage — création du compte admin", "email", demoEmail)
		c.must(http.MethodPost, "/api/setup/init", map[string]any{
			"email":    demoEmail,
			"password": demoPassword,
		}, http.StatusOK)
		slog.Info("compte admin créé ✓")
	} else {
		// Essaie de s'inscrire ; si le compte existe déjà on se connecte.
		resp := c.call(http.MethodPost, "/api/auth/register", map[string]any{
			"email":    demoEmail,
			"password": demoPassword,
		})
		if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusOK {
			// Compte existant ou tout juste créé → login pour obtenir les cookies.
			c.must(http.MethodPost, "/api/auth/login", map[string]any{
				"email":    demoEmail,
				"password": demoPassword,
			}, http.StatusOK)
			slog.Info("connecté avec le compte demo", "email", demoEmail)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			slog.Error("impossible de créer/connecter le compte demo", "status", resp.StatusCode, "body", string(body))
			os.Exit(1)
		}
	}

	// ── 3. Workspace ──────────────────────────────────────────────────────────
	wsResp := c.must(http.MethodPost, "/api/workspaces", map[string]any{
		"name": "Demo",
	}, http.StatusCreated)
	wsID, _ := wsResp["id"].(string)
	slog.Info("workspace créé", "id", wsID, "name", "Demo")

	// ── 4. Gate 1 : Portail Principal ─────────────────────────────────────────
	g1 := c.must(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates", wsID), map[string]any{
		"name":             "Portail Principal",
		"integration_type": "NONE",
		"status_config": map[string]any{
			"type": "HTTP_INBOUND",
			"config": map[string]any{
				"mapping": map[string]any{
					"status": map[string]any{"field": "status"},
				},
			},
		},
		"meta_config": []map[string]any{
			{"key": "battery", "label": "Batterie", "unit": "%"},
			{"key": "rssi", "label": "Signal RSSI", "unit": "dBm"},
		},
		"status_rules": []map[string]any{
			{"key": "battery", "op": "lt", "value": "20", "set_status": "low_battery"},
		},
		"custom_statuses": []string{"low_battery"},
	}, http.StatusCreated)
	token1, _ := g1["gate_token"].(string)
	gateID1, _ := g1["id"].(string)
	slog.Info("gate créée", "name", "Portail Principal", "id", gateID1)

	// ── 5. Gate 2 : Garage ────────────────────────────────────────────────────
	g2 := c.must(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates", wsID), map[string]any{
		"name":             "Garage",
		"integration_type": "NONE",
	}, http.StatusCreated)
	token2, _ := g2["gate_token"].(string)
	gateID2, _ := g2["id"].(string)
	slog.Info("gate créée", "name", "Garage", "id", gateID2)

	// ── Résumé ────────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  DONNÉES DÉMO CRÉÉES")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Email    : %s\n", demoEmail)
	fmt.Printf("  Mot de passe : %s\n", demoPassword)
	fmt.Printf("  Workspace  : Demo (%s)\n", wsID)
	fmt.Println()
	fmt.Printf("  [1] Portail Principal  id=%s\n", gateID1)
	fmt.Printf("      token : %s\n", token1)
	fmt.Println()
	fmt.Printf("  [2] Garage             id=%s\n", gateID2)
	fmt.Printf("      token : %s\n", token2)
	fmt.Println()
	fmt.Println("  Simuler le portail (HTTP_INBOUND, pousse des statuts) :")
	fmt.Printf("    go run ./cmd/gatesim --mode=http --token=%s\n", token1)
	fmt.Println()
	fmt.Println("  Ouvre l'UI : http://localhost:5173")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// ─── client minimal ───────────────────────────────────────────────────────────

type seedClient struct {
	base string
	http *http.Client
}

func (c *seedClient) call(method, path string, body any) *http.Response {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, r)
	if err != nil {
		slog.Error("build request", "err", err)
		os.Exit(1)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// Vérifie si l'API est accessible
		u, _ := url.Parse(c.base)
		slog.Error("impossible de joindre l'API — lance `task dev-api`",
			"host", u.Host, "err", err)
		os.Exit(1)
	}
	return resp
}

// must appelle call et échoue si le code HTTP ne correspond pas.
// Retourne le corps JSON décodé.
func (c *seedClient) must(method, path string, body any, wantStatus int) map[string]any {
	resp := c.call(method, path, body)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		slog.Error("réponse inattendue",
			"method", method, "path", path,
			"got", resp.StatusCode, "want", wantStatus,
			"body", string(raw))
		os.Exit(1)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
