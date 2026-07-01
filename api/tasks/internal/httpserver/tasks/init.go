package tasks

import (
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/libs/hs-common/humautil"
)

func init() {
	humautil.Rank = map[string]int{
		model.RoleRead:   1,
		model.RoleModify: 2,
		model.RoleAdmin:  3,
	}
}
