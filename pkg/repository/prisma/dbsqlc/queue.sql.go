// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.24.0
// source: queue.sql

package dbsqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const bulkQueueItems = `-- name: BulkQueueItems :exec
UPDATE
    "QueueItem" qi
SET
    "isQueued" = false
WHERE
    qi."id" = ANY($1::bigint[])
`

func (q *Queries) BulkQueueItems(ctx context.Context, db DBTX, ids []int64) error {
	_, err := db.Exec(ctx, bulkQueueItems, ids)
	return err
}

const cleanupInternalQueueItems = `-- name: CleanupInternalQueueItems :exec
DELETE FROM "InternalQueueItem"
WHERE "isQueued" = 'f'
AND
    "id" >= $1::bigint
    AND "id" <= $2::bigint
    AND "tenantId" = $3::uuid
`

type CleanupInternalQueueItemsParams struct {
	Minid    int64       `json:"minid"`
	Maxid    int64       `json:"maxid"`
	Tenantid pgtype.UUID `json:"tenantid"`
}

func (q *Queries) CleanupInternalQueueItems(ctx context.Context, db DBTX, arg CleanupInternalQueueItemsParams) error {
	_, err := db.Exec(ctx, cleanupInternalQueueItems, arg.Minid, arg.Maxid, arg.Tenantid)
	return err
}

const cleanupQueueItems = `-- name: CleanupQueueItems :exec
DELETE FROM "QueueItem"
WHERE "isQueued" = 'f'
AND
    "id" >= $1::bigint
    AND "id" <= $2::bigint
    AND "tenantId" = $3::uuid
`

type CleanupQueueItemsParams struct {
	Minid    int64       `json:"minid"`
	Maxid    int64       `json:"maxid"`
	Tenantid pgtype.UUID `json:"tenantid"`
}

func (q *Queries) CleanupQueueItems(ctx context.Context, db DBTX, arg CleanupQueueItemsParams) error {
	_, err := db.Exec(ctx, cleanupQueueItems, arg.Minid, arg.Maxid, arg.Tenantid)
	return err
}

const cleanupTimeoutQueueItems = `-- name: CleanupTimeoutQueueItems :exec
DELETE FROM "TimeoutQueueItem"
WHERE "isQueued" = 'f'
AND
    "id" >= $1::bigint
    AND "id" <= $2::bigint
    AND "tenantId" = $3::uuid
`

type CleanupTimeoutQueueItemsParams struct {
	Minid    int64       `json:"minid"`
	Maxid    int64       `json:"maxid"`
	Tenantid pgtype.UUID `json:"tenantid"`
}

func (q *Queries) CleanupTimeoutQueueItems(ctx context.Context, db DBTX, arg CleanupTimeoutQueueItemsParams) error {
	_, err := db.Exec(ctx, cleanupTimeoutQueueItems, arg.Minid, arg.Maxid, arg.Tenantid)
	return err
}

const createInternalQueueItemsBulk = `-- name: CreateInternalQueueItemsBulk :exec
INSERT INTO
    "InternalQueueItem" (
        "queue",
        "isQueued",
        "data",
        "tenantId",
        "priority"
    )
SELECT
    $1::"InternalQueue",
    true,
    input."data",
    $2::uuid,
    1
FROM (
    SELECT
        unnest($3::json[]) AS "data"
) AS input
ON CONFLICT DO NOTHING
`

type CreateInternalQueueItemsBulkParams struct {
	Queue    InternalQueue `json:"queue"`
	Tenantid pgtype.UUID   `json:"tenantid"`
	Datas    [][]byte      `json:"datas"`
}

func (q *Queries) CreateInternalQueueItemsBulk(ctx context.Context, db DBTX, arg CreateInternalQueueItemsBulkParams) error {
	_, err := db.Exec(ctx, createInternalQueueItemsBulk, arg.Queue, arg.Tenantid, arg.Datas)
	return err
}

const createQueueItem = `-- name: CreateQueueItem :exec
INSERT INTO
    "QueueItem" (
        "stepRunId",
        "stepId",
        "actionId",
        "scheduleTimeoutAt",
        "stepTimeout",
        "priority",
        "isQueued",
        "tenantId",
        "queue",
        "sticky",
        "desiredWorkerId"
    )
VALUES
    (
        $1::uuid,
        $2::uuid,
        $3::text,
        $4::timestamp,
        $5::text,
        COALESCE($6::integer, 1),
        true,
        $7::uuid,
        $8,
        $9::"StickyStrategy",
        $10::uuid
    )
`

