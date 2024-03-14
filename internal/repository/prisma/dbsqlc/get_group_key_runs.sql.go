// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.24.0
// source: get_group_key_runs.sql

package dbsqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const assignGetGroupKeyRunToTicker = `-- name: AssignGetGroupKeyRunToTicker :one
WITH selected_ticker AS (
    SELECT
        t."id"
    FROM
        "Ticker" t
    WHERE
        t."lastHeartbeatAt" > NOW() - INTERVAL '6 seconds'
    ORDER BY random()
    LIMIT 1
)
UPDATE
    "GetGroupKeyRun"
SET
    "tickerId" = (
        SELECT "id"
        FROM selected_ticker
    )
WHERE
    "id" = $1::uuid AND
    "tenantId" = $2::uuid AND
    EXISTS (SELECT 1 FROM selected_ticker)
RETURNING "GetGroupKeyRun"."id", "GetGroupKeyRun"."tickerId"
`

type AssignGetGroupKeyRunToTickerParams struct {
	Getgroupkeyrunid pgtype.UUID `json:"getgroupkeyrunid"`
	Tenantid         pgtype.UUID `json:"tenantid"`
}

type AssignGetGroupKeyRunToTickerRow struct {
	ID       pgtype.UUID `json:"id"`
	TickerId pgtype.UUID `json:"tickerId"`
}

func (q *Queries) AssignGetGroupKeyRunToTicker(ctx context.Context, db DBTX, arg AssignGetGroupKeyRunToTickerParams) (*AssignGetGroupKeyRunToTickerRow, error) {
	row := db.QueryRow(ctx, assignGetGroupKeyRunToTicker, arg.Getgroupkeyrunid, arg.Tenantid)
	var i AssignGetGroupKeyRunToTickerRow
	err := row.Scan(&i.ID, &i.TickerId)
	return &i, err
}

const assignGetGroupKeyRunToWorker = `-- name: AssignGetGroupKeyRunToWorker :one
WITH get_group_key_run AS (
    SELECT
        ggr."id",
        ggr."status",
        a."id" AS "actionId"
    FROM
        "GetGroupKeyRun" ggr
    JOIN
        "WorkflowRun" wr ON ggr."workflowRunId" = wr."id"
    JOIN
        "WorkflowVersion" wv ON wr."workflowVersionId" = wv."id"
    JOIN
        "WorkflowConcurrency" wc ON wv."id" = wc."workflowVersionId"
    JOIN
        "Action" a ON wc."getConcurrencyGroupId" = a."id"
    WHERE
        ggr."id" = $1::uuid AND
        ggr."tenantId" = $2::uuid
    FOR UPDATE SKIP LOCKED
), valid_workers AS (
    SELECT
        w."id", w."dispatcherId"
    FROM
        "Worker" w, get_group_key_run
    WHERE
        w."tenantId" = $2::uuid
        AND w."lastHeartbeatAt" > NOW() - INTERVAL '5 seconds'
        AND w."id" IN (
            SELECT "_ActionToWorker"."B"
            FROM "_ActionToWorker"
            INNER JOIN "Action" ON "Action"."id" = "_ActionToWorker"."A"
            WHERE "Action"."tenantId" = $2 AND "Action"."id" = get_group_key_run."actionId"
        )
    ORDER BY random()
), selected_worker AS (
    SELECT "id", "dispatcherId"
    FROM valid_workers
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE
    "GetGroupKeyRun"
SET
    "status" = 'ASSIGNED',
    "workerId" = (
        SELECT "id"
        FROM selected_worker
        LIMIT 1
    ),
    "updatedAt" = CURRENT_TIMESTAMP
WHERE
    "id" = $1::uuid AND
    "tenantId" = $2::uuid AND
    EXISTS (SELECT 1 FROM selected_worker)
RETURNING "GetGroupKeyRun"."id", "GetGroupKeyRun"."workerId", (SELECT "dispatcherId" FROM selected_worker) AS "dispatcherId"
`

type AssignGetGroupKeyRunToWorkerParams struct {
	Getgroupkeyrunid pgtype.UUID `json:"getgroupkeyrunid"`
	Tenantid         pgtype.UUID `json:"tenantid"`
}

