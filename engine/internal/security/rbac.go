package security

// rolePermissions maps each Role to its complete permission set.
// Follows least-privilege: each role only gets what it needs.
var rolePermissions = map[Role][]Permission{
	RoleSuperAdmin: {
		PermTradeExecute, PermTradeCancel, PermTradeClose, PermTradeView,
		PermRiskOverride, PermRiskView,
		PermStrategyEnable, PermStrategyDisable, PermStrategyView,
		PermReconRun, PermReconView,
		PermSystemShutdown, PermSystemReset,
		PermUserManage, PermAuditView, PermMetricsView,
	},
	RoleAdmin: {
		PermTradeExecute, PermTradeCancel, PermTradeClose, PermTradeView,
		PermRiskOverride, PermRiskView,
		PermStrategyEnable, PermStrategyDisable, PermStrategyView,
		PermReconRun, PermReconView,
		PermSystemShutdown, PermSystemReset,
		PermAuditView, PermMetricsView,
	},
	RoleTrader: {
		PermTradeExecute, PermTradeCancel, PermTradeClose, PermTradeView,
		PermRiskView,
		PermStrategyView,
		PermReconView,
		PermMetricsView,
	},
	RoleRiskManager: {
		PermTradeView,
		PermRiskOverride, PermRiskView,
		PermStrategyEnable, PermStrategyDisable, PermStrategyView,
		PermReconRun, PermReconView,
		PermAuditView, PermMetricsView,
	},
	RoleAnalyst: {
		PermTradeView,
		PermRiskView,
		PermStrategyView,
		PermReconView,
		PermMetricsView,
	},
	RoleViewer: {
		PermTradeView,
		PermRiskView,
		PermStrategyView,
		PermMetricsView,
	},
	RoleService: {
		PermTradeExecute, PermTradeView,
		PermRiskView,
		PermStrategyView,
		PermReconView,
		PermMetricsView,
	},
}

// PermissionsForRole returns the canonical permission set for a role.
func PermissionsForRole(role Role) []Permission {
	perms, ok := rolePermissions[role]
	if !ok {
		return nil
	}
	// Return a copy to prevent external mutation.
	cp := make([]Permission, len(perms))
	copy(cp, perms)
	return cp
}

// RoleFromString converts a string claim to a Role, defaulting to RoleViewer.
func RoleFromString(s string) Role {
	switch Role(s) {
	case RoleSuperAdmin, RoleAdmin, RoleTrader, RoleRiskManager, RoleAnalyst, RoleViewer, RoleService:
		return Role(s)
	default:
		return RoleViewer
	}
}

// endpointPermissions maps path prefixes to the minimum required permission.
// Checked in order; first match wins. Paths NOT listed here are public (health checks).
var endpointPermissions = []endpointPolicy{
	// Admin — most restrictive first.
	{prefix: "/api/admin/kill", method: "POST", perm: PermSystemShutdown},
	{prefix: "/api/admin/close-all", method: "POST", perm: PermSystemShutdown},
	{prefix: "/api/admin/reset", method: "POST", perm: PermSystemReset},
	{prefix: "/api/admin/clear-history", method: "POST", perm: PermSystemReset},
	// Risk.
	{prefix: "/api/risk", method: "", perm: PermRiskView},
	// Trade data.
	{prefix: "/api/trades", method: "GET", perm: PermTradeView},
	{prefix: "/api/positions", method: "GET", perm: PermTradeView},
	{prefix: "/api/stats", method: "GET", perm: PermTradeView},
	// Strategy data.
	{prefix: "/api/strategies", method: "GET", perm: PermStrategyView},
	// AI + signals.
	{prefix: "/api/ai", method: "", perm: PermTradeView},
	// Reconciliation.
	{prefix: "/api/reconciliation", method: "", perm: PermReconView},
	// Delta live (trade execution).
	{prefix: "/api/delta-live/order", method: "POST", perm: PermTradeExecute},
	{prefix: "/api/delta-live/enable", method: "POST", perm: PermStrategyEnable},
	{prefix: "/api/delta-live", method: "GET", perm: PermTradeView},
	// Options engines.
	{prefix: "/api/options", method: "GET", perm: PermTradeView},
	{prefix: "/api/options/reset", method: "POST", perm: PermSystemReset},
	{prefix: "/api/nifty-options", method: "GET", perm: PermTradeView},
	{prefix: "/api/nifty-options/reset", method: "POST", perm: PermSystemReset},
	{prefix: "/api/nifty-stocks", method: "GET", perm: PermTradeView},
	// Paper OMS.
	{prefix: "/paper/", method: "", perm: PermTradeView},
	// Logs — analyst+ only.
	{prefix: "/api/logs", method: "GET", perm: PermAuditView},
	// Metrics — any authenticated viewer.
	{prefix: "/metrics", method: "GET", perm: PermMetricsView},
}

type endpointPolicy struct {
	prefix string
	method string
	perm   Permission
}

// RequiredPermission returns the permission needed to access an endpoint,
// and whether any authentication is needed at all.
// Returns ("", false) for public endpoints (health checks).
func RequiredPermission(path, method string) (Permission, bool) {
	for _, ep := range endpointPermissions {
		if !hasPrefix(path, ep.prefix) {
			continue
		}
		if ep.method != "" && ep.method != method {
			continue
		}
		return ep.perm, true
	}
	return "", false
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
