// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.24.0
// source: tickers.sql

package dbsqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createTicker = `-- name: CreateTicker :one
INSERT INTO
    "Ticker" ("id", "lastHeartbeatAt", "isActive")
VALUES
    ($1::uuid, CURRENT_TIMESTAMP, 't')
RETURNING id, "createdAt", "updatedAt", "lastHeartbeatAt", "isActive"
`

func (q *Queries) CreateTicker(ctx context.Context, db DBTX, id pgtype.UUID) (*Ticker, error) {
	row := db.QueryRow(ctx, createTicker, id)
	var i Ticker
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.LastHeartbeatAt,
		&i.IsActive,
	)
	return &i, err
}

const deactivateTicker = `-- name: DeactivateTicker :one
UPDATE
    "Ticker" t
SET
    "isActive" = false
WHERE
    "id" = $1::uuid
RETURNING id, "createdAt", "updatedAt", "lastHeartbeatAt", "isActive"
`

func (q *Queries) DeactivateTicker(ctx context.Context, db DBTX, id pgtype.UUID) (*Ticker, error) {
	row := db.QueryRow(ctx, deactivateTicker, id)
	var i Ticker
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.LastHeartbeatAt,
		&i.IsActive,
	)
	return &i, err
}

const listActiveTickers = `-- name: ListActiveTickers :many
SELECT
    tickers.id, tickers."createdAt", tickers."updatedAt", tickers."lastHeartbeatAt", tickers."isActive"
FROM "Ticker" as tickers
WHERE
    -- last heartbeat greater than 15 seconds
    "lastHeartbeatAt" > NOW () - INTERVAL '15 seconds'
    -- active
    AND "isActive" = true
`

type ListActiveTickersRow struct {
	Ticker Ticker `json:"ticker"`
}

