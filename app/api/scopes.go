package api

import (
	"slices"
	"strings"
)

// Scopes an API key can hold.
const (
	ScopeCatalogRead = "catalog:read"
	ScopeCartRead    = "cart:read"
	ScopeCartWrite   = "cart:write"
	ScopeOrdersRead  = "orders:read"
	ScopeShopRead    = "shop:read"
	ScopeShopWrite   = "shop:write"
)

// AllScopes lists every scope in display order.
func AllScopes() []string {
	return []string{ScopeCatalogRead, ScopeCartRead, ScopeCartWrite, ScopeOrdersRead, ScopeShopRead, ScopeShopWrite}
}

// ValidScope reports whether s is a known scope.
func ValidScope(s string) bool { return slices.Contains(AllScopes(), s) }

// parseScopes splits the stored space-separated list, dropping unknown
// entries so a renamed scope can never grant more than intended.
func parseScopes(s string) []string {
	var out []string

	for f := range strings.FieldsSeq(s) {
		if ValidScope(f) && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}

	return out
}

// normalizeScopes sorts scopes in AllScopes order without duplicates.
func normalizeScopes(in []string) []string {
	var out []string

	for _, s := range AllScopes() {
		if slices.Contains(in, s) {
			out = append(out, s)
		}
	}

	return out
}
