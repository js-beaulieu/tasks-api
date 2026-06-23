package model

import "time"

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description,omitempty"`
	DueDate       *string   `json:"due_date,omitempty"`
	OwnerID       string    `json:"owner_id"`
	AssigneeID    *string   `json:"assignee_id,omitempty"`
	EffectiveRole string    `json:"effective_role,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProjectMember struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
}

type ProjectStatus struct {
	ProjectID string `json:"project_id"`
	Status    string `json:"status"`
	Position  int    `json:"position"`
}

type Task struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ParentID    *string   `json:"parent_id,omitempty"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status"`
	DueDate     *string   `json:"due_date,omitempty"`
	OwnerID     string    `json:"owner_id"`
	AssigneeID  *string   `json:"assignee_id,omitempty"`
	Position    int       `json:"position"`
	Recurrence  *string   `json:"recurrence,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const (
	RoleRead   = "read"
	RoleModify = "modify"
	RoleAdmin  = "admin"
)

var DefaultStatuses = []string{"todo", "in_progress", "done", "cancelled"}

// PermanentStatuses cannot be deleted from a project.
// "cancelled" is a default but not permanent — admins can remove it.
var PermanentStatuses = []string{"todo", "in_progress", "done"}

func IsPermanentStatus(status string) bool {
	for _, s := range PermanentStatuses {
		if s == status {
			return true
		}
	}
	return false
}