type CreateQueueItemParams struct {
	StepRunId         pgtype.UUID        `json:"stepRunId"`
	StepId            pgtype.UUID        `json:"stepId"`
	ActionId          pgtype.Text        `json:"actionId"`
	ScheduleTimeoutAt pgtype.Timestamp   `json:"scheduleTimeoutAt"`
	StepTimeout       pgtype.Text        `json:"stepTimeout"`
	Priority          pgtype.Int4        `json:"priority"`
	Tenantid          pgtype.UUID        `json:"tenantid"`
	Queue             string             `json:"queue"`
	Sticky            NullStickyStrategy `json:"sticky"`
	DesiredWorkerId   pgtype.UUID        `json:"desiredWorkerId"`
}

func (q *Queries) CreateQueueItem(ctx context.Context, db DBTX, arg CreateQueueItemParams) error {
	_, err := db.Exec(ctx, createQueueItem,
		arg.StepRunId,
		arg.StepId,
		arg.ActionId,
		arg.ScheduleTimeoutAt,
		arg.StepTimeout,
		arg.Priority,
		arg.Tenantid,
		arg.Queue,
		arg.Sticky,
		arg.DesiredWorkerId,
	)
	return err
}

const createTimeoutQueueItem = `-- name: CreateTimeoutQueueItem :exec
INSERT INTO
    "InternalQueueItem" (
        "stepRunId",
        "retryCount",
        "timeoutAt",
        "tenantId",
        "isQueued"
    )
SELECT
    $1::uuid,
    $2::integer,
    $3::timestamp,
    $4::uuid,
    true
ON CONFLICT DO NOTHING
`

type CreateTimeoutQueueItemParams struct {
	Steprunid  pgtype.UUID      `json:"steprunid"`
	Retrycount int32            `json:"retrycount"`
	Timeoutat  pgtype.Timestamp `json:"timeoutat"`
	Tenantid   pgtype.UUID      `json:"tenantid"`
}

func (q *Queries) CreateTimeoutQueueItem(ctx context.Context, db DBTX, arg CreateTimeoutQueueItemParams) error {
	_, err := db.Exec(ctx, createTimeoutQueueItem,
		arg.Steprunid,
		arg.Retrycount,
		arg.Timeoutat,
		arg.Tenantid,
	)
	return err
}

const createUniqueInternalQueueItemsBulk = `-- name: CreateUniqueInternalQueueItemsBulk :exec
INSERT INTO
    "InternalQueueItem" (
        "queue",
        "isQueued",
        "data",
        "tenantId",
        "priority",
        "uniqueKey"
    )
SELECT
    $1::"InternalQueue",
    true,
    input."data",
    $2::uuid,
    1,
    input."uniqueKey"
FROM (
    SELECT
        unnest($3::json[]) AS "data",
        unnest($4::text[]) AS "uniqueKey"
) AS input
ON CONFLICT DO NOTHING
`

type CreateUniqueInternalQueueItemsBulkParams struct {
	Queue      InternalQueue `json:"queue"`
	Tenantid   pgtype.UUID   `json:"tenantid"`
	Datas      [][]byte      `json:"datas"`
	Uniquekeys []string      `json:"uniquekeys"`
}

func (q *Queries) CreateUniqueInternalQueueItemsBulk(ctx context.Context, db DBTX, arg CreateUniqueInternalQueueItemsBulkParams) error {
	_, err := db.Exec(ctx, createUniqueInternalQueueItemsBulk,
		arg.Queue,
		arg.Tenantid,
		arg.Datas,
		arg.Uniquekeys,
	)
	return err
}

const getMinMaxProcessedInternalQueueItems = `-- name: GetMinMaxProcessedInternalQueueItems :one
SELECT
    COALESCE(MIN("id"), 0)::bigint AS "minId",
    COALESCE(MAX("id"), 0)::bigint AS "maxId"
FROM
    "InternalQueueItem"
WHERE
    "isQueued" = 'f'
    AND "tenantId" = $1::uuid
`

type GetMinMaxProcessedInternalQueueItemsRow struct {
	MinId int64 `json:"minId"`
	MaxId int64 `json:"maxId"`
}

func (q *Queries) GetMinMaxProcessedInternalQueueItems(ctx context.Context, db DBTX, tenantid pgtype.UUID) (*GetMinMaxProcessedInternalQueueItemsRow, error) {
	row := db.QueryRow(ctx, getMinMaxProcessedInternalQueueItems, tenantid)
	var i GetMinMaxProcessedInternalQueueItemsRow
	err := row.Scan(&i.MinId, &i.MaxId)
	return &i, err
}

const getMinMaxProcessedQueueItems = `-- name: GetMinMaxProcessedQueueItems :one
SELECT
    COALESCE(MIN("id"), 0)::bigint AS "minId",
    COALESCE(MAX("id"), 0)::bigint AS "maxId"
FROM
    "QueueItem"
WHERE
    "isQueued" = 'f'
    AND "tenantId" = $1::uuid
`

