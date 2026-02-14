package integration

import (
	"net/http"
	"testing"
)

func TestAuth_GenerateToken_Success(t *testing.T) {
	app := setupTestApp(t)

	w := doFormRequest(app, http.MethodPost, "/auth/token", map[string]string{
		"name": "test-user",
		"role": "enduser",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	token, ok := resp["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected non-empty token in response")
	}
}

func TestAuth_GenerateToken_MissingFields(t *testing.T) {
	app := setupTestApp(t)

	// Missing role
	w := doFormRequest(app, http.MethodPost, "/auth/token", map[string]string{
		"name": "test-user",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing role, got %d: %s", w.Code, w.Body.String())
	}

	// Missing name
	w = doFormRequest(app, http.MethodPost, "/auth/token", map[string]string{
		"role": "enduser",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d: %s", w.Code, w.Body.String())
	}

	// Missing both
	w = doFormRequest(app, http.MethodPost, "/auth/token", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing both, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_GenerateToken_AllRoles(t *testing.T) {
	app := setupTestApp(t)

	roles := []string{"enduser", "drone", "admin"}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			w := doFormRequest(app, http.MethodPost, "/auth/token", map[string]string{
				"name": "test-" + role,
				"role": role,
			})
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 for role %s, got %d: %s", role, w.Code, w.Body.String())
			}
			resp := parseJSON(t, w)
			if _, ok := resp["token"].(string); !ok {
				t.Fatalf("expected token for role %s", role)
			}
		})
	}
}

func TestAuth_ProtectedEndpoint_NoToken(t *testing.T) {
	app := setupTestApp(t)

	w := doRequest(app, http.MethodGet, "/orders", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_ProtectedEndpoint_InvalidToken(t *testing.T) {
	app := setupTestApp(t)

	w := doRequest(app, http.MethodGet, "/orders", nil, "invalid-token")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_RoleGuard_WrongRole(t *testing.T) {
	app := setupTestApp(t)

	// Drone token trying to access enduser endpoint
	token := droneToken(t, app, "drone-1")
	w := doRequest(app, http.MethodGet, "/orders", nil, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for drone→enduser, got %d: %s", w.Code, w.Body.String())
	}

	// Enduser token trying to access drone endpoint
	token = enduserToken(t, app, "user-1")
	w = doRequest(app, http.MethodGet, "/drone/jobs", nil, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for enduser→drone, got %d: %s", w.Code, w.Body.String())
	}

	// Enduser token trying to access admin endpoint
	w = doRequest(app, http.MethodGet, "/admin/orders", nil, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for enduser→admin, got %d: %s", w.Code, w.Body.String())
	}
}
