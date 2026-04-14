package projects

import "github.com/js-beaulieu/tasks/internal/model"

var roleRank = map[string]int{
	model.RoleRead:   1,
	model.RoleModify: 2,
	model.RoleAdmin:  3,
}

// RequireRole returns true if actual role satisfies the minimum required role.
func RequireRole(min, actual string) bool {
	return roleRank[actual] >= roleRank[min]
}

// validRole returns true if r is one of the three known roles.
func validRole(r string) bool {
	_, ok := roleRank[r]
	return ok
}
