// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.24.0
// source: events.sql

package dbsqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const countEvents = `-- name: CountEvents :one
SELECT
    count(*) OVER() AS total
FROM
    "Event" as events
LEFT JOIN
  "WorkflowRunTriggeredBy" as runTriggers ON events."id" = runTriggers."eventId"
LEFT JOIN
  "WorkflowRun" as runs ON runTriggers."parentId" = runs."id"
LEFT JOIN
  "WorkflowVersion" as workflowVersion ON workflowVersion."id" = runs."workflowVersionId"
LEFT JOIN
  "Workflow" as workflow ON workflowVersion."workflowId" = workflow."id"
WHERE
  events."tenantId" = $1 AND
  (
    $2::text[] IS NULL OR
    events."key" = ANY($2::text[])
    ) AND
  (
    ($3::text[])::uuid[] IS NULL OR
    (workflow."id" = ANY($3::text[]::uuid[]))
    ) AND
  (
    $4::text IS NULL OR
    jsonb_path_exists(events."data", cast(concat('$.** ? (@.type() == "string" && @ like_regex "', $4::text, '")') as jsonpath))
  ) AND
    (
        $5::text[] IS NULL OR
        "status" = ANY(cast($5::text[] as "WorkflowRunStatus"[]))
    )
`

type CountEventsParams struct {
	TenantId  pgtype.UUID `json:"tenantId"`
	Keys      []string    `json:"keys"`
	Workflows []string    `json:"workflows"`
	Search    pgtype.Text `json:"search"`
	Statuses  []string    `json:"statuses"`
}

func (q *Queries) CountEvents(ctx context.Context, db DBTX, arg CountEventsParams) (int64, error) {
	row := db.QueryRow(ctx, countEvents,
		arg.TenantId,
		arg.Keys,
		arg.Workflows,
		arg.Search,
		arg.Statuses,
	)
	var total int64
	err := row.Scan(&total)
	return total, err
}

const createEvent = `-- name: CreateEvent :one
INSERT INTO "Event" (
    "id",
    "createdAt",
    "updatedAt",
    "deletedAt",
    "key",
    "tenantId",
    "replayedFromId",
    "data"
) VALUES (
    $1::uuid,
    coalesce($2::timestamp, CURRENT_TIMESTAMP),
    coalesce($3::timestamp, CURRENT_TIMESTAMP),
    $4::timestamp,
    $5::text,
    $6::uuid,
    $7::uuid,
    $8::jsonb
) RETURNING id, "createdAt", "updatedAt", "deletedAt", key, "tenantId", "replayedFromId", data
`

type CreateEventParams struct {
	ID             pgtype.UUID      `json:"id"`
	CreatedAt      pgtype.Timestamp `json:"createdAt"`
	UpdatedAt      pgtype.Timestamp `json:"updatedAt"`
	Deletedat      pgtype.Timestamp `json:"deletedat"`
	Key            string           `json:"key"`
	Tenantid       pgtype.UUID      `json:"tenantid"`
	ReplayedFromId pgtype.UUID      `json:"replayedFromId"`
	Data           []byte           `json:"data"`
}

func (q *Queries) CreateEvent(ctx context.Context, db DBTX, arg CreateEventParams) (*Event, error) {
	row := db.QueryRow(ctx, createEvent,
		arg.ID,
		arg.CreatedAt,
		arg.UpdatedAt,
		arg.Deletedat,
		arg.Key,
		arg.Tenantid,
		arg.ReplayedFromId,
		arg.Data,
	)
	var i Event
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
		&i.Key,
		&i.TenantId,
		&i.ReplayedFromId,
		&i.Data,
	)
	return &i, err
}

const getEventsForRange = `-- name: GetEventsForRange :many
SELECT
    date_trunc('hour', "createdAt") AS event_hour,
    COUNT(*) AS event_count
FROM
    "Event"
WHERE
    "createdAt" >= NOW() - INTERVAL '1 week'
GROUP BY
    event_hour
ORDER BY
    event_hour
`

type GetEventsForRangeRow struct {
	EventHour  pgtype.Interval `json:"event_hour"`
	EventCount int64           `json:"event_count"`
}

