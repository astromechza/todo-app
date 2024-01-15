package sqlmodel

import (
	"context"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5"
	pgxlib "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/astromechza/todo-app/backend/model"
	"github.com/astromechza/todo-app/pkg/ref"
)

func init() {
	sql.Register("postgres", pgxlib.GetDefaultDriver())
}

func NewSqlModel(ctx context.Context, connString string) (model.Modelling, error) {
	if !strings.Contains(connString, "://") {
		return nil, fmt.Errorf("invalid database string, expected <driver>:// prefix")
	}
	driver := strings.Split(connString, "://")[0]
	if driver == "mysql" {
		connString = strings.TrimPrefix(connString, "mysql://")
	}

	// TODO: add sqltrace wrapper for opentracing
	db, err := sql.Open(driver, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: '%s' '%s' %w", driver, connString, err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(time.Minute * 10)
	db.SetConnMaxLifetime(time.Hour)
	logger := slog.With("driver", driver, "dsn", connString)

	logger.Info("verifying sql connection")
	_, err = db.ExecContext(ctx, `SELECT 1`)
	if err != nil {
		t := time.NewTicker(time.Second)
		defer t.Stop()
	Loop:
		for {
			logger.Warn("failed to connect to database", "err", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-t.C:
				logger.Info("verifying sql connection")
				if _, err = db.ExecContext(ctx, `SELECT 1`); err == nil {
					break Loop
				}
			}
		}
	}
	logger.Info("successfully connected to database")
	modelling := &sqlModel{driver: driver, db: db}

	if err := goose.SetDialect(driver); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	goose.SetTableName("todos_migrations")
	goose.SetLogger(&gooseLogger{logger: logger})
	goose.SetBaseFS(embedMigrations)
	if _, err := goose.EnsureDBVersionContext(ctx, db); err != nil {
		if !errors.Is(err, goose.ErrNoNextVersion) {
			_ = db.Close()
			return nil, fmt.Errorf("failed to check migration state: %w", err)
		} else if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS `+goose.TableName()); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to reset migration state: %w", err)
		}
	}

	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return modelling, nil
}

type sqlModel struct {
	driver string
	db     *sql.DB
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

type gooseLogger struct {
	logger *slog.Logger
}

func (g *gooseLogger) Fatalf(format string, v ...interface{}) {
}

func (g *gooseLogger) Printf(format string, v ...interface{}) {
	slog.Info(fmt.Sprintf(format, v...))
}

func (s *sqlModel) Close(ctx context.Context) error {
	return s.db.Close()
}

func (s *sqlModel) GetTodo(ctx context.Context, workspaceId string, id string) (*model.Todo, error) {
	groupPrefix, todoId := model.SplitGroupId(id)
	var out model.Todo
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT 
    	id, epoch, epoch_at, revision, revision_at, 
       	group_id, group_epoch, workspace_id, workspace_epoch,
		title, details, status
		FROM todos WHERE workspace_id = $1 AND group_id = $2 AND id = $3`,
		workspaceId, groupPrefix, todoId,
	).Scan(
		&out.Id, &out.Epoch, &out.EpochAt, &out.Revision, &out.RevisionAt,
		&out.Group.Id, &out.Group.Epoch, &out.Workspace.Id, &out.Workspace.Epoch,
		&out.Title, &out.Details, &out.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound("todo not found")
		}
		return nil, fmt.Errorf("failed to query and scan todo: %w", err)
	}
	return &out, nil
}

func (s *sqlModel) ListTodos(ctx context.Context, workspaceId string, params model.ListTodosParams) (*model.ListTodosPage, error) {
	var pageToken struct {
		LastGroupId string `json:"g"`
		LastId      int64  `json:"i"`
	}

	if params.PageToken != nil {
		rawToken, err := base64.RawURLEncoding.DecodeString(*params.PageToken)
		if err != nil {
			return nil, model.ErrBadRequest("failed to decode page token")
		}
		if err := json.Unmarshal(rawToken, &pageToken); err != nil {
			return nil, model.ErrBadRequest("failed to unmarshal page token")
		}
	}

	limit := 20
	if params.PageSize != nil {
		if *params.PageSize < 1 || *params.PageSize > 1000 {
			return nil, model.ErrBadRequest("page size out of range [1,1000]")
		}
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
    	id, epoch, epoch_at, revision, revision_at, 
       	group_id, group_epoch, workspace_id, workspace_epoch,
		title, details, status
		FROM todos 
		WHERE workspace_id = $1
	    AND ($2::text[] IS NULL OR group_id = ANY($2::text[]))
	    AND ($3::text[] IS NULL OR status = ANY($3::text[]))
	  	AND group_id > $4 OR (group_id = $4 AND id > $5)
		ORDER BY workspace_id, group_id, id
	  	LIMIT $6`,
		workspaceId, params.ByGroup, params.ByStatus, pageToken.LastGroupId, pageToken.LastId, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query todos: %w", err)
	}
	defer rows.Close()
	outRows := make([]model.Todo, 0)
	for rows.Next() {
		var out model.Todo
		if err := rows.Scan(
			&out.Id, &out.Epoch, &out.EpochAt, &out.Revision, &out.RevisionAt,
			&out.Group.Id, &out.Group.Epoch, &out.Workspace.Id, &out.Workspace.Epoch,
			&out.Title, &out.Details, &out.Status,
		); err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		outRows = append(outRows, out)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan todos: %w", err)
	}

	var remaining int
	if len(outRows) > 0 {
		pageToken.LastGroupId = outRows[len(outRows)-1].Group.Id
		pageToken.LastId = outRows[len(outRows)-1].Id
	}
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM todos 
		WHERE workspace_id = $1
	    AND ($2::text[] IS NULL OR group_id = ANY($2::text[]))
	    AND ($3::text[] IS NULL OR status = ANY($3::text[]))
	  	AND group_id > $4 OR (group_id = $4 AND id > $5)`,
		workspaceId, params.ByGroup, params.ByStatus, pageToken.LastGroupId, pageToken.LastId,
	).Scan(&remaining); err != nil {
		return nil, fmt.Errorf("failed to query and scan remaining count: %w", err)
	}

	page := &model.ListTodosPage{
		Items:          outRows,
		RemainingItems: remaining,
	}
	if len(outRows) > 0 && remaining > 0 {
		if raw, err := json.Marshal(pageToken); err != nil {
			return nil, fmt.Errorf("failed to marshal page token: %w", err)
		} else {
			page.NextPageToken = ref.Ref(base64.RawURLEncoding.EncodeToString(raw))
		}
	}

	return page, nil
}

