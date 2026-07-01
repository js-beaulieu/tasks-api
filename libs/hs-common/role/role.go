// Package role provides generic role-ranking helpers used by Home Stack API services.
package role

// Rank maps role names to numeric precedence.
type Rank map[string]int

// Require returns true if actual meets or exceeds min according to the rank map.
func (r Rank) Require(min, actual string) bool {
	return r[actual] >= r[min]
}

// Valid reports whether r defines the given role.
func (r Rank) Valid(role string) bool {
	_, ok := r[role]
	return ok
}
