// Package security implements Zero Trust institutional security for the trading engine.
// Every request is authenticated, every action is authorized, every operation is audited.
package security

import (
	"context"
	"net/http"
)

// Role defines the access level of a principal.
type Role string

const (
	RoleSuperAdmin   Role = "SUPER_ADMIN"
	RoleAdmin        Role = "ADMIN"
	RoleTrader       Role = "TRADER"
	RoleRiskManager  Role = "RISK_MANAGER"
	RoleAnalyst      Role = "ANALYST"
	RoleViewer       Role = "VIEWER"
	RoleService      Role = "SERVICE" // internal service identity
)

// Principal represents an authenticated identity on a request.
type Principal struct {
	UserID      string
	Email       string
	Role        Role
	Permissions []Permission
	SessionID   string
	ServiceName string // set for service-to-service calls
	IsService   bool
	IP          string
}

// Permission is a fine-grained capability string.
type Permission string

const (
	PermTradeRequest    Permission = "trade.request" // human: submit execution intent only
	PermTradeExecute    Permission = "trade.execute" // service/engine: broker submission
	PermTradeCancel     Permission = "trade.cancel"
	PermTradeClose      Permission = "trade.close"
	PermTradeView       Permission = "trade.view"
	PermRiskOverride    Permission = "risk.override"
	PermRiskView        Permission = "risk.view"
	PermStrategyEnable  Permission = "strategy.enable"
	PermStrategyDisable Permission = "strategy.disable"
	PermStrategyView    Permission = "strategy.view"
	PermReconRun        Permission = "reconciliation.run"
	PermReconView       Permission = "reconciliation.view"
	PermSystemShutdown  Permission = "system.shutdown"
	PermSystemReset     Permission = "system.reset"
	PermUserManage      Permission = "user.manage"
	PermAuditView       Permission = "audit.view"
	PermMetricsView     Permission = "metrics.view"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey int

const (
	principalKey contextKey = iota
)

// WithPrincipal stores the authenticated Principal in the request context.
func WithPrincipal(r *http.Request, p Principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey, p))
}

// PrincipalFrom extracts the Principal from the request context.
// Returns zero Principal and false if not present.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// HasPermission returns true if the principal holds the requested permission.
func (p Principal) HasPermission(perm Permission) bool {
	for _, pp := range p.Permissions {
		if pp == perm {
			return true
		}
	}
	return false
}

// IsAdmin returns true for admin-level and above roles.
func (p Principal) IsAdmin() bool {
	return p.Role == RoleSuperAdmin || p.Role == RoleAdmin
}