func (q *Queries) GetEventsForRange(ctx context.Context, db DBTX) ([]*GetEventsForRangeRow, error) {
	rows, err := db.Query(ctx, getEventsForRange)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetEventsForRangeRow
	for rows.Next() {
		var i GetEventsForRangeRow
		if err := rows.Scan(&i.EventHour, &i.EventCount); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listEvents = `-- name: ListEvents :many
SELECT
    events.id, events."createdAt", events."updatedAt", events."deletedAt", events.key, events."tenantId", events."replayedFromId", events.data,
    sum(case when runs."status" = 'PENDING' then 1 else 0 end) AS pendingRuns,
    sum(case when runs."status" = 'RUNNING' then 1 else 0 end) AS runningRuns,
    sum(case when runs."status" = 'SUCCEEDED' then 1 else 0 end) AS succeededRuns,
    sum(case when runs."status" = 'FAILED' then 1 else 0 end) AS failedRuns
FROM
    "Event" as events
LEFT JOIN
    "WorkflowRunTriggeredBy" as runTriggers ON events."id" = runTriggers."eventId"
LEFT JOIN
    "WorkflowRun" as runs ON runTriggers."parentId" = runs."id"
LEFT JOIN
    "WorkflowVersion" as workflowVersion ON workflowVersion."id" = runs."workflowVersionId"
LEFT JOIN
    "Workflow" as workflow ON workflowVersion."workflowId" = workflow."id"
WHERE
    events."tenantId" = $1 AND
    (
        $2::text[] IS NULL OR
        events."key" = ANY($2::text[])
    ) AND
    (
        ($3::text[])::uuid[] IS NULL OR
        (workflow."id" = ANY($3::text[]::uuid[]))
    ) AND
    (
        $4::text IS NULL OR
        workflow.name like concat('%', $4::text, '%') OR
        jsonb_path_exists(events."data", cast(concat('$.** ? (@.type() == "string" && @ like_regex "', $4::text, '")') as jsonpath))
    ) AND
    (
        $5::text[] IS NULL OR
        "status" = ANY(cast($5::text[] as "WorkflowRunStatus"[]))
    )
GROUP BY
    events."id"
ORDER BY
    case when $6 = 'createdAt ASC' THEN events."createdAt" END ASC ,
    case when $6 = 'createdAt DESC' then events."createdAt" END DESC
OFFSET
    COALESCE($7, 0)
LIMIT
    COALESCE($8, 50)
`

type ListEventsParams struct {
	TenantId  pgtype.UUID `json:"tenantId"`
	Keys      []string    `json:"keys"`
	Workflows []string    `json:"workflows"`
	Search    pgtype.Text `json:"search"`
	Statuses  []string    `json:"statuses"`
	Orderby   interface{} `json:"orderby"`
	Offset    interface{} `json:"offset"`
	Limit     interface{} `json:"limit"`
}

type ListEventsRow struct {
	Event         Event `json:"event"`
	Pendingruns   int64 `json:"pendingruns"`
	Runningruns   int64 `json:"runningruns"`
	Succeededruns int64 `json:"succeededruns"`
	Failedruns    int64 `json:"failedruns"`
}

func (q *Queries) ListEvents(ctx context.Context, db DBTX, arg ListEventsParams) ([]*ListEventsRow, error) {
	rows, err := db.Query(ctx, listEvents,
		arg.TenantId,
		arg.Keys,
		arg.Workflows,
		arg.Search,
		arg.Statuses,
		arg.Orderby,
		arg.Offset,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*ListEventsRow
	for rows.Next() {
		var i ListEventsRow
		if err := rows.Scan(
			&i.Event.ID,
			&i.Event.CreatedAt,
			&i.Event.UpdatedAt,
			&i.Event.DeletedAt,
			&i.Event.Key,
			&i.Event.TenantId,
			&i.Event.ReplayedFromId,
			&i.Event.Data,
			&i.Pendingruns,
			&i.Runningruns,
			&i.Succeededruns,
			&i.Failedruns,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
