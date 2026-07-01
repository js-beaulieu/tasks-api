//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	testdb "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/db"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/seed"
	repoerr "github.com/js-beaulieu/hs-api/libs/hs-common/repo"
)

// ---- CreateProject additional statuses ----

func TestCreateProject_AdditionalStatuses(t *testing.T) {
	ctx := context.Background()

	t.Run("no extra statuses seeds exactly 3 defaults", func(t *testing.T) {
		_, store := testdb.Open(t)
		owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
		p := &model.Project{Name: "P1", OwnerID: owner.ID}
		if err := store.Projects.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		statuses, err := store.Projects.ListStatuses(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListStatuses: %v", err)
		}
		if len(statuses) != len(model.DefaultStatuses) {
			t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(model.DefaultStatuses))
		}
		for i, s := range statuses {
			if s.Status != model.DefaultStatuses[i] {
				t.Errorf("statuses[%d] = %q, want %q", i, s.Status, model.DefaultStatuses[i])
			}
			if s.Position != i {
				t.Errorf("statuses[%d].Position = %d, want %d", i, s.Position, i)
			}
		}
	})

	t.Run("extra statuses appended after defaults", func(t *testing.T) {
		_, store := testdb.Open(t)
		owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
		p := &model.Project{Name: "P2", OwnerID: owner.ID}
		if err := store.Projects.Create(ctx, p, "À faire", "En cours"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		statuses, err := store.Projects.ListStatuses(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListStatuses: %v", err)
		}
		if len(statuses) != 5 {
			t.Fatalf("len(statuses) = %d, want 5", len(statuses))
		}
		extras := statuses[3:]
		if extras[0].Status != "À faire" || extras[0].Position != 3 {
			t.Errorf("extras[0] = {%q, %d}, want {%q, 3}", extras[0].Status, extras[0].Position, "À faire")
		}
		if extras[1].Status != "En cours" || extras[1].Position != 4 {
			t.Errorf("extras[1] = {%q, %d}, want {%q, 4}", extras[1].Status, extras[1].Position, "En cours")
		}
	})

	t.Run("extra status duplicating a default is silently skipped", func(t *testing.T) {
		_, store := testdb.Open(t)
		owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
		p := &model.Project{Name: "P3", OwnerID: owner.ID}
		if err := store.Projects.Create(ctx, p, "todo", "extra"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		statuses, err := store.Projects.ListStatuses(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListStatuses: %v", err)
		}
		// "todo" is a duplicate — only "extra" should be appended
		if len(statuses) != 4 {
			t.Fatalf("len(statuses) = %d, want 4 (3 defaults + 1 non-duplicate extra)", len(statuses))
		}
		if statuses[3].Status != "extra" {
			t.Errorf("statuses[3] = %q, want %q", statuses[3].Status, "extra")
		}
	})
}

// ---- Create / Get ----

func TestProjects_CreateGet(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})

	desc := "a nice project"
	due := "2026-12-31"
	p := &model.Project{
		Name:        "My Project",
		Description: &desc,
		DueDate:     &due,
		OwnerID:     owner.ID,
	}
	if err := store.Projects.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("Create did not assign an ID to p.ID")
	}

	got, err := store.Projects.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.Name != "My Project" {
		t.Errorf("Name = %q, want %q", got.Name, "My Project")
	}
	if got.Description == nil || *got.Description != desc {
		t.Errorf("Description = %v, want %q", got.Description, desc)
	}
	if got.DueDate == nil || *got.DueDate != due {
		t.Errorf("DueDate = %v, want %q", got.DueDate, due)
	}
	if got.OwnerID != owner.ID {
		t.Errorf("OwnerID = %q, want %q", got.OwnerID, owner.ID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}
}

func TestProjects_Get_NotFound(t *testing.T) {
	_, store := testdb.Open(t)
	_, err := store.Projects.Get(context.Background(), "no-such-id")
	if err != repoerr.ErrNotFound {
		t.Errorf("err = %v, want repoerr.ErrNotFound", err)
	}
}

// ---- List ----

