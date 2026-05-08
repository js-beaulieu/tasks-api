package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"sort"
)

const (
	advisoryTaskSiblingListNamespace int32 = 1
	advisoryProjectStatusesNamespace int32 = 2
)

type advisoryLock struct {
	namespace int32
	key       int32
	sortKey   string
}

func lockProjectStatuses(ctx context.Context, tx *sql.Tx, projectID string) error {
	return lockAdvisory(ctx, tx, advisoryLock{
		namespace: advisoryProjectStatusesNamespace,
		key:       advisoryKey(projectID),
		sortKey:   projectID,
	})
}

func lockTaskSiblingLists(ctx context.Context, tx *sql.Tx, lists ...taskSiblingList) error {
	locks := make([]advisoryLock, 0, len(lists))
	seen := map[string]bool{}
	for _, list := range lists {
		sortKey := list.key()
		if seen[sortKey] {
			continue
		}
		seen[sortKey] = true
		locks = append(locks, advisoryLock{
			namespace: advisoryTaskSiblingListNamespace,
			key:       advisoryKey(sortKey),
			sortKey:   sortKey,
		})
	}
	sort.Slice(locks, func(i, j int) bool {
		return locks[i].sortKey < locks[j].sortKey
	})
	for _, lock := range locks {
		if err := lockAdvisory(ctx, tx, lock); err != nil {
			return err
		}
	}
	return nil
}

type taskSiblingList struct {
	projectID string
	parentID  *string
}

func (l taskSiblingList) key() string {
	parentID := "<root>"
	if l.parentID != nil {
		parentID = *l.parentID
	}
	return l.projectID + ":" + parentID
}

func lockAdvisory(ctx context.Context, tx *sql.Tx, lock advisoryLock) error {
	if _, err := tx.ExecContext(ctx, bind(`SELECT pg_advisory_xact_lock(?, ?)`), lock.namespace, lock.key); err != nil {
		return fmt.Errorf("acquire advisory lock %q: %w", lock.sortKey, err)
	}
	return nil
}

func advisoryKey(value string) int32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return int32(h.Sum32())
}
