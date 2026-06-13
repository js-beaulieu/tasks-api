//go:build integration

package tags_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestTagsIntegration_AddAndListForTask(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{"tag": "backend"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID + "/tags", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tags []string
	httptestutil.Decode(t, res, &tags)
	if len(tags) != 1 || tags[0] != "backend" {
		t.Fatalf("tags = %v, want [backend]", tags)
	}
}

func TestTagsIntegration_ListDistinctForUser(t *testing.T) {
	env := httptestutil.NewEnv(t)
	otherUser := seed.User(t, env.Store, seed.UserInput{ID: "u-other-1", Name: "Other User"})

	ownProject := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID, Name: "Own Project"})
	ownTask := seed.Task(t, env.Store, seed.TaskInput{ProjectID: ownProject.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + ownTask.ID + "/tags", Body: map[string]any{"tag": "alpha"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}
	res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + ownTask.ID + "/tags", Body: map[string]any{"tag": "beta"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	t.Run("lists distinct tags visible to caller", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tags", Body: nil, UserID: env.User.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var tags []string
		httptestutil.Decode(t, res, &tags)

		found := map[string]bool{}
		for _, tg := range tags {
			found[tg] = true
		}
		if !found["alpha"] || !found["beta"] {
			t.Fatalf("expected alpha and beta in tags, got %v", tags)
		}
	})

	t.Run("excludes tags from inaccessible projects", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tags", Body: nil, UserID: otherUser.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var tags []string
		httptestutil.Decode(t, res, &tags)

		for _, tg := range tags {
			if tg == "alpha" || tg == "beta" {
				t.Fatalf("other user should not see tags from inaccessible project, got %v", tags)
			}
		}
	})
}

func TestTagsIntegration_DeduplicatesAcrossVisibleTasks(t *testing.T) {
	env := httptestutil.NewEnv(t)

	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task1 := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	task2 := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task1.ID + "/tags", Body: map[string]any{"tag": "shared"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}
	res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task2.ID + "/tags", Body: map[string]any{"tag": "shared"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tags", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tags []string
	httptestutil.Decode(t, res, &tags)

	count := 0
	for _, tg := range tags {
		if tg == "shared" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 occurrence of 'shared', got %d in %v", count, tags)
	}
}

func TestTagsIntegration_EmptyResultReturnsEmptyArray(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tags", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tags []string
	httptestutil.Decode(t, res, &tags)

	if tags == nil {
		t.Fatal("expected empty array [], got null")
	}
	if len(tags) != 0 {
		t.Fatalf("expected empty array, got %v", tags)
	}
}

func TestTagsIntegration_MemberSeesTagsFromSharedProject(t *testing.T) {
	env := httptestutil.NewEnv(t)
	member := seed.User(t, env.Store, seed.UserInput{ID: "u-member-1", Name: "Member User"})

	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{"tag": "gamma"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	if err := env.Store.Projects.AddMember(t.Context(), &model.ProjectMember{
		ProjectID: project.ID,
		UserID:    member.ID,
		Role:      model.RoleRead,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	res = httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tags", Body: nil, UserID: member.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tags []string
	httptestutil.Decode(t, res, &tags)

	found := false
	for _, tg := range tags {
		if tg == "gamma" {
			found = true
		}
	}
	if !found {
		t.Fatalf("member should see tag gamma from shared project, got %v", tags)
	}
}
