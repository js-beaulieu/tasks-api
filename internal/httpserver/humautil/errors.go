package humautil

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/js-beaulieu/tasks-api/internal/repo"
)

func RepoError(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
		return huma.Error404NotFound("not found")
	}
	if errors.Is(err, repo.ErrNoAccess) {
		return huma.Error403Forbidden("forbidden")
	}
	return huma.Error500InternalServerError("internal error")
}
