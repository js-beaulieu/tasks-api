package tools

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/repo"
)

var ListTagsTool = &mcp.Tool{
	Name:        "list_tags",
	Description: "List all distinct tags visible to the given user.",
}

type listTagsInput struct {
	UserID string `json:"user_id"`
}

type listTagsResult struct {
	Tags []string `json:"tags"`
}

func ListTagsHandler(tags repo.TagRepo) mcp.ToolHandlerFor[listTagsInput, *listTagsResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listTagsInput) (*mcp.CallToolResult, *listTagsResult, error) {
		if in.UserID == "" {
			return nil, nil, errors.New("user_id is required")
		}
		list, err := tags.ListDistinctForUser(ctx, in.UserID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &listTagsResult{Tags: list}, nil
	}
}