func TestProjects_List(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Owner", Email: "owner@test.com"})
	outsider := seed.User(t, store, seed.UserInput{ID: "u2", Name: "Outsider", Email: "out@test.com"})
	p := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t.Run("owner sees project", func(t *testing.T) {
		list, err := store.Projects.List(ctx, owner.ID)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		var found bool
		for _, proj := range list {
			if proj.ID == p.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("project %q not found in owner's list", p.ID)
		}
	})

	t.Run("non-member sees nothing", func(t *testing.T) {
		list, err := store.Projects.List(ctx, outsider.ID)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("non-member list len = %d, want 0", len(list))
		}
	})
}

// ---- Update ----

func TestProjects_Update(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Owner", Email: "owner@test.com"})

	desc := "original description"
	p := &model.Project{Name: "Original", Description: &desc, OwnerID: owner.ID}
	if err := store.Projects.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	t.Run("nil pointer field leaves column unchanged", func(t *testing.T) {
		p.Name = "Updated Name"
		p.Description = nil // nil → do not touch the DB column
		if err := store.Projects.Update(ctx, p); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := store.Projects.Get(ctx, p.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Name != "Updated Name" {
			t.Errorf("Name = %q, want %q", got.Name, "Updated Name")
		}
		if got.Description == nil || *got.Description != "original description" {
			t.Errorf("Description = %v, want %q (should be unchanged)", got.Description, "original description")
		}
	})

	t.Run("non-nil pointer field updates column", func(t *testing.T) {
		newDesc := "new description"
		p.Description = &newDesc
		if err := store.Projects.Update(ctx, p); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := store.Projects.Get(ctx, p.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Description == nil || *got.Description != newDesc {
			t.Errorf("Description = %v, want %q", got.Description, newDesc)
		}
	})
}

// ---- Delete ----

func TestProjects_Delete(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Owner", Email: "owner@test.com"})
	p := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	if err := store.Projects.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Projects.Get(ctx, p.ID)
	if err != repoerr.ErrNotFound {
		t.Errorf("err = %v, want repoerr.ErrNotFound after Delete", err)
	}
}

// ---- GetMemberRole ----

func TestProjects_GetMemberRole(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Owner", Email: "owner@test.com"})
	outsider := seed.User(t, store, seed.UserInput{ID: "u2", Name: "Outsider", Email: "out@test.com"})
	p := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t.Run("owner returns admin without project_members row", func(t *testing.T) {
		role, err := store.Projects.GetMemberRole(ctx, p.ID, owner.ID)
		if err != nil {
			t.Fatalf("GetMemberRole: %v", err)
		}
		if role != model.RoleAdmin {
			t.Errorf("role = %q, want %q", role, model.RoleAdmin)
		}
	})

	t.Run("non-member returns ErrNoAccess", func(t *testing.T) {
		_, err := store.Projects.GetMemberRole(ctx, p.ID, outsider.ID)
		if err != repoerr.ErrNoAccess {
			t.Errorf("err = %v, want repoerr.ErrNoAccess", err)
		}
	})
}

// ---- Members ----

func TestProjects_Members(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Owner", Email: "owner@test.com"})
	p := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t.Run("AddMember then role visible via GetMemberRole", func(t *testing.T) {
		userA := seed.User(t, store, seed.UserInput{ID: "u2", Name: "UserA", Email: "a@test.com"})
		m := &model.ProjectMember{ProjectID: p.ID, UserID: userA.ID, Role: model.RoleRead}
		if err := store.Projects.AddMember(ctx, m); err != nil {
			t.Fatalf("AddMember: %v", err)
		}
		role, err := store.Projects.GetMemberRole(ctx, p.ID, userA.ID)
		if err != nil {
			t.Fatalf("GetMemberRole: %v", err)
		}
		if role != model.RoleRead {
			t.Errorf("role = %q, want %q", role, model.RoleRead)
		}
	})

	t.Run("UpdateMemberRole changes role", func(t *testing.T) {
		userB := seed.User(t, store, seed.UserInput{ID: "u3", Name: "UserB", Email: "b@test.com"})
		if err := store.Projects.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: userB.ID, Role: model.RoleRead}); err != nil {
			t.Fatalf("AddMember: %v", err)
		}
		if err := store.Projects.UpdateMemberRole(ctx, p.ID, userB.ID, model.RoleModify); err != nil {
			t.Fatalf("UpdateMemberRole: %v", err)
		}
		role, err := store.Projects.GetMemberRole(ctx, p.ID, userB.ID)
		if err != nil {
			t.Fatalf("GetMemberRole: %v", err)
		}
		if role != model.RoleModify {
			t.Errorf("role = %q, want %q", role, model.RoleModify)
		}
	})

	t.Run("RemoveMember returns ErrNoAccess afterwards", func(t *testing.T) {
		userC := seed.User(t, store, seed.UserInput{ID: "u4", Name: "UserC", Email: "c@test.com"})
		if err := store.Projects.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: userC.ID, Role: model.RoleRead}); err != nil {
			t.Fatalf("AddMember: %v", err)
		}
		reassigned, err := store.Projects.RemoveMember(ctx, p.ID, userC.ID)
		if err != nil {
			t.Fatalf("RemoveMember: %v", err)
		}
		if reassigned != 0 {
			t.Errorf("reassigned = %d, want 0", reassigned)
		}
		_, err = store.Projects.GetMemberRole(ctx, p.ID, userC.ID)
		if err != repoerr.ErrNoAccess {
			t.Errorf("err = %v, want repoerr.ErrNoAccess after RemoveMember", err)
		}
	})

	t.Run("AddMember duplicate returns ErrConflict", func(t *testing.T) {
		userD := seed.User(t, store, seed.UserInput{ID: "u5", Name: "UserD", Email: "d@test.com"})
		m := &model.ProjectMember{ProjectID: p.ID, UserID: userD.ID, Role: model.RoleRead}
		if err := store.Projects.AddMember(ctx, m); err != nil {
			t.Fatalf("first AddMember: %v", err)
		}
		if err := store.Projects.AddMember(ctx, m); err != repoerr.ErrConflict {
			t.Errorf("duplicate AddMember: err = %v, want repoerr.ErrConflict", err)
		}
	})

	t.Run("ListMembers includes added member", func(t *testing.T) {
		userE := seed.User(t, store, seed.UserInput{ID: "u6", Name: "UserE", Email: "e@test.com"})
		if err := store.Projects.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: userE.ID, Role: model.RoleModify}); err != nil {
			t.Fatalf("AddMember: %v", err)
		}
		members, err := store.Projects.ListMembers(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		var found bool
		for _, mem := range members {
			if mem.UserID == userE.ID {
				found = true
				if mem.Role != model.RoleModify {
					t.Errorf("member role = %q, want %q", mem.Role, model.RoleModify)
				}
			}
		}
		if !found {
			t.Errorf("member %q not found in ListMembers result", userE.ID)
		}
	})

	t.Run("ListMembers includes owner as admin", func(t *testing.T) {
		members, err := store.Projects.ListMembers(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		ownerMember := findMember(members, owner.ID)
		if ownerMember == nil {
			t.Fatalf("owner %q not found in ListMembers result", owner.ID)
		}
		if ownerMember.Role != model.RoleAdmin {
			t.Errorf("owner role = %q, want %q", ownerMember.Role, model.RoleAdmin)
		}
	})

	t.Run("ListMembers does not duplicate owner if already an explicit member", func(t *testing.T) {
		if err := store.Projects.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: owner.ID, Role: model.RoleAdmin}); err != nil {
			t.Fatalf("AddMember owner: %v", err)
		}
		members, err := store.Projects.ListMembers(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		count := 0
		for _, m := range members {
			if m.UserID == owner.ID {
				count++
			}
		}
		if count != 1 {
			t.Errorf("owner appears %d times in members, want 1", count)
		}
	})

	t.Run("RemoveMember reassigns tasks to owner", func(t *testing.T) {
		memberUser := seed.User(t, store, seed.UserInput{ID: "u-reassign-1", Name: "Reassign Member", Email: "reassign1@test.com"})
		if err := store.Projects.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: memberUser.ID, Role: model.RoleModify}); err != nil {
			t.Fatalf("AddMember: %v", err)
		}

		assignedTask := seed.Task(t, store, seed.TaskInput{
			ProjectID:  p.ID,
			Name:       "Assigned Task",
			OwnerID:    owner.ID,
			AssigneeID: &memberUser.ID,
		})
		seed.Task(t, store, seed.TaskInput{
			ProjectID:  p.ID,
			Name:       "Another Assigned Task",
			OwnerID:    owner.ID,
			AssigneeID: &memberUser.ID,
		})
		seed.Task(t, store, seed.TaskInput{
			ProjectID: p.ID,
			Name:      "Unassigned Task",
			OwnerID:   owner.ID,
		})

		reassigned, err := store.Projects.RemoveMember(ctx, p.ID, memberUser.ID)
		if err != nil {
			t.Fatalf("RemoveMember: %v", err)
		}
		if reassigned != 2 {
			t.Errorf("reassigned = %d, want 2", reassigned)
		}

		updated, err := store.Tasks.Get(ctx, assignedTask.ID)
		if err != nil {
			t.Fatalf("Get task: %v", err)
		}
		if updated.AssigneeID == nil || *updated.AssigneeID != owner.ID {
			t.Errorf("AssigneeID = %v, want %q", updated.AssigneeID, owner.ID)
		}
	})
}