type GetMinMaxProcessedQueueItemsRow struct {
	MinId int64 `json:"minId"`
	MaxId int64 `json:"maxId"`
}

func (q *Queries) GetMinMaxProcessedQueueItems(ctx context.Context, db DBTX, tenantid pgtype.UUID) (*GetMinMaxProcessedQueueItemsRow, error) {
	row := db.QueryRow(ctx, getMinMaxProcessedQueueItems, tenantid)
	var i GetMinMaxProcessedQueueItemsRow
	err := row.Scan(&i.MinId, &i.MaxId)
	return &i, err
}

const getMinMaxProcessedTimeoutQueueItems = `-- name: GetMinMaxProcessedTimeoutQueueItems :one
SELECT
    COALESCE(MIN("id"), 0)::bigint AS "minId",
    COALESCE(MAX("id"), 0)::bigint AS "maxId"
FROM
    "TimeoutQueueItem"
WHERE
    "isQueued" = 'f'
    AND "tenantId" = $1::uuid
`

type GetMinMaxProcessedTimeoutQueueItemsRow struct {
	MinId int64 `json:"minId"`
	MaxId int64 `json:"maxId"`
}

func (q *Queries) GetMinMaxProcessedTimeoutQueueItems(ctx context.Context, db DBTX, tenantid pgtype.UUID) (*GetMinMaxProcessedTimeoutQueueItemsRow, error) {
	row := db.QueryRow(ctx, getMinMaxProcessedTimeoutQueueItems, tenantid)
	var i GetMinMaxProcessedTimeoutQueueItemsRow
	err := row.Scan(&i.MinId, &i.MaxId)
	return &i, err
}

const getQueuedCounts = `-- name: GetQueuedCounts :many
SELECT
    "queue",
    COUNT(*) AS "count"
FROM
    "QueueItem" qi
WHERE
    qi."isQueued" = true
    AND qi."tenantId" = $1::uuid
GROUP BY
    qi."queue"
`

type GetQueuedCountsRow struct {
	Queue string `json:"queue"`
	Count int64  `json:"count"`
}

