package api

import (
	"context"
	"fmt"

	"github.com/astromechza/todo-app/backend/model"
	"github.com/astromechza/todo-app/pkg/ref"
)

func toApiTodo(item *model.Todo) Todo {
	return Todo{
		Metadata: TodoMetadata{
			Id:             fmt.Sprintf("%s-%d", item.Group.Id, item.Id),
			Epoch:          int(item.Epoch),
			WorkspaceId:    item.Workspace.Id,
			WorkspaceEpoch: int(item.Workspace.Epoch),
			Revision:       int(item.Revision),
			CreatedAt:      item.EpochAt,
			UpdatedAt:      item.RevisionAt,
			GroupId:        item.Group.Id,
			GroupEpoch:     int(item.Group.Epoch),
		},
		Status:  item.Status,
		Details: item.Details,
		Title:   item.Title,
	}
}

func (s *Server) GetTodo(ctx context.Context, request GetTodoRequestObject) (GetTodoResponseObject, error) {
	res, err := s.Database.GetTodo(ctx, request.WorkspaceId, request.TodoId)
	if err != nil {
		return nil, err
	}
	return GetTodo200JSONResponse(toApiTodo(res)), nil
}

func (s *Server) ListTodos(ctx context.Context, request ListTodosRequestObject) (ListTodosResponseObject, error) {
	params := model.ListTodosParams{
		PageToken: request.Params.Page,
		PageSize:  request.Params.PageSize,
	}
	if request.Params.Status != nil {
		params.ByStatus = []string{*request.Params.Status}
	}

	res, err := s.Database.ListTodos(ctx, request.WorkspaceId, params)
	if err != nil {
		return nil, err
	}
	out := make([]Todo, len(res.Items))
	for i, item := range res.Items {
		out[i] = toApiTodo(&item)
	}
	return ListTodos200JSONResponse(TodoPage{
		Items:          out,
		RemainingItems: res.RemainingItems,
		NextPageToken:  res.NextPageToken,
	}), nil
}

func (s *Server) CreateTodo(ctx context.Context, request CreateTodoRequestObject) (CreateTodoResponseObject, error) {
	params := model.CreateTodosParams{
		GroupId: ref.DeRefOr(request.Body.GroupId, model.DefaultGroupId),
		Title:   request.Body.Title,
		Details: request.Body.Details,
	}
	if res, err := s.Database.CreateTodo(ctx, request.WorkspaceId, params); err != nil {
		return nil, err
	} else {
		return CreateTodo201JSONResponse(toApiTodo(res)), nil
	}
}

func (s *Server) DeleteTodo(ctx context.Context, request DeleteTodoRequestObject) (DeleteTodoResponseObject, error) {
	if err := s.Database.DeleteTodo(ctx, request.WorkspaceId, request.TodoId, model.DeleteTodosParams{}); err != nil {
		return nil, err
	}
	return DeleteTodo204Response{}, nil
}
