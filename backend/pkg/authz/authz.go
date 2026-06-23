package authz

var rolePermissions = map[string][]string{
	"admin": {"*"},
	"operator": {
		"instances:read", "instances:write", "instances:delete",
		"hosts:read", "hosts:write",
		"backups:read", "backups:write",
		"upgrades:read", "upgrades:write",
		"switch:read", "switch:write",
		"migrations:read", "migrations:write",
		"deployments:read", "deployments:write", "deployments:delete",
		"topology:read",
		"monitoring:read",
		"alerts:read", "alerts:write",
		"tasks:read",
		"env-checks:read", "env-checks:write",
	},
	"viewer": {
		"instances:read",
		"hosts:read",
		"backups:read",
		"upgrades:read",
		"topology:read",
		"monitoring:read",
		"alerts:read",
		"tasks:read",
		"audit-logs:read",
	},
}

func HasPermission(role, permission string) bool {
	if role == "" {
		role = "viewer"
	}
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == "*" || p == permission {
			return true
		}
	}
	return false
}

func IsAdmin(role string) bool {
	return role == "admin"
}
