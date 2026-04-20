package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

var ListTagsTool = &mcp.Tool{
	Name:        "list_tags",
	Description: "List all distinct tags visible to the given user.",
}

type listTagsInput struct{}

type listTagsResult struct {
	Tags []string `json:"tags"`
}

func ListTagsHandler(tags repo.TagRepo) mcp.ToolHandlerFor[listTagsInput, *listTagsResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listTagsInput) (*mcp.CallToolResult, *listTagsResult, error) {
		userID := middleware.UserFromCtx(ctx).ID
		list, err := tags.ListDistinctForUser(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &listTagsResult{Tags: list}, nil
	}
}