func (s *sqlModel) CreateTodo(ctx context.Context, workspaceId string, params model.CreateTodosParams) (*model.Todo, error) {
	// TODO: actually need to validate this workspace and workspace epoch from somewhere.
	//		this should require that we can check against our in-memory workspace data source?
	// 		for now we're going to assume workspaces only have 1 epoch

	if workspaceId != model.SharedWorkspaceId {
		return nil, model.ErrNotFound(fmt.Sprintf("workspace '%s' not found", workspaceId))
	}

	var workspaceEpoch int64
	var groupEpoch int64
	var nextId int
	if err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO todos_groups (id, epoch, epoch_at, workspace_id, workspace_epoch, last_serial) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (workspace_id, id) DO UPDATE SET last_serial = todos_groups.last_serial + 1 RETURNING workspace_epoch, epoch, last_serial`,
		params.GroupId, model.DefaultGroupEpoch, time.Now().UTC(), workspaceId, model.DefaultWorkspaceEpoch, 1,
	).Scan(&workspaceEpoch, &groupEpoch, &nextId); err != nil {
		return nil, fmt.Errorf("failed to create or increment group: %w", err)
	}

	now := time.Now().UTC()
	out := model.Todo{
		Id:         int64(nextId),
		Epoch:      rand.Int63(),
		EpochAt:    now,
		Revision:   0,
		RevisionAt: now,
		Workspace: model.EntityReference{
			Id:    workspaceId,
			Epoch: workspaceEpoch,
		},
		Group: model.EntityReference{
			Id:    params.GroupId,
			Epoch: groupEpoch,
		},
		Title:   params.Title,
		Status:  "open",
		Details: params.Details,
	}

	if res, err := s.db.ExecContext(
		ctx,
		`INSERT INTO todos (id, epoch, epoch_at, revision, revision_at, group_id, group_epoch, workspace_id, workspace_epoch, title, details, status) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		out.Id, out.Epoch, out.EpochAt, out.Revision, out.RevisionAt, out.Group.Id, out.Group.Epoch, out.Workspace.Id, out.Workspace.Epoch, out.Title, ref.DeRefToNullString(out.Details), out.Status,
	); err != nil {
		return nil, fmt.Errorf("failed to insert todo: %w", err)
	} else if count, _ := res.RowsAffected(); count == 0 {
		return nil, fmt.Errorf("no rows inserted")
	}
	return &out, nil
}

func (s *sqlModel) DeleteTodo(ctx context.Context, workspaceId string, id string, params model.DeleteTodosParams) error {
	groupId, todoId := model.SplitGroupId(id)
	var deleted bool
	var returnedRevision int64
	if err := s.db.QueryRowContext(
		ctx,
		`WITH deleted_revisions AS (
    		DELETE FROM todos WHERE workspace_id = $1 AND group_id = $2 AND id = $3 AND ($4 = 0 OR epoch = $4) AND ($5 = 0 OR revision = $5) RETURNING true, revision
		), remaining_revisions AS (
			SELECT false, revision FROM todos WHERE workspace_id = $1 AND group_id = $2 AND id = $3 AND ($4 = 0 OR epoch = $4) AND ($5 != 0 AND revision != $5)
		)
		SELECT * FROM deleted_revisions UNION ALL SELECT * FROM remaining_revisions`,
		workspaceId, groupId, todoId, ref.DeRefOr(params.Epoch, 0), ref.DeRefOr(params.Revision, 0),
	).Scan(&deleted, &returnedRevision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound("todo not found")
		}
		return fmt.Errorf("failed to exec delete: %w", err)
	}
	if deleted {
		return nil
	}
	return model.ErrBadRequest("incorrect revision number")
}
