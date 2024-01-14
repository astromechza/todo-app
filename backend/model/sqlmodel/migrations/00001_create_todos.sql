-- +goose Up

CREATE TABLE todos_groups (

    -- The unique identity of this item is made up of:
    --- the id which is unique right now, but may be duplicated over time
    id text not null,
    --- the version which is a unique nonce for this lifecycle of the id
    epoch bigint not null,
    --- the timestamp at which the version was assigned (== the created-at time)
    epoch_at timestamp with time zone not null,

    -- The item is assigned to a workspace which also has a limited unique lifecycle
    --- the unique workspace id
    workspace_id text not null,
    --- the version of the workspace this item is linked to
    workspace_epoch bigint not null,

    last_serial bigint not null,

    CONSTRAINT todos_groups_pk PRIMARY KEY (workspace_id, id)

);

CREATE TABLE todos (
    -- The unique identity of this item is made up of:
    --- the id which is unique right now, but may be duplicated over time
    id bigint not null,
    --- the version which is a unique nonce for this lifecycle of the id
    epoch bigint not null,
    --- the timestamp at which the version was assigned (== the created-at time)
    epoch_at timestamp with time zone not null,

    -- The update revision of the item is made up of:
    --- the revision number of updates
    revision bigint not null,
    --- the timestamp at which the revision number was assigned (== the updated-at time)
    revision_at timestamp with time zone not null,

    -- The item is assigned to a workspace which also has a limited unique lifecycle
    --- the unique workspace id
    group_id text not null,
    --- the version of the workspace this item is linked to
    group_epoch bigint not null,

    -- The item is assigned to a workspace which also has a limited unique lifecycle
    --- the unique workspace id
    workspace_id text not null,
    --- the version of the workspace this item is linked to
    workspace_epoch bigint not null,

    title text not null,
    details text,
    status text not null,

    CONSTRAINT todos_pk PRIMARY KEY (workspace_id, group_id, id),
    CONSTRAINT todos_group_fk FOREIGN KEY (workspace_id, group_id) REFERENCES todos_groups (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX todos_status_idx ON todos (status);

-- +goose Down

DROP TABLE IF EXISTS todos_group;
DROP TABLE IF EXISTS todos;