type AssignGetGroupKeyRunToWorkerRow struct {
	ID           pgtype.UUID `json:"id"`
	WorkerId     pgtype.UUID `json:"workerId"`
	DispatcherId pgtype.UUID `json:"dispatcherId"`
}

func (q *Queries) AssignGetGroupKeyRunToWorker(ctx context.Context, db DBTX, arg AssignGetGroupKeyRunToWorkerParams) (*AssignGetGroupKeyRunToWorkerRow, error) {
	row := db.QueryRow(ctx, assignGetGroupKeyRunToWorker, arg.Getgroupkeyrunid, arg.Tenantid)
	var i AssignGetGroupKeyRunToWorkerRow
	err := row.Scan(&i.ID, &i.WorkerId, &i.DispatcherId)
	return &i, err
}

const getGroupKeyRunForEngine = `-- name: GetGroupKeyRunForEngine :many
SELECT
    ggr.id, ggr."createdAt", ggr."updatedAt", ggr."deletedAt", ggr."tenantId", ggr."workerId", ggr."tickerId", ggr.status, ggr.input, ggr.output, ggr."requeueAfter", ggr.error, ggr."startedAt", ggr."finishedAt", ggr."timeoutAt", ggr."cancelledAt", ggr."cancelledReason", ggr."cancelledError", ggr."workflowRunId", ggr."scheduleTimeoutAt",
    -- TODO: everything below this line is cacheable and should be moved to a separate query
    wr."id" AS "workflowRunId",
    wv."id" AS "workflowVersionId",
    wv."workflowId" AS "workflowId",
    a."actionId" AS "actionId"
FROM
    "GetGroupKeyRun" ggr
JOIN
    "WorkflowRun" wr ON ggr."workflowRunId" = wr."id"
JOIN
    "WorkflowVersion" wv ON wr."workflowVersionId" = wv."id"
JOIN
    "WorkflowConcurrency" wc ON wv."id" = wc."workflowVersionId"
JOIN
    "Action" a ON wc."getConcurrencyGroupId" = a."id"
WHERE
    ggr."id" = ANY($1::uuid[]) AND
    ggr."tenantId" = $2::uuid
`

type GetGroupKeyRunForEngineParams struct {
	Ids      []pgtype.UUID `json:"ids"`
	Tenantid pgtype.UUID   `json:"tenantid"`
}

type GetGroupKeyRunForEngineRow struct {
	GetGroupKeyRun    GetGroupKeyRun `json:"get_group_key_run"`
	WorkflowRunId     pgtype.UUID    `json:"workflowRunId"`
	WorkflowVersionId pgtype.UUID    `json:"workflowVersionId"`
	WorkflowId        pgtype.UUID    `json:"workflowId"`
	ActionId          string         `json:"actionId"`
}