func (q *Queries) GetQueuedCounts(ctx context.Context, db DBTX, tenantid pgtype.UUID) ([]*GetQueuedCountsRow, error) {
	rows, err := db.Query(ctx, getQueuedCounts, tenantid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*GetQueuedCountsRow
	for rows.Next() {
		var i GetQueuedCountsRow
		if err := rows.Scan(&i.Queue, &i.Count); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listAvailableSlotsForWorkers = `-- name: ListAvailableSlotsForWorkers :many
WITH worker_max_runs AS (
    SELECT
        "id",
        "maxRuns"
    FROM
        "Worker"
    WHERE
        "tenantId" = $1::uuid
        AND "id" = ANY($2::uuid[])
), worker_filled_slots AS (
    SELECT
        "workerId",
        COUNT("stepRunId") AS "filledSlots"
    FROM
        "SemaphoreQueueItem"
    WHERE
        "tenantId" = $1::uuid
        AND "workerId" = ANY($2::uuid[])
    GROUP BY
        "workerId"
)
SELECT
    wmr."id",
    wmr."maxRuns" - COALESCE(wfs."filledSlots", 0) AS "availableSlots"
FROM
    worker_max_runs wmr
LEFT JOIN
    worker_filled_slots wfs ON wmr."id" = wfs."workerId"
`

type ListAvailableSlotsForWorkersParams struct {
	Tenantid  pgtype.UUID   `json:"tenantid"`
	Workerids []pgtype.UUID `json:"workerids"`
}

type ListAvailableSlotsForWorkersRow struct {
	ID             pgtype.UUID `json:"id"`
	AvailableSlots int32       `json:"availableSlots"`
}

// subtract the filled slots from the max runs to get the available slots
func (q *Queries) ListAvailableSlotsForWorkers(ctx context.Context, db DBTX, arg ListAvailableSlotsForWorkersParams) ([]*ListAvailableSlotsForWorkersRow, error) {
	rows, err := db.Query(ctx, listAvailableSlotsForWorkers, arg.Tenantid, arg.Workerids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*ListAvailableSlotsForWorkersRow
	for rows.Next() {
		var i ListAvailableSlotsForWorkersRow
		if err := rows.Scan(&i.ID, &i.AvailableSlots); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listInternalQueueItems = `-- name: ListInternalQueueItems :many
SELECT
    id, queue, "isQueued", data, "tenantId", priority, "uniqueKey"
FROM
    "InternalQueueItem" qi
WHERE
    qi."isQueued" = true
    AND qi."tenantId" = $1::uuid
    AND qi."queue" = $2::"InternalQueue"
    AND (
        $3::bigint IS NULL OR
        qi."id" >= $3::bigint
    )
    -- Added to ensure that the index is used
    AND qi."priority" >= 1 AND qi."priority" <= 4
ORDER BY
    qi."priority" DESC,
    qi."id" ASC
LIMIT
    COALESCE($4::integer, 100)
FOR UPDATE SKIP LOCKED
`

type ListInternalQueueItemsParams struct {
	Tenantid pgtype.UUID   `json:"tenantid"`
	Queue    InternalQueue `json:"queue"`
	GtId     pgtype.Int8   `json:"gtId"`
	Limit    pgtype.Int4   `json:"limit"`
}

func (q *Queries) ListInternalQueueItems(ctx context.Context, db DBTX, arg ListInternalQueueItemsParams) ([]*InternalQueueItem, error) {
	rows, err := db.Query(ctx, listInternalQueueItems,
		arg.Tenantid,
		arg.Queue,
		arg.GtId,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*InternalQueueItem
	for rows.Next() {
		var i InternalQueueItem
		if err := rows.Scan(
			&i.ID,
			&i.Queue,
			&i.IsQueued,
			&i.Data,
			&i.TenantId,
			&i.Priority,
			&i.UniqueKey,
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

const listQueues = `-- name: ListQueues :many
SELECT
    id, "tenantId", name
FROM
    "Queue"
WHERE
    "tenantId" = $1::uuid
`

func (q *Queries) ListQueues(ctx context.Context, db DBTX, tenantid pgtype.UUID) ([]*Queue, error) {
	rows, err := db.Query(ctx, listQueues, tenantid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*Queue
	for rows.Next() {
		var i Queue
		if err := rows.Scan(&i.ID, &i.TenantId, &i.Name); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const markInternalQueueItemsProcessed = `-- name: MarkInternalQueueItemsProcessed :exec
UPDATE
    "InternalQueueItem" qi
SET
    "isQueued" = false
WHERE
    qi."id" = ANY($1::bigint[])
`

func (q *Queries) MarkInternalQueueItemsProcessed(ctx context.Context, db DBTX, ids []int64) error {
	_, err := db.Exec(ctx, markInternalQueueItemsProcessed, ids)
	return err
}

const popTimeoutQueueItems = `-- name: PopTimeoutQueueItems :many
WITH qis AS (
    SELECT
        "id",
        "stepRunId"
    FROM
        "TimeoutQueueItem"
    WHERE
        "isQueued" = true
        AND "tenantId" = $1::uuid
        AND "timeoutAt" <= NOW()
    ORDER BY
        "timeoutAt" ASC
    LIMIT
        COALESCE($2::integer, 100)
    FOR UPDATE SKIP LOCKED
)
UPDATE
    "TimeoutQueueItem" qi
SET
    "isQueued" = false
FROM
    qis
WHERE
    qi."id" = qis."id"
RETURNING
    qis."stepRunId" AS "stepRunId"
`

type PopTimeoutQueueItemsParams struct {
	Tenantid pgtype.UUID `json:"tenantid"`
	Limit    pgtype.Int4 `json:"limit"`
}

func (q *Queries) PopTimeoutQueueItems(ctx context.Context, db DBTX, arg PopTimeoutQueueItemsParams) ([]pgtype.UUID, error) {
	rows, err := db.Query(ctx, popTimeoutQueueItems, arg.Tenantid, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []pgtype.UUID
	for rows.Next() {
		var stepRunId pgtype.UUID
		if err := rows.Scan(&stepRunId); err != nil {
			return nil, err
		}
		items = append(items, stepRunId)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const removeTimeoutQueueItem = `-- name: RemoveTimeoutQueueItem :exec
DELETE FROM
    "TimeoutQueueItem"
WHERE
    "stepRunId" = $1::uuid
    AND "retryCount" = $2::integer
`

type RemoveTimeoutQueueItemParams struct {
	Steprunid  pgtype.UUID `json:"steprunid"`
	Retrycount int32       `json:"retrycount"`
}

func (q *Queries) RemoveTimeoutQueueItem(ctx context.Context, db DBTX, arg RemoveTimeoutQueueItemParams) error {
	_, err := db.Exec(ctx, removeTimeoutQueueItem, arg.Steprunid, arg.Retrycount)
	return err
}

const upsertQueue = `-- name: UpsertQueue :exec
INSERT INTO
    "Queue" (
        "tenantId",
        "name"
    )
VALUES
    (
        $1::uuid,
        $2::text
    )
ON CONFLICT ("tenantId", "name") DO NOTHING
`

type UpsertQueueParams struct {
	Tenantid pgtype.UUID `json:"tenantid"`
	Name     string      `json:"name"`
}

func (q *Queries) UpsertQueue(ctx context.Context, db DBTX, arg UpsertQueueParams) error {
	_, err := db.Exec(ctx, upsertQueue, arg.Tenantid, arg.Name)
	return err
}