// ---- Statuses ----

func TestProjects_Statuses(t *testing.T) {
	sqlDB, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Owner", Email: "owner@test.com"})
	p := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t.Run("all 4 defaults seeded on Create", func(t *testing.T) {
		statuses, err := store.Projects.ListStatuses(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListStatuses: %v", err)
		}
		if len(statuses) != len(model.DefaultStatuses) {
			t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(model.DefaultStatuses))
		}
		got := make(map[string]bool, len(statuses))
		for _, s := range statuses {
			got[s.Status] = true
		}
		for _, want := range model.DefaultStatuses {
			if !got[want] {
				t.Errorf("default status %q missing", want)
			}
		}
	})

	t.Run("AddStatus appended at end", func(t *testing.T) {
		if err := store.Projects.AddStatus(ctx, p.ID, "review"); err != nil {
			t.Fatalf("AddStatus: %v", err)
		}
		statuses, err := store.Projects.ListStatuses(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListStatuses: %v", err)
		}
		last := statuses[len(statuses)-1]
		if last.Status != "review" {
			t.Errorf("last status = %q, want %q", last.Status, "review")
		}
	})

	t.Run("DeleteStatus with no tasks succeeds", func(t *testing.T) {
		if err := store.Projects.DeleteStatus(ctx, p.ID, "review"); err != nil {
			t.Fatalf("DeleteStatus: %v", err)
		}
		statuses, err := store.Projects.ListStatuses(ctx, p.ID)
		if err != nil {
			t.Fatalf("ListStatuses: %v", err)
		}
		for _, s := range statuses {
			if s.Status == "review" {
				t.Error("deleted status 'review' still present")
			}
		}
		for i, s := range statuses {
			if s.Position != i {
				t.Fatalf("status %q position = %d, want %d", s.Status, s.Position, i)
			}
		}
	})

	t.Run("DeleteStatus with active tasks returns ErrConflict", func(t *testing.T) {
		_, err := sqlDB.ExecContext(ctx,
			`INSERT INTO tasks (id, project_id, name, status, owner_id, position)
			 VALUES ('task-conflict-1', $1, 'Blocking Task', 'todo', $2, 0)`,
			p.ID, owner.ID,
		)
		if err != nil {
			t.Fatalf("insert raw task: %v", err)
		}
		if err := store.Projects.DeleteStatus(ctx, p.ID, "todo"); err != repoerr.ErrConflict {
			t.Errorf("err = %v, want repoerr.ErrConflict", err)
		}
	})
}

func findMember(members []*model.ProjectMember, userID string) *model.ProjectMember {
	for _, m := range members {
		if m.UserID == userID {
			return m
		}
	}
	return nil
}