func (q *Queries) GetGroupKeyRunForEngine(ctx context.Context, db DBTX, arg GetGroupKeyRunForEngineParams) ([]*GetGroupKeyRunForEngineRow, error) {
	rows, err := db.Query(ctx, getGroupKeyRunForEngine, arg.Ids, arg.Tenantid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetGroupKeyRunForEngineRow
	for rows.Next() {
		var i GetGroupKeyRunForEngineRow
		if err := rows.Scan(
			&i.GetGroupKeyRun.ID,
			&i.GetGroupKeyRun.CreatedAt,
			&i.GetGroupKeyRun.UpdatedAt,
			&i.GetGroupKeyRun.DeletedAt,
			&i.GetGroupKeyRun.TenantId,
			&i.GetGroupKeyRun.WorkerId,
			&i.GetGroupKeyRun.TickerId,
			&i.GetGroupKeyRun.Status,
			&i.GetGroupKeyRun.Input,
			&i.GetGroupKeyRun.Output,
			&i.GetGroupKeyRun.RequeueAfter,
			&i.GetGroupKeyRun.Error,
			&i.GetGroupKeyRun.StartedAt,
			&i.GetGroupKeyRun.FinishedAt,
			&i.GetGroupKeyRun.TimeoutAt,
			&i.GetGroupKeyRun.CancelledAt,
			&i.GetGroupKeyRun.CancelledReason,
			&i.GetGroupKeyRun.CancelledError,
			&i.GetGroupKeyRun.WorkflowRunId,
			&i.GetGroupKeyRun.ScheduleTimeoutAt,
			&i.WorkflowRunId,
			&i.WorkflowVersionId,
			&i.WorkflowId,
			&i.ActionId,
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

const listGetGroupKeyRunsToReassign = `-- name: ListGetGroupKeyRunsToReassign :many
SELECT
    ggr.id, ggr."createdAt", ggr."updatedAt", ggr."deletedAt", ggr."tenantId", ggr."workerId", ggr."tickerId", ggr.status, ggr.input, ggr.output, ggr."requeueAfter", ggr.error, ggr."startedAt", ggr."finishedAt", ggr."timeoutAt", ggr."cancelledAt", ggr."cancelledReason", ggr."cancelledError", ggr."workflowRunId", ggr."scheduleTimeoutAt"
FROM
    "GetGroupKeyRun" ggr
LEFT JOIN
    "Worker" w ON ggr."workerId" = w."id"
WHERE
    ggr."tenantId" = $1::uuid
    AND ((
        ggr."status" = 'RUNNING'
        AND w."lastHeartbeatAt" < NOW() - INTERVAL '60 seconds'
    ) OR (
        ggr."status" = 'ASSIGNED'
        AND w."lastHeartbeatAt" < NOW() - INTERVAL '5 seconds'
    ))
ORDER BY
    ggr."createdAt" ASC
`

func (q *Queries) ListGetGroupKeyRunsToReassign(ctx context.Context, db DBTX, tenantid pgtype.UUID) ([]*GetGroupKeyRun, error) {
	rows, err := db.Query(ctx, listGetGroupKeyRunsToReassign, tenantid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetGroupKeyRun
	for rows.Next() {
		var i GetGroupKeyRun
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.DeletedAt,
			&i.TenantId,
			&i.WorkerId,
			&i.TickerId,
			&i.Status,
			&i.Input,
			&i.Output,
			&i.RequeueAfter,
			&i.Error,
			&i.StartedAt,
			&i.FinishedAt,
			&i.TimeoutAt,
			&i.CancelledAt,
			&i.CancelledReason,
			&i.CancelledError,
			&i.WorkflowRunId,
			&i.ScheduleTimeoutAt,
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

const listGetGroupKeyRunsToRequeue = `-- name: ListGetGroupKeyRunsToRequeue :many
SELECT
    ggr.id, ggr."createdAt", ggr."updatedAt", ggr."deletedAt", ggr."tenantId", ggr."workerId", ggr."tickerId", ggr.status, ggr.input, ggr.output, ggr."requeueAfter", ggr.error, ggr."startedAt", ggr."finishedAt", ggr."timeoutAt", ggr."cancelledAt", ggr."cancelledReason", ggr."cancelledError", ggr."workflowRunId", ggr."scheduleTimeoutAt"
FROM
    "GetGroupKeyRun" ggr
LEFT JOIN
    "Worker" w ON ggr."workerId" = w."id"
WHERE
    ggr."tenantId" = $1::uuid
    AND ggr."requeueAfter" < NOW()
    AND ggr."workerId" IS NULL
    AND (ggr."status" = 'PENDING' OR ggr."status" = 'PENDING_ASSIGNMENT')
ORDER BY
    ggr."createdAt" ASC
`

func (q *Queries) ListGetGroupKeyRunsToRequeue(ctx context.Context, db DBTX, tenantid pgtype.UUID) ([]*GetGroupKeyRun, error) {
	rows, err := db.Query(ctx, listGetGroupKeyRunsToRequeue, tenantid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetGroupKeyRun
	for rows.Next() {
		var i GetGroupKeyRun
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.DeletedAt,
			&i.TenantId,
			&i.WorkerId,
			&i.TickerId,
			&i.Status,
			&i.Input,
			&i.Output,
			&i.RequeueAfter,
			&i.Error,
			&i.StartedAt,
			&i.FinishedAt,
			&i.TimeoutAt,
			&i.CancelledAt,
			&i.CancelledReason,
			&i.CancelledError,
			&i.WorkflowRunId,
			&i.ScheduleTimeoutAt,
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

const updateGetGroupKeyRun = `-- name: UpdateGetGroupKeyRun :one
UPDATE
    "GetGroupKeyRun"
SET
    "requeueAfter" = COALESCE($1::timestamp, "requeueAfter"),
    "startedAt" = COALESCE($2::timestamp, "startedAt"),
    "finishedAt" = COALESCE($3::timestamp, "finishedAt"),
    "scheduleTimeoutAt" = COALESCE($4::timestamp, "scheduleTimeoutAt"),
    "status" = CASE 
        -- Final states are final, cannot be updated
        WHEN "status" IN ('SUCCEEDED', 'FAILED', 'CANCELLED') THEN "status"
        ELSE COALESCE($5, "status")
    END,
    "input" = COALESCE($6::jsonb, "input"),
    "output" = COALESCE($7::text, "output"),
    "error" = COALESCE($8::text, "error"),
    "cancelledAt" = COALESCE($9::timestamp, "cancelledAt"),
    "cancelledReason" = COALESCE($10::text, "cancelledReason")
WHERE 
  "id" = $11::uuid AND
  "tenantId" = $12::uuid
RETURNING "GetGroupKeyRun".id, "GetGroupKeyRun"."createdAt", "GetGroupKeyRun"."updatedAt", "GetGroupKeyRun"."deletedAt", "GetGroupKeyRun"."tenantId", "GetGroupKeyRun"."workerId", "GetGroupKeyRun"."tickerId", "GetGroupKeyRun".status, "GetGroupKeyRun".input, "GetGroupKeyRun".output, "GetGroupKeyRun"."requeueAfter", "GetGroupKeyRun".error, "GetGroupKeyRun"."startedAt", "GetGroupKeyRun"."finishedAt", "GetGroupKeyRun"."timeoutAt", "GetGroupKeyRun"."cancelledAt", "GetGroupKeyRun"."cancelledReason", "GetGroupKeyRun"."cancelledError", "GetGroupKeyRun"."workflowRunId", "GetGroupKeyRun"."scheduleTimeoutAt"
`

type UpdateGetGroupKeyRunParams struct {
	RequeueAfter      pgtype.Timestamp  `json:"requeueAfter"`
	StartedAt         pgtype.Timestamp  `json:"startedAt"`
	FinishedAt        pgtype.Timestamp  `json:"finishedAt"`
	ScheduleTimeoutAt pgtype.Timestamp  `json:"scheduleTimeoutAt"`
	Status            NullStepRunStatus `json:"status"`
	Input             []byte            `json:"input"`
	Output            pgtype.Text       `json:"output"`
	Error             pgtype.Text       `json:"error"`
	CancelledAt       pgtype.Timestamp  `json:"cancelledAt"`
	CancelledReason   pgtype.Text       `json:"cancelledReason"`
	ID                pgtype.UUID       `json:"id"`
	Tenantid          pgtype.UUID       `json:"tenantid"`
}

func (q *Queries) UpdateGetGroupKeyRun(ctx context.Context, db DBTX, arg UpdateGetGroupKeyRunParams) (*GetGroupKeyRun, error) {
	row := db.QueryRow(ctx, updateGetGroupKeyRun,
		arg.RequeueAfter,
		arg.StartedAt,
		arg.FinishedAt,
		arg.ScheduleTimeoutAt,
		arg.Status,
		arg.Input,
		arg.Output,
		arg.Error,
		arg.CancelledAt,
		arg.CancelledReason,
		arg.ID,
		arg.Tenantid,
	)
	var i GetGroupKeyRun
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
		&i.TenantId,
		&i.WorkerId,
		&i.TickerId,
		&i.Status,
		&i.Input,
		&i.Output,
		&i.RequeueAfter,
		&i.Error,
		&i.StartedAt,
		&i.FinishedAt,
		&i.TimeoutAt,
		&i.CancelledAt,
		&i.CancelledReason,
		&i.CancelledError,
		&i.WorkflowRunId,
		&i.ScheduleTimeoutAt,
	)
	return &i, err
}
