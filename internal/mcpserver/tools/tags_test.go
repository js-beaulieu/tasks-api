package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

func TestListTagsHandler(t *testing.T) {
	t.Run("valid user_id returns tag list without error", func(t *testing.T) {
		tr := &mock.TagRepo{
			ListDistinctForUserFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{"alpha", "beta", "gamma"}, nil
			},
		}
		handler := ListTagsHandler(tr)
		_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, listTagsInput{UserID: "u1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output == nil {
			t.Fatal("expected non-nil output")
		}
	})

	t.Run("missing user_id returns error", func(t *testing.T) {
		handler := ListTagsHandler(&mock.TagRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, listTagsInput{})
		if err == nil {
			t.Fatal("expected error for missing user_id")
		}
	})
}
