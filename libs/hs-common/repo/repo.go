// Package repo provides sentinel errors and generic repository helpers shared
// across Home Stack API services.
package repo

import "errors"

var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrNoAccess indicates the caller lacks permission to access or mutate the entity.
	ErrNoAccess = errors.New("no access")
	// ErrConflict indicates a business-rule or uniqueness conflict.
	ErrConflict = errors.New("conflict")
)
