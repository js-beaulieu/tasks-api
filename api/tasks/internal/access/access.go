// Package access provides app-local role-ranking helpers for project access control.
package access

import (
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/libs/hs-common/role"
)

var rank = role.Rank{
	model.RoleRead:   1,
	model.RoleModify: 2,
	model.RoleAdmin:  3,
}

// RequireRole returns true if actual meets or exceeds min in the tasks API role hierarchy.
func RequireRole(min, actual string) bool { return rank.Require(min, actual) }

// ValidRole reports whether the given role is defined in the tasks API role hierarchy.
func ValidRole(r string) bool { return rank.Valid(r) }