func (q *Queries) ListActiveTickers(ctx context.Context, db DBTX) ([]*ListActiveTickersRow, error) {
	rows, err := db.Query(ctx, listActiveTickers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*ListActiveTickersRow
	for rows.Next() {
		var i ListActiveTickersRow
		if err := rows.Scan(
			&i.Ticker.ID,
			&i.Ticker.CreatedAt,
			&i.Ticker.UpdatedAt,
			&i.Ticker.LastHeartbeatAt,
			&i.Ticker.IsActive,
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

const listNewlyStaleTickers = `-- name: ListNewlyStaleTickers :many
SELECT
    tickers.id, tickers."createdAt", tickers."updatedAt", tickers."lastHeartbeatAt", tickers."isActive"
FROM "Ticker" as tickers
WHERE
    -- last heartbeat older than 15 seconds
    "lastHeartbeatAt" < NOW () - INTERVAL '15 seconds'
    -- active
    AND "isActive" = true
`

type ListNewlyStaleTickersRow struct {
	Ticker Ticker `json:"ticker"`
}

func (q *Queries) ListNewlyStaleTickers(ctx context.Context, db DBTX) ([]*ListNewlyStaleTickersRow, error) {
	rows, err := db.Query(ctx, listNewlyStaleTickers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*ListNewlyStaleTickersRow
	for rows.Next() {
		var i ListNewlyStaleTickersRow
		if err := rows.Scan(
			&i.Ticker.ID,
			&i.Ticker.CreatedAt,
			&i.Ticker.UpdatedAt,
			&i.Ticker.LastHeartbeatAt,
			&i.Ticker.IsActive,
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

const listTickers = `-- name: ListTickers :many
SELECT
    id, "createdAt", "updatedAt", "lastHeartbeatAt", "isActive"
FROM
    "Ticker" as tickers
WHERE
    (
        $1::boolean IS NULL OR
        "isActive" = $1::boolean
    )
    AND
    (
        $2::timestamp IS NULL OR
        tickers."lastHeartbeatAt" > $2::timestamp
    )
`

type ListTickersParams struct {
	IsActive           bool             `json:"isActive"`
	LastHeartbeatAfter pgtype.Timestamp `json:"lastHeartbeatAfter"`
}

func (q *Queries) ListTickers(ctx context.Context, db DBTX, arg ListTickersParams) ([]*Ticker, error) {
	rows, err := db.Query(ctx, listTickers, arg.IsActive, arg.LastHeartbeatAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*Ticker
	for rows.Next() {
		var i Ticker
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.LastHeartbeatAt,
			&i.IsActive,
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

const pollCronSchedules = `-- name: PollCronSchedules :many
WITH latest_workflow_versions AS (
    SELECT
        "workflowId",
        MAX("order") as max_order
    FROM
        "WorkflowVersion"
    WHERE
        "deletedAt" IS NULL
    GROUP BY "workflowId"
),
active_cron_schedules AS (
    SELECT
        cronSchedule."parentId",
        versions."id" AS "workflowVersionId",
        triggers."tenantId" AS "tenantId"
    FROM
        "WorkflowTriggerCronRef" as cronSchedule
    JOIN
        "WorkflowTriggers" as triggers ON triggers."id" = cronSchedule."parentId"
    JOIN
        "WorkflowVersion" as versions ON versions."id" = triggers."workflowVersionId"
    JOIN
        latest_workflow_versions l ON versions."workflowId" = l."workflowId" AND versions."order" = l.max_order
    WHERE
        "enabled" = TRUE
        AND versions."deletedAt" IS NULL
        AND (
            "tickerId" IS NULL
            OR NOT EXISTS (
                SELECT 1 FROM "Ticker" WHERE "id" = cronSchedule."tickerId" AND "isActive" = true AND "lastHeartbeatAt" >= NOW() - INTERVAL '10 seconds'
            )
            OR "tickerId" = $1::uuid
        )
    FOR UPDATE SKIP LOCKED
)
UPDATE
    "WorkflowTriggerCronRef" as cronSchedules
SET
    "tickerId" = $1::uuid
FROM
    active_cron_schedules
WHERE
    cronSchedules."parentId" = active_cron_schedules."parentId"
RETURNING cronschedules."parentId", cronschedules.cron, cronschedules."tickerId", cronschedules.input, cronschedules.enabled, cronschedules."additionalMetadata", cronschedules."createdAt", cronschedules."deletedAt", cronschedules."updatedAt", cronschedules.name, cronschedules.id, cronschedules.method, active_cron_schedules."workflowVersionId", active_cron_schedules."tenantId"
`

type PollCronSchedulesRow struct {
	ParentId           pgtype.UUID                   `json:"parentId"`
	Cron               string                        `json:"cron"`
	TickerId           pgtype.UUID                   `json:"tickerId"`
	Input              []byte                        `json:"input"`
	Enabled            bool                          `json:"enabled"`
	AdditionalMetadata []byte                        `json:"additionalMetadata"`
	CreatedAt          pgtype.Timestamp              `json:"createdAt"`
	DeletedAt          pgtype.Timestamp              `json:"deletedAt"`
	UpdatedAt          pgtype.Timestamp              `json:"updatedAt"`
	Name               pgtype.Text                   `json:"name"`
	ID                 pgtype.UUID                   `json:"id"`
	Method             WorkflowTriggerCronRefMethods `json:"method"`
	WorkflowVersionId  pgtype.UUID                   `json:"workflowVersionId"`
	TenantId           pgtype.UUID                   `json:"tenantId"`
}

func (q *Queries) PollCronSchedules(ctx context.Context, db DBTX, tickerid pgtype.UUID) ([]*PollCronSchedulesRow, error) {
	rows, err := db.Query(ctx, pollCronSchedules, tickerid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*PollCronSchedulesRow
	for rows.Next() {
		var i PollCronSchedulesRow
		if err := rows.Scan(
			&i.ParentId,
			&i.Cron,
			&i.TickerId,
			&i.Input,
			&i.Enabled,
			&i.AdditionalMetadata,
			&i.CreatedAt,
			&i.DeletedAt,
			&i.UpdatedAt,
			&i.Name,
			&i.ID,
			&i.Method,
			&i.WorkflowVersionId,
			&i.TenantId,
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

const pollExpiringTokens = `-- name: PollExpiringTokens :many
WITH expiring_tokens AS (
    SELECT
        t0."id", t0."name", t0."expiresAt"
    FROM
        "APIToken" as t0
    WHERE
        t0."revoked" = false
        AND t0."expiresAt" <= NOW() + INTERVAL '7 days'
        AND t0."expiresAt" >= NOW()
        AND (
            t0."nextAlertAt" IS NULL OR
            t0."nextAlertAt" <= NOW()
        )
    FOR UPDATE SKIP LOCKED
    LIMIT 100
)
UPDATE
    "APIToken" as t1
SET
    "nextAlertAt" = NOW() + INTERVAL '1 day'
FROM
    expiring_tokens
WHERE
    t1."id" = expiring_tokens."id"
RETURNING
    t1."id",
    t1."name",
    t1."tenantId",
    t1."expiresAt"
`

type PollExpiringTokensRow struct {
	ID        pgtype.UUID      `json:"id"`
	Name      pgtype.Text      `json:"name"`
	TenantId  pgtype.UUID      `json:"tenantId"`
	ExpiresAt pgtype.Timestamp `json:"expiresAt"`
}

func (q *Queries) PollExpiringTokens(ctx context.Context, db DBTX) ([]*PollExpiringTokensRow, error) {
	rows, err := db.Query(ctx, pollExpiringTokens)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*PollExpiringTokensRow
	for rows.Next() {
		var i PollExpiringTokensRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.TenantId,
			&i.ExpiresAt,
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

const pollGetGroupKeyRuns = `-- name: PollGetGroupKeyRuns :many
WITH getGroupKeyRunsToTimeout AS (
    SELECT
        getGroupKeyRun."id"
    FROM
        "GetGroupKeyRun" as getGroupKeyRun
    WHERE
        "status" = ANY(ARRAY['RUNNING', 'ASSIGNED']::"StepRunStatus"[])
        AND "timeoutAt" < NOW()
        AND "deletedAt" IS NULL
        AND (
            NOT EXISTS (
                SELECT 1 FROM "Ticker" WHERE "id" = getGroupKeyRun."tickerId" AND "isActive" = true AND "lastHeartbeatAt" >= NOW() - INTERVAL '10 seconds'
            )
            OR "tickerId" IS NULL
        )
    FOR UPDATE SKIP LOCKED
)
UPDATE
    "GetGroupKeyRun" as getGroupKeyRuns
SET
    "tickerId" = $1::uuid
FROM
    getGroupKeyRunsToTimeout
WHERE
    getGroupKeyRuns."id" = getGroupKeyRunsToTimeout."id"
RETURNING getgroupkeyruns.id, getgroupkeyruns."createdAt", getgroupkeyruns."updatedAt", getgroupkeyruns."deletedAt", getgroupkeyruns."tenantId", getgroupkeyruns."workerId", getgroupkeyruns."tickerId", getgroupkeyruns.status, getgroupkeyruns.input, getgroupkeyruns.output, getgroupkeyruns."requeueAfter", getgroupkeyruns.error, getgroupkeyruns."startedAt", getgroupkeyruns."finishedAt", getgroupkeyruns."timeoutAt", getgroupkeyruns."cancelledAt", getgroupkeyruns."cancelledReason", getgroupkeyruns."cancelledError", getgroupkeyruns."workflowRunId", getgroupkeyruns."scheduleTimeoutAt"
`

func (q *Queries) PollGetGroupKeyRuns(ctx context.Context, db DBTX, tickerid pgtype.UUID) ([]*GetGroupKeyRun, error) {
	rows, err := db.Query(ctx, pollGetGroupKeyRuns, tickerid)
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

const pollScheduledWorkflows = `-- name: PollScheduledWorkflows :many
WITH latest_workflow_versions AS (
    SELECT
        DISTINCT ON("workflowId")
        "workflowId",
        "id"
    FROM
        "WorkflowVersion"
    WHERE
        "deletedAt" IS NULL
    ORDER BY "workflowId", "order" DESC
), not_run_scheduled_workflows AS (
    SELECT
        scheduledWorkflow."id",
        latestVersions."id" AS "workflowVersionId",
        workflow."tenantId" AS "tenantId",
        scheduledWorkflow."additionalMetadata" AS "additionalMetadata"
    FROM
        "WorkflowTriggerScheduledRef" AS scheduledWorkflow
    JOIN
        "WorkflowVersion" AS versions ON versions."id" = scheduledWorkflow."parentId"
    JOIN
        "Workflow" AS workflow ON workflow."id" = versions."workflowId"
    JOIN
        latest_workflow_versions AS latestVersions ON latestVersions."workflowId" = workflow."id"
    LEFT JOIN
        "WorkflowRunTriggeredBy" AS runTriggeredBy ON runTriggeredBy."scheduledId" = scheduledWorkflow."id"
    WHERE
        "triggerAt" <= NOW() + INTERVAL '5 seconds'
        AND runTriggeredBy IS NULL
        AND versions."deletedAt" IS NULL
        AND workflow."deletedAt" IS NULL
        AND (
            "tickerId" IS NULL
            OR NOT EXISTS (
                SELECT 1 FROM "Ticker" WHERE "id" = scheduledWorkflow."tickerId" AND "isActive" = true AND "lastHeartbeatAt" >= NOW() - INTERVAL '10 seconds'
            )
            OR "tickerId" = $1::uuid
        )
),
active_scheduled_workflows AS (
    SELECT
        id, "workflowVersionId", "tenantId", "additionalMetadata"
    FROM
        not_run_scheduled_workflows
    FOR UPDATE SKIP LOCKED
)
UPDATE
    "WorkflowTriggerScheduledRef" as scheduledWorkflows
SET
    "tickerId" = $1::uuid
FROM
    active_scheduled_workflows
WHERE
    scheduledWorkflows."id" = active_scheduled_workflows."id"
RETURNING scheduledworkflows.id, scheduledworkflows."parentId", scheduledworkflows."triggerAt", scheduledworkflows."tickerId", scheduledworkflows.input, scheduledworkflows."childIndex", scheduledworkflows."childKey", scheduledworkflows."parentStepRunId", scheduledworkflows."parentWorkflowRunId", scheduledworkflows."additionalMetadata", scheduledworkflows."createdAt", scheduledworkflows."deletedAt", scheduledworkflows."updatedAt", scheduledworkflows.method, active_scheduled_workflows."workflowVersionId", active_scheduled_workflows."tenantId"
`

type PollScheduledWorkflowsRow struct {
	ID                  pgtype.UUID                        `json:"id"`
	ParentId            pgtype.UUID                        `json:"parentId"`
	TriggerAt           pgtype.Timestamp                   `json:"triggerAt"`
	TickerId            pgtype.UUID                        `json:"tickerId"`
	Input               []byte                             `json:"input"`
	ChildIndex          pgtype.Int4                        `json:"childIndex"`
	ChildKey            pgtype.Text                        `json:"childKey"`
	ParentStepRunId     pgtype.UUID                        `json:"parentStepRunId"`
	ParentWorkflowRunId pgtype.UUID                        `json:"parentWorkflowRunId"`
	AdditionalMetadata  []byte                             `json:"additionalMetadata"`
	CreatedAt           pgtype.Timestamp                   `json:"createdAt"`
	DeletedAt           pgtype.Timestamp                   `json:"deletedAt"`
	UpdatedAt           pgtype.Timestamp                   `json:"updatedAt"`
	Method              WorkflowTriggerScheduledRefMethods `json:"method"`
	WorkflowVersionId   pgtype.UUID                        `json:"workflowVersionId"`
	TenantId            pgtype.UUID                        `json:"tenantId"`
}

// Finds workflows that are either past their execution time or will be in the next 5 seconds and assigns them
// to a ticker, or finds workflows that were assigned to a ticker that is no longer active
func (q *Queries) PollScheduledWorkflows(ctx context.Context, db DBTX, tickerid pgtype.UUID) ([]*PollScheduledWorkflowsRow, error) {
	rows, err := db.Query(ctx, pollScheduledWorkflows, tickerid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*PollScheduledWorkflowsRow
	for rows.Next() {
		var i PollScheduledWorkflowsRow
		if err := rows.Scan(
			&i.ID,
			&i.ParentId,
			&i.TriggerAt,
			&i.TickerId,
			&i.Input,
			&i.ChildIndex,
			&i.ChildKey,
			&i.ParentStepRunId,
			&i.ParentWorkflowRunId,
			&i.AdditionalMetadata,
			&i.CreatedAt,
			&i.DeletedAt,
			&i.UpdatedAt,
			&i.Method,
			&i.WorkflowVersionId,
			&i.TenantId,
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

const pollTenantAlerts = `-- name: PollTenantAlerts :many
WITH active_tenant_alerts AS (
    SELECT
        alerts.id, alerts."createdAt", alerts."updatedAt", alerts."deletedAt", alerts."tenantId", alerts."maxFrequency", alerts."lastAlertedAt", alerts."tickerId", alerts."enableExpiringTokenAlerts", alerts."enableWorkflowRunFailureAlerts", alerts."enableTenantResourceLimitAlerts"
    FROM
        "TenantAlertingSettings" as alerts
    WHERE
        "lastAlertedAt" IS NULL OR
        "lastAlertedAt" <= NOW() - convert_duration_to_interval(alerts."maxFrequency")
    FOR UPDATE SKIP LOCKED
),
failed_run_count_by_tenant AS (
    SELECT
        workflowRun."tenantId",
        COUNT(*) as "failedWorkflowRunCount"
    FROM
        "WorkflowRun" as workflowRun
    JOIN
        active_tenant_alerts ON active_tenant_alerts."tenantId" = workflowRun."tenantId"
    WHERE
        "status" = 'FAILED'
        AND workflowRun."deletedAt" IS NULL
        AND (
            (
                "lastAlertedAt" IS NULL AND
                workflowRun."finishedAt" >= NOW() - convert_duration_to_interval(active_tenant_alerts."maxFrequency")
            ) OR
            workflowRun."finishedAt" >= "lastAlertedAt"
        )
    GROUP BY workflowRun."tenantId"
)
UPDATE
    "TenantAlertingSettings" as alerts
SET
    "tickerId" = $1::uuid,
    "lastAlertedAt" = NOW()
FROM
    active_tenant_alerts
WHERE
    alerts."id" = active_tenant_alerts."id" AND
    alerts."tenantId" IN (SELECT "tenantId" FROM failed_run_count_by_tenant WHERE "failedWorkflowRunCount" > 0)
RETURNING alerts.id, alerts."createdAt", alerts."updatedAt", alerts."deletedAt", alerts."tenantId", alerts."maxFrequency", alerts."lastAlertedAt", alerts."tickerId", alerts."enableExpiringTokenAlerts", alerts."enableWorkflowRunFailureAlerts", alerts."enableTenantResourceLimitAlerts", active_tenant_alerts."lastAlertedAt" AS "prevLastAlertedAt"
`

type PollTenantAlertsRow struct {
	ID                              pgtype.UUID      `json:"id"`
	CreatedAt                       pgtype.Timestamp `json:"createdAt"`
	UpdatedAt                       pgtype.Timestamp `json:"updatedAt"`
	DeletedAt                       pgtype.Timestamp `json:"deletedAt"`
	TenantId                        pgtype.UUID      `json:"tenantId"`
	MaxFrequency                    string           `json:"maxFrequency"`
	LastAlertedAt                   pgtype.Timestamp `json:"lastAlertedAt"`
	TickerId                        pgtype.UUID      `json:"tickerId"`
	EnableExpiringTokenAlerts       bool             `json:"enableExpiringTokenAlerts"`
	EnableWorkflowRunFailureAlerts  bool             `json:"enableWorkflowRunFailureAlerts"`
	EnableTenantResourceLimitAlerts bool             `json:"enableTenantResourceLimitAlerts"`
	PrevLastAlertedAt               pgtype.Timestamp `json:"prevLastAlertedAt"`
}

// Finds tenant alerts which haven't alerted since their frequency and assigns them to a ticker
func (q *Queries) PollTenantAlerts(ctx context.Context, db DBTX, tickerid pgtype.UUID) ([]*PollTenantAlertsRow, error) {
	rows, err := db.Query(ctx, pollTenantAlerts, tickerid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*PollTenantAlertsRow
	for rows.Next() {
		var i PollTenantAlertsRow
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.DeletedAt,
			&i.TenantId,
			&i.MaxFrequency,
			&i.LastAlertedAt,
			&i.TickerId,
			&i.EnableExpiringTokenAlerts,
			&i.EnableWorkflowRunFailureAlerts,
			&i.EnableTenantResourceLimitAlerts,
			&i.PrevLastAlertedAt,
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

const pollTenantResourceLimitAlerts = `-- name: PollTenantResourceLimitAlerts :many
WITH alerting_resource_limits AS (
    SELECT
        rl."id" AS "resourceLimitId",
        rl."tenantId",
        rl."resource",
        rl."limitValue",
        rl."alarmValue",
        rl."value",
        rl."window",
        rl."lastRefill",
        CASE
            WHEN rl."value" >= rl."limitValue" THEN 'Exhausted'
            WHEN rl."alarmValue" IS NOT NULL AND rl."value" >= rl."alarmValue" THEN 'Alarm'
        END AS "alertType"
    FROM
        "TenantResourceLimit" AS rl
    JOIN
        "TenantAlertingSettings" AS ta
    ON
        ta."tenantId" = rl."tenantId"::uuid
    WHERE
        ta."enableTenantResourceLimitAlerts" = true
        AND (
            (rl."alarmValue" IS NOT NULL AND rl."value" >= rl."alarmValue")
            OR rl."value" >= rl."limitValue"
        )
    FOR UPDATE SKIP LOCKED
),
new_alerts AS (
    SELECT
        arl."resourceLimitId",
        arl."tenantId",
        arl."resource",
        arl."alertType",
        arl."value",
        arl."limitValue" AS "limit",
        EXISTS (
            SELECT 1
            FROM "TenantResourceLimitAlert" AS trla
            WHERE trla."resourceLimitId" = arl."resourceLimitId"
            AND trla."alertType" = arl."alertType"::"TenantResourceLimitAlertType"
            AND trla."createdAt" >= NOW() - arl."window"::INTERVAL
        ) AS "existingAlert"
    FROM
        alerting_resource_limits AS arl
)
INSERT INTO "TenantResourceLimitAlert" (
    "id",
    "createdAt",
    "updatedAt",
    "resourceLimitId",
    "resource",
    "alertType",
    "value",
    "limit",
    "tenantId"
)
SELECT
    gen_random_uuid(),
    NOW(),
    NOW(),
    na."resourceLimitId",
    na."resource",
    na."alertType"::"TenantResourceLimitAlertType",
    na."value",
    na."limit",
    na."tenantId"
FROM
    new_alerts AS na
WHERE
    na."existingAlert" = false
RETURNING id, "createdAt", "updatedAt", "resourceLimitId", "tenantId", resource, "alertType", value, "limit"
`

func (q *Queries) PollTenantResourceLimitAlerts(ctx context.Context, db DBTX) ([]*TenantResourceLimitAlert, error) {
	rows, err := db.Query(ctx, pollTenantResourceLimitAlerts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*TenantResourceLimitAlert
	for rows.Next() {
		var i TenantResourceLimitAlert
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ResourceLimitId,
			&i.TenantId,
			&i.Resource,
			&i.AlertType,
			&i.Value,
			&i.Limit,
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

const pollUnresolvedFailedStepRuns = `-- name: PollUnresolvedFailedStepRuns :many
SELECT
	sr."id",
    sr."tenantId"
FROM "StepRun" sr
JOIN "JobRun" jr on jr."id" = sr."jobRunId"
WHERE
	(
		(sr."status" = 'FAILED' AND jr."status" != 'FAILED')
	OR
		(sr."status" = 'CANCELLED' AND jr."status" != 'CANCELLED')
	)
	AND sr."updatedAt" < CURRENT_TIMESTAMP - INTERVAL '5 seconds'
`

type PollUnresolvedFailedStepRunsRow struct {
	ID       pgtype.UUID `json:"id"`
	TenantId pgtype.UUID `json:"tenantId"`
}

func (q *Queries) PollUnresolvedFailedStepRuns(ctx context.Context, db DBTX) ([]*PollUnresolvedFailedStepRunsRow, error) {
	rows, err := db.Query(ctx, pollUnresolvedFailedStepRuns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*PollUnresolvedFailedStepRunsRow
	for rows.Next() {
		var i PollUnresolvedFailedStepRunsRow
		if err := rows.Scan(&i.ID, &i.TenantId); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setTickersInactive = `-- name: SetTickersInactive :many
UPDATE
    "Ticker" as tickers
SET
    "isActive" = false
WHERE
    "id" = ANY ($1::uuid[])
RETURNING
    tickers.id, tickers."createdAt", tickers."updatedAt", tickers."lastHeartbeatAt", tickers."isActive"
`

type SetTickersInactiveRow struct {
	Ticker Ticker `json:"ticker"`
}

func (q *Queries) SetTickersInactive(ctx context.Context, db DBTX, ids []pgtype.UUID) ([]*SetTickersInactiveRow, error) {
	rows, err := db.Query(ctx, setTickersInactive, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*SetTickersInactiveRow
	for rows.Next() {
		var i SetTickersInactiveRow
		if err := rows.Scan(
			&i.Ticker.ID,
			&i.Ticker.CreatedAt,
			&i.Ticker.UpdatedAt,
			&i.Ticker.LastHeartbeatAt,
			&i.Ticker.IsActive,
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

const updateTicker = `-- name: UpdateTicker :one
UPDATE
    "Ticker" as tickers
SET
    "lastHeartbeatAt" = $1::timestamp
WHERE
    "id" = $2::uuid
RETURNING id, "createdAt", "updatedAt", "lastHeartbeatAt", "isActive"
`

type UpdateTickerParams struct {
	LastHeartbeatAt pgtype.Timestamp `json:"lastHeartbeatAt"`
	ID              pgtype.UUID      `json:"id"`
}

func (q *Queries) UpdateTicker(ctx context.Context, db DBTX, arg UpdateTickerParams) (*Ticker, error) {
	row := db.QueryRow(ctx, updateTicker, arg.LastHeartbeatAt, arg.ID)
	var i Ticker
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.LastHeartbeatAt,
		&i.IsActive,
	)
	return &i, err
}
