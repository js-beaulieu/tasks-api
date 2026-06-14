//go:build integration

package projects_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func createUser(t *testing.T, env *httptestutil.Env, id, name string) *model.User {
	t.Helper()
	u, err := env.Store.Users.Create(context.Background(), id, name, id+"@example.com")
	if err != nil {
		t.Fatalf("create user %q: %v", id, err)
	}
	return u
}

func addMember(t *testing.T, env *httptestutil.Env, projectID, userID, role string) {
	t.Helper()
	m := &model.ProjectMember{ProjectID: projectID, UserID: userID, Role: role}
	if err := env.Store.Projects.AddMember(context.Background(), m); err != nil {
		t.Fatalf("add member %s/%s: %v", userID, role, err)
	}
}

func projectPath(id string) string { return "/projects/" + id }
func memberPath(projectID, userID string) string {
	return fmt.Sprintf("/projects/%s/members/%s", projectID, userID)
}
func statusPath(projectID, status string) string {
	return fmt.Sprintf("/projects/%s/statuses/%s", projectID, status)
}

func assertNoContent(t *testing.T, res *http.Response) {
	t.Helper()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNoContent)
	}
}

func findMemberByID(members []*model.ProjectMember, userID string) *model.ProjectMember {
	for _, m := range members {
		if m.UserID == userID {
			return m
		}
	}
	return nil
}

func findStatus(statuses []*model.ProjectStatus, status string) *model.ProjectStatus {
	for _, s := range statuses {
		if s.Status == status {
			return s
		}
	}
	return nil
}

func projectIDs(projects []*model.Project) []string {
	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	return ids
}

// ── Projects ──────────────────────────────────────────────────────────────────

func TestProjectsIntegration_CreateWithOptionalFields(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects", Body: map[string]any{
		"name":        "Proj A",
		"description": "desc",
		"due_date":    "2026-07-01",
		"statuses":    []string{"review", "blocked"},
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	var p model.Project
	httptestutil.Decode(t, res, &p)
	if p.Name != "Proj A" {
		t.Fatalf("name = %q, want %q", p.Name, "Proj A")
	}
	if p.Description == nil || *p.Description != "desc" {
		t.Fatalf("description = %v, want %q", p.Description, "desc")
	}
	if p.DueDate == nil || *p.DueDate != "2026-07-01" {
		t.Fatalf("due_date = %v, want %q", p.DueDate, "2026-07-01")
	}
	if p.OwnerID != env.User.ID {
		t.Fatalf("owner_id = %q, want %q", p.OwnerID, env.User.ID)
	}

	statuses, err := env.Store.Projects.ListStatuses(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("list statuses: %v", err)
	}
	wantStatuses := []string{"todo", "in_progress", "done", "cancelled", "review", "blocked"}
	if len(statuses) != len(wantStatuses) {
		t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(wantStatuses))
	}
	for i, s := range statuses {
		if s.Status != wantStatuses[i] {
			t.Fatalf("statuses[%d] = %q, want %q", i, s.Status, wantStatuses[i])
		}
	}
}

