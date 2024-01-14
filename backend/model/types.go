package model

import (
	"context"
	"strings"
	"time"
)

const SharedWorkspaceId = "public"
const DefaultWorkspaceEpoch = 0
const DefaultGroupId = "TODO"
const DefaultGroupEpoch = 0

func SplitGroupId(id string) (string, string) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

type EntityReference struct {
	Id    string
	Epoch int64
}

type Todo struct {
	Id         int64
	Epoch      int64
	EpochAt    time.Time
	Revision   int64
	RevisionAt time.Time
	Workspace  EntityReference
	Group      EntityReference

	Title   string
	Status  string
	Details *string
}

type ListTodosParams struct {
	ByGroup   []string
	ByStatus  []string
	PageToken *string
	PageSize  *int
}

type ListTodosPage struct {
	Items          []Todo
	RemainingItems int
	NextPageToken  *string
}

type CreateTodosParams struct {
	GroupId string
	Title   string
	Details *string
}

type Modelling interface {
	GetTodo(ctx context.Context, workspaceId string, id string) (*Todo, error)
	ListTodos(ctx context.Context, workspaceId string, params ListTodosParams) (*ListTodosPage, error)
	CreateTodo(ctx context.Context, workspaceId string, params CreateTodosParams) (*Todo, error)
	DeleteTodo(ctx context.Context, workspaceId, id, epoch string, revision int64) error
	Close(ctx context.Context) error
}
