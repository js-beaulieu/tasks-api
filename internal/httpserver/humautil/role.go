package humautil

import "github.com/js-beaulieu/tasks-api/internal/model"

var RoleRank = map[string]int{
	model.RoleRead:   1,
	model.RoleModify: 2,
	model.RoleAdmin:  3,
}

func RequireRole(min, actual string) bool {
	return RoleRank[actual] >= RoleRank[min]
}

func ValidRole(r string) bool {
	_, ok := RoleRank[r]
	return ok
}