func TestProjectsIntegration_CreateValidation(t *testing.T) {
	env := httptestutil.NewEnv(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing name", `{"description":"x"}`, http.StatusUnprocessableEntity},
		{"blank name", `{"name":"  "}`, http.StatusUnprocessableEntity},
		{"invalid JSON", `{not json}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects", Body: tt.body, UserID: env.User.ID})
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestProjectsIntegration_ListIncludesOwnedAndMember(t *testing.T) {
	env := httptestutil.NewEnv(t)
	other := createUser(t, env, "u-other", "Other")

	owned := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	member := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: other.ID})
	addMember(t, env, member.ID, env.User.ID, model.RoleRead)
	inaccessible := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: other.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/projects", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var projects []*model.Project
	httptestutil.Decode(t, res, &projects)
	ids := projectIDs(projects)
	if !containsStr(ids, owned.ID) {
		t.Fatal("owned project missing from list")
	}
	if !containsStr(ids, member.ID) {
		t.Fatal("member project missing from list")
	}
	if containsStr(ids, inaccessible.ID) {
		t.Fatal("inaccessible project should not appear in list")
	}
}

func TestProjectsIntegration_Get(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	reader := createUser(t, env, "u-reader", "Reader")
	outsider := createUser(t, env, "u-outside", "Outsider")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, reader.ID, model.RoleRead)

	t.Run("owner access", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: projectPath(p.ID), Body: nil, UserID: owner.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	t.Run("reader access", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: projectPath(p.ID), Body: nil, UserID: reader.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	t.Run("no access 403", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: projectPath(p.ID), Body: nil, UserID: outsider.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("not found", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: projectPath("nonexistent"), Body: nil, UserID: owner.ID})
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
		}
	})
}

func TestProjectsIntegration_Update(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	modifier := createUser(t, env, "u-mod", "Modifier")
	reader := createUser(t, env, "u-reader", "Reader")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, modifier.ID, model.RoleModify)
	addMember(t, env, p.ID, reader.ID, model.RoleRead)

	t.Run("owner can update", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: projectPath(p.ID), Body: map[string]any{
			"name": "Updated Name",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var updated model.Project
		httptestutil.Decode(t, res, &updated)
		if updated.Name != "Updated Name" {
			t.Fatalf("name = %q, want %q", updated.Name, "Updated Name")
		}

		got, err := env.Store.Projects.Get(context.Background(), p.ID)
		if err != nil {
			t.Fatalf("get project: %v", err)
		}
		if got.Name != "Updated Name" {
			t.Fatalf("db name = %q, want %q", got.Name, "Updated Name")
		}
	})

	t.Run("modifier can update", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: projectPath(p.ID), Body: map[string]any{
			"name": "Modifier Update",
		}, UserID: modifier.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	t.Run("reader forbidden", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: projectPath(p.ID), Body: map[string]any{
			"name": "Should Fail",
		}, UserID: reader.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: projectPath(p.ID), Body: `{bad`, UserID: owner.ID})
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestProjectsIntegration_Delete(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	modifier := createUser(t, env, "u-mod2", "Modifier2")
	reader := createUser(t, env, "u-reader2", "Reader2")

	t.Run("owner can delete", func(t *testing.T) {
		p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: projectPath(p.ID), Body: nil, UserID: owner.ID})
		assertNoContent(t, res)

		if _, err := env.Store.Projects.Get(context.Background(), p.ID); err == nil {
			t.Fatal("project should be deleted from DB")
		}
	})

	t.Run("modifier forbidden", func(t *testing.T) {
		p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
		addMember(t, env, p.ID, modifier.ID, model.RoleModify)
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: projectPath(p.ID), Body: nil, UserID: modifier.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("reader forbidden", func(t *testing.T) {
		p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
		addMember(t, env, p.ID, reader.ID, model.RoleRead)
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: projectPath(p.ID), Body: nil, UserID: reader.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})
}

// ── Members ───────────────────────────────────────────────────────────────────

func TestProjectsIntegration_ListMembers(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})

	member := createUser(t, env, "u-mem1", "Member1")
	addMember(t, env, p.ID, member.ID, model.RoleModify)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: projectPath(p.ID) + "/members", Body: nil, UserID: owner.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var members []*model.ProjectMember
	httptestutil.Decode(t, res, &members)
	m := findMemberByID(members, member.ID)
	if m == nil {
		t.Fatal("added member not found in members list")
	}
	if m.Role != model.RoleModify {
		t.Fatalf("role = %q, want %q", m.Role, model.RoleModify)
	}

	ownerMember := findMemberByID(members, owner.ID)
	if ownerMember == nil {
		t.Fatalf("owner %q not found in members list", owner.ID)
	}
	if ownerMember.Role != model.RoleAdmin {
		t.Errorf("owner role = %q, want %q", ownerMember.Role, model.RoleAdmin)
	}
}

func TestProjectsIntegration_AddMember(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	admin := createUser(t, env, "u-admin1", "Admin")
	modifier := createUser(t, env, "u-mod3", "Modifier3")
	reader := createUser(t, env, "u-read3", "Reader3")
	target := createUser(t, env, "u-target1", "Target")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, admin.ID, model.RoleAdmin)
	addMember(t, env, p.ID, modifier.ID, model.RoleModify)
	addMember(t, env, p.ID, reader.ID, model.RoleRead)

	tests := []struct {
		name       string
		userID     string
		body       map[string]any
		wantStatus int
	}{
		{"admin adds read role", admin.ID, map[string]any{"user_id": target.ID, "role": model.RoleRead}, http.StatusCreated},
		{"modifier forbidden", modifier.ID, map[string]any{"user_id": target.ID, "role": model.RoleRead}, http.StatusForbidden},
		{"reader forbidden", reader.ID, map[string]any{"user_id": target.ID, "role": model.RoleRead}, http.StatusForbidden},
		{"invalid role", owner.ID, map[string]any{"user_id": target.ID, "role": "superuser"}, http.StatusUnprocessableEntity},
		{"blank user_id", owner.ID, map[string]any{"user_id": "", "role": model.RoleRead}, http.StatusUnprocessableEntity},
		{"self add", owner.ID, map[string]any{"user_id": owner.ID, "role": model.RoleRead}, http.StatusUnprocessableEntity},
		{"invalid JSON", owner.ID, nil, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.body
			var res *http.Response
			if tt.name == "invalid JSON" {
				res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/members", Body: `{bad`, UserID: tt.userID})
			} else {
				res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/members", Body: body, UserID: tt.userID})
			}
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestProjectsIntegration_AddMemberRoles(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User

	for _, role := range []string{model.RoleRead, model.RoleModify, model.RoleAdmin} {
		t.Run("role_"+role, func(t *testing.T) {
			p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
			target := createUser(t, env, "u-"+role+"-"+p.ID[:4], "User")

			res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/members", Body: map[string]any{
				"user_id": target.ID,
				"role":    role,
			}, UserID: owner.ID})
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
			}

			var m model.ProjectMember
			httptestutil.Decode(t, res, &m)
			if m.Role != role {
				t.Fatalf("role = %q, want %q", m.Role, role)
			}
		})
	}
}

func TestProjectsIntegration_UpdateMember(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	admin := createUser(t, env, "u-admin2", "Admin2")
	member := createUser(t, env, "u-member1", "Member1")
	outsider := createUser(t, env, "u-outside2", "Outsider2")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, admin.ID, model.RoleAdmin)
	addMember(t, env, p.ID, member.ID, model.RoleRead)

	t.Run("admin updates role", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: memberPath(p.ID, member.ID), Body: map[string]any{
			"role": model.RoleModify,
		}, UserID: admin.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}
	})

	t.Run("reject owner role change", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: memberPath(p.ID, owner.ID), Body: map[string]any{
			"role": model.RoleModify,
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: memberPath(p.ID, member.ID), Body: map[string]any{
			"role": "superuser",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: memberPath(p.ID, member.ID), Body: `{bad`, UserID: owner.ID})
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: memberPath(p.ID, member.ID), Body: map[string]any{
			"role": model.RoleModify,
		}, UserID: outsider.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})
}

func TestProjectsIntegration_RemoveMember(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	member := createUser(t, env, "u-member2", "Member2")
	outsider := createUser(t, env, "u-outside3", "Outsider3")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, member.ID, model.RoleRead)

	t.Run("owner removes member", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: memberPath(p.ID, member.ID), Body: nil, UserID: owner.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}
		var got struct {
			Reassigned int `json:"reassigned"`
		}
		httptestutil.Decode(t, res, &got)
		if got.Reassigned != 0 {
			t.Errorf("reassigned = %d, want 0", got.Reassigned)
		}
	})

	t.Run("reject owner removal", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: memberPath(p.ID, owner.ID), Body: nil, UserID: owner.ID})
		if res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		p2 := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
		m2 := createUser(t, env, "u-member3", "Member3")
		addMember(t, env, p2.ID, m2.ID, model.RoleRead)

		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: memberPath(p2.ID, m2.ID), Body: nil, UserID: outsider.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("removal reassigns tasks to owner", func(t *testing.T) {
		p3 := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
		m3 := createUser(t, env, "u-reassign", "ReassignMember")
		addMember(t, env, p3.ID, m3.ID, model.RoleModify)

		seed.Task(t, env.Store, seed.TaskInput{ProjectID: p3.ID, Name: "Assigned 1", OwnerID: owner.ID, AssigneeID: &m3.ID})
		seed.Task(t, env.Store, seed.TaskInput{ProjectID: p3.ID, Name: "Assigned 2", OwnerID: owner.ID, AssigneeID: &m3.ID})
		seed.Task(t, env.Store, seed.TaskInput{ProjectID: p3.ID, Name: "Unassigned", OwnerID: owner.ID})

		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: memberPath(p3.ID, m3.ID), Body: nil, UserID: owner.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}
		var got struct {
			Reassigned int `json:"reassigned"`
		}
		httptestutil.Decode(t, res, &got)
		if got.Reassigned != 2 {
			t.Errorf("reassigned = %d, want 2", got.Reassigned)
		}
	})
}

// ── Statuses ──────────────────────────────────────────────────────────────────

func TestProjectsIntegration_ListStatusesOrder(t *testing.T) {
	env := httptestutil.NewEnv(t)
	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID, AdditionalStatuses: []string{"review"}})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: projectPath(p.ID) + "/statuses", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var statuses []*model.ProjectStatus
	httptestutil.Decode(t, res, &statuses)

	want := []string{"todo", "in_progress", "done", "cancelled", "review"}
	if len(statuses) != len(want) {
		t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(want))
	}
	for i, s := range statuses {
		if s.Status != want[i] {
			t.Fatalf("statuses[%d] = %q, want %q", i, s.Status, want[i])
		}
		if s.Position != i {
			t.Fatalf("statuses[%d].Position = %d, want %d", i, s.Position, i)
		}
	}
}

func TestProjectsIntegration_AddStatus(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	modifier := createUser(t, env, "u-mod4", "Modifier4")
	reader := createUser(t, env, "u-read4", "Reader4")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, modifier.ID, model.RoleModify)
	addMember(t, env, p.ID, reader.ID, model.RoleRead)

	t.Run("owner adds status", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/statuses", Body: map[string]any{
			"status": "blocked",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
		}

		statuses, _ := env.Store.Projects.ListStatuses(context.Background(), p.ID)
		if s := findStatus(statuses, "blocked"); s == nil {
			t.Fatal("blocked status not found in DB")
		}
	})

	t.Run("modifier forbidden", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/statuses", Body: map[string]any{
			"status": "nope",
		}, UserID: modifier.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("reader forbidden", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/statuses", Body: map[string]any{
			"status": "nope",
		}, UserID: reader.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("blank status 422", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/statuses", Body: map[string]any{
			"status": "  ",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid JSON 400", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: projectPath(p.ID) + "/statuses", Body: `{bad`, UserID: owner.ID})
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestProjectsIntegration_DeleteStatus(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	modifier := createUser(t, env, "u-mod5", "Modifier5")

	t.Run("delete unused custom status succeeds", func(t *testing.T) {
		p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID, AdditionalStatuses: []string{"review"}})
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: statusPath(p.ID, "review"), Body: nil, UserID: owner.ID})
		assertNoContent(t, res)

		statuses, _ := env.Store.Projects.ListStatuses(context.Background(), p.ID)
		if findStatus(statuses, "review") != nil {
			t.Fatal("review status should be deleted")
		}
	})

	t.Run("in-use status 409", func(t *testing.T) {
		p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID, AdditionalStatuses: []string{"review"}})
		seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: owner.ID, Status: "review"})

		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: statusPath(p.ID, "review"), Body: nil, UserID: owner.ID})
		if res.StatusCode != http.StatusConflict {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusConflict)
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID, AdditionalStatuses: []string{"review"}})
		addMember(t, env, p.ID, modifier.ID, model.RoleModify)

		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: statusPath(p.ID, "review"), Body: nil, UserID: modifier.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
