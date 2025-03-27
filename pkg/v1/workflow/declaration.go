// Package workflow provides functionality for defining, managing, and executing
// workflows in Hatchet. A workflow is a collection of tasks with defined
// dependencies and execution logic.
package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/client"
	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/hatchet-dev/hatchet/pkg/client/create"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	"github.com/hatchet-dev/hatchet/pkg/client/types"
	"github.com/hatchet-dev/hatchet/pkg/v1/features"
	"github.com/hatchet-dev/hatchet/pkg/v1/task"
	"github.com/hatchet-dev/hatchet/pkg/worker"

	"reflect"

	contracts "github.com/hatchet-dev/hatchet/internal/services/shared/proto/v1"
)

// WrappedTaskFn represents a task function that can be executed by the Hatchet worker.
// It takes a HatchetContext and returns an interface{} result and an error.
type WrappedTaskFn func(ctx worker.HatchetContext) (interface{}, error)

// DurableWrappedTaskFn represents a durable task function that can be executed by the Hatchet worker.
// It takes a DurableHatchetContext and returns an interface{} result and an error.
type DurableWrappedTaskFn func(ctx worker.DurableHatchetContext) (interface{}, error)

// NamedFunction represents a function with its associated action ID
type NamedFunction struct {
	ActionID string
	Fn       WrappedTaskFn
}

// WorkflowBase defines the common interface for all workflow types.
type WorkflowBase interface {
	// Dump converts the workflow declaration into a protobuf request and function mappings.
	// Returns the workflow definition, regular task functions, durable task functions, and the on failure task function.
	Dump() (*contracts.CreateWorkflowVersionRequest, []NamedFunction, []NamedFunction, WrappedTaskFn)
}

type RunOpts struct {
	AdditionalMetadata *map[string]interface{}

	childOpts *client.ChildWorkflowOpts
}

type RunAsChildOpts struct {
	RunOpts
	Sticky *bool
	Key    *string
}

// WorkflowDeclaration represents a workflow with input type I and output type O.
// It provides methods to define tasks, specify dependencies, and execute the workflow.
type WorkflowDeclaration[I, O any] interface {
	WorkflowBase

	// Task registers a task that will be executed as part of the workflow
	Task(opts create.WorkflowTask[I, O], fn func(ctx worker.HatchetContext, input I) (interface{}, error)) *task.TaskDeclaration[I]

	// DurableTask registers a durable task that will be executed as part of the workflow.
	// Durable tasks can be paused and resumed across workflow runs, making them suitable
	// for long-running operations or tasks that require human intervention.
	DurableTask(opts create.WorkflowTask[I, O], fn func(ctx worker.DurableHatchetContext, input I) (interface{}, error)) *task.DurableTaskDeclaration[I]

	// OnFailureTask registers a task that will be executed if the workflow fails.
	OnFailure(opts create.WorkflowOnFailureTask[I, O], fn func(ctx worker.HatchetContext, input I) (interface{}, error)) *task.OnFailureTaskDeclaration[I]

	// Run executes the workflow with the provided input.
	Run(input I, opts ...RunOpts) (*O, error)

	// RunChild executes a child workflow with the provided input.
	RunAsChild(ctx worker.HatchetContext, input I, opts ...RunAsChildOpts) (*O, error)

	// RunNoWait executes the workflow with the provided input without waiting for it to complete.
	// Instead it returns a run ID that can be used to check the status of the workflow.
	RunNoWait(input I, opts ...RunOpts) (*v0Client.Workflow, error)

	// Cron schedules the workflow to run on a regular basis using a cron expression.
	Cron(name string, cronExpr string, input I, opts ...RunOpts) (*rest.CronWorkflows, error)

	// Schedule schedules the workflow to run at a specific time.
	Schedule(triggerAt time.Time, input I, opts ...RunOpts) (*rest.ScheduledWorkflows, error)

	// Get retrieves the current state of the workflow.
	Get() (*rest.Workflow, error)

	// // IsPaused checks if the workflow is currently paused.
	// IsPaused() (bool, error)

	// // Pause pauses the assignment of new workflow runs.
	// Pause() error

	// // Unpause resumes the assignment of workflow runs.
	// Unpause() error

	// Metrics retrieves metrics for this workflow.
	Metrics(opts ...rest.WorkflowGetMetricsParams) (*rest.WorkflowMetrics, error)

	// QueueMetrics retrieves queue metrics for this workflow.
	QueueMetrics(opts ...rest.TenantGetQueueMetricsParams) (*rest.TenantGetQueueMetricsResponse, error)
}

// Define a TaskDeclaration with specific output type
type TaskWithSpecificOutput[I any, T any] struct {
	Name string
	Fn   func(ctx worker.HatchetContext, input I) (*T, error)
}

// workflowDeclarationImpl is the concrete implementation of WorkflowDeclaration.
// It contains all the data and logic needed to define and execute a workflow.
type workflowDeclarationImpl[I any, O any] struct {
	v0        v0Client.Client
	crons     features.CronsClient
	schedules features.SchedulesClient
	metrics   features.MetricsClient
	workflows features.WorkflowsClient

	outputKey *string

	Name           string
	Version        *string
	Description    *string
	OnEvents       []string
	OnCron         []string
	Concurrency    *types.Concurrency
	OnFailureTask  *task.OnFailureTaskDeclaration[I]
	StickyStrategy *types.StickyStrategy

	TaskDefaults *create.TaskDefaults

	tasks        []*task.TaskDeclaration[I]
	durableTasks []*task.DurableTaskDeclaration[I]

	// Store task functions with their specific output types
	taskFuncs        map[string]interface{}
	durableTaskFuncs map[string]interface{}

	// Map to store task output setters
	outputSetters map[string]func(*O, interface{})
}

// NewWorkflowDeclaration creates a new workflow declaration with the specified options and client.
// The workflow will have input type I and output type O.
func NewWorkflowDeclaration[I any, O any](opts create.WorkflowCreateOpts[I], v0 v0Client.Client) WorkflowDeclaration[I, O] {

	api := v0.API()
	tenantId := v0.TenantId()

	crons := features.NewCronsClient(api, &tenantId)
	schedules := features.NewSchedulesClient(api, &tenantId)
	metrics := features.NewMetricsClient(api, &tenantId)
	workflows := features.NewWorkflowsClient(api, &tenantId)

	wf := &workflowDeclarationImpl[I, O]{
		v0:          v0,
		crons:       crons,
		schedules:   schedules,
		metrics:     metrics,
		workflows:   workflows,
		Name:        opts.Name,
		OnEvents:    opts.OnEvents,
		OnCron:      opts.OnCron,
		Concurrency: opts.Concurrency,
		// OnFailureTask:    opts.OnFailureTask, // TODO: add this back in
		StickyStrategy:   opts.StickyStrategy,
		TaskDefaults:     opts.TaskDefaults,
		outputKey:        opts.OutputKey,
		tasks:            []*task.TaskDeclaration[I]{},
		taskFuncs:        make(map[string]interface{}),
		durableTasks:     []*task.DurableTaskDeclaration[I]{},
		durableTaskFuncs: make(map[string]interface{}),
		outputSetters:    make(map[string]func(*O, interface{})),
	}

	if opts.Version != "" {
		wf.Version = &opts.Version
	}

	if opts.Description != "" {
		wf.Description = &opts.Description
	}

	return wf
}

// Task registers a standard (non-durable) task with the workflow
func (w *workflowDeclarationImpl[I, O]) Task(opts create.WorkflowTask[I, O], fn func(ctx worker.HatchetContext, input I) (interface{}, error)) *task.TaskDeclaration[I] {
	name := opts.Name

	// Use reflection to validate the function type
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func ||
		fnType.NumIn() != 2 ||
		fnType.NumOut() != 2 ||
		!fnType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic("Invalid function type for task " + name + ": must be func(I, worker.HatchetContext) (*T, error)")
	}

	// Create a setter function that can set this specific output type to the corresponding field in O
	w.outputSetters[name] = func(result *O, output interface{}) {
		resultValue := reflect.ValueOf(result).Elem()
		field := resultValue.FieldByName(name)

		// If the field isn't found by name, try to find it by JSON tag
		resultType := resultValue.Type()
		for i := 0; i < resultType.NumField(); i++ {
			fieldType := resultType.Field(i)
			jsonTag := fieldType.Tag.Get("json")
			// Extract the name part from the json tag (before any comma)
			if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
				jsonTag = jsonTag[:commaIdx]
			}
			if jsonTag == name || strings.EqualFold(fieldType.Name, name) {
				field = resultValue.Field(i)
				break
			}
		}

		if field.IsValid() && field.CanSet() {
			outputValue := reflect.ValueOf(output).Elem()
			field.Set(outputValue)
		}
	}

	// Create a generic task function that wraps the specific one
	genericFn := func(ctx worker.HatchetContext, input I) (*any, error) {
		// Use reflection to call the specific function
		fnValue := reflect.ValueOf(fn)
		inputs := []reflect.Value{reflect.ValueOf(input), reflect.ValueOf(ctx)}
		results := fnValue.Call(inputs)

		// Handle errors
		if !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}

		// Return the output as any
		output := results[0].Interface()
		return &output, nil
	}

	// Initialize pointers only for non-zero values
	var retryBackoffFactor *float32
	var retryMaxBackoffSeconds *int32
	var executionTimeout *time.Duration
	var scheduleTimeout *time.Duration
	var retries *int32

	if opts.RetryBackoffFactor != 0 {
		retryBackoffFactor = &opts.RetryBackoffFactor
	}
	if opts.RetryMaxBackoffSeconds != 0 {
		retryMaxBackoffSeconds = &opts.RetryMaxBackoffSeconds
	}
	if opts.ExecutionTimeout != 0 {
		executionTimeout = &opts.ExecutionTimeout
	}
	if opts.ScheduleTimeout != 0 {
		scheduleTimeout = &opts.ScheduleTimeout
	}
	if opts.Retries != 0 {
		retries = &opts.Retries
	}

	// Convert parent task declarations to parent task names
	parentNames := make([]string, len(opts.Parents))
	for i, parent := range opts.Parents {
		parentNames[i] = parent.GetName()
	}

	taskDecl := &task.TaskDeclaration[I]{
		Name:     opts.Name,
		Fn:       genericFn,
		Parents:  parentNames,
		WaitFor:  opts.WaitFor,
		SkipIf:   opts.SkipIf,
		CancelIf: opts.CancelIf,
		TaskShared: task.TaskShared{
			ExecutionTimeout:       executionTimeout,
			ScheduleTimeout:        scheduleTimeout,
			Retries:                retries,
			RetryBackoffFactor:     retryBackoffFactor,
			RetryMaxBackoffSeconds: retryMaxBackoffSeconds,
			RateLimits:             opts.RateLimits,
			WorkerLabels:           opts.WorkerLabels,
			Concurrency:            opts.Concurrency,
		},
	}

	w.tasks = append(w.tasks, taskDecl)
	w.taskFuncs[name] = fn

	return taskDecl
}

// DurableTask registers a durable task with the workflow
func (w *workflowDeclarationImpl[I, O]) DurableTask(opts create.WorkflowTask[I, O], fn func(ctx worker.DurableHatchetContext, input I) (interface{}, error)) *task.DurableTaskDeclaration[I] {
	name := opts.Name

	// Use reflection to validate the function type
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func ||
		fnType.NumIn() != 2 ||
		fnType.NumOut() != 2 ||
		!fnType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic("Invalid function type for durable task " + name + ": must be func(I, worker.DurableHatchetContext) (*T, error)")
	}

	// Create a setter function that can set this specific output type to the corresponding field in O
	w.outputSetters[name] = func(result *O, output interface{}) {
		resultValue := reflect.ValueOf(result).Elem()
		field := resultValue.FieldByName(name)

		if field.IsValid() && field.CanSet() {
			outputValue := reflect.ValueOf(output).Elem()
			field.Set(outputValue)
		}
	}

	// Create a generic task function that wraps the specific one
	genericFn := func(ctx worker.DurableHatchetContext, input I) (*any, error) {
		// Use reflection to call the specific function
		fnValue := reflect.ValueOf(fn)
		inputs := []reflect.Value{reflect.ValueOf(input), reflect.ValueOf(ctx)}
		results := fnValue.Call(inputs)

		// Handle errors
		if !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}

		// Return the output as any
		output := results[0].Interface()
		return &output, nil
	}

	// Initialize pointers only for non-zero values
	var retryBackoffFactor *float32
	var retryMaxBackoffSeconds *int32
	var executionTimeout *time.Duration
	var scheduleTimeout *time.Duration
	var retries *int32

	if opts.RetryBackoffFactor != 0 {
		retryBackoffFactor = &opts.RetryBackoffFactor
	}
	if opts.RetryMaxBackoffSeconds != 0 {
		retryMaxBackoffSeconds = &opts.RetryMaxBackoffSeconds
	}
	if opts.ExecutionTimeout != 0 {
		executionTimeout = &opts.ExecutionTimeout
	}
	if opts.ScheduleTimeout != 0 {
		scheduleTimeout = &opts.ScheduleTimeout
	}
	if opts.Retries != 0 {
		retries = &opts.Retries
	}

	// Convert parent task declarations to parent task names
	parentNames := make([]string, len(opts.Parents))
	for i, parent := range opts.Parents {
		parentNames[i] = parent.GetName()
	}

	taskDecl := &task.DurableTaskDeclaration[I]{
		Name:     opts.Name,
		Fn:       genericFn,
		Parents:  parentNames,
		WaitFor:  opts.WaitFor,
		SkipIf:   opts.SkipIf,
		CancelIf: opts.CancelIf,
		TaskShared: task.TaskShared{
			ExecutionTimeout:       executionTimeout,
			ScheduleTimeout:        scheduleTimeout,
			Retries:                retries,
			RetryBackoffFactor:     retryBackoffFactor,
			RetryMaxBackoffSeconds: retryMaxBackoffSeconds,
			RateLimits:             opts.RateLimits,
			WorkerLabels:           opts.WorkerLabels,
			Concurrency:            opts.Concurrency,
		},
	}

	w.durableTasks = append(w.durableTasks, taskDecl)
	w.durableTaskFuncs[name] = fn

	return taskDecl
}

// OnFailureTask registers a task that will be executed if the workflow fails.
func (w *workflowDeclarationImpl[I, O]) OnFailure(opts create.WorkflowOnFailureTask[I, O], fn func(ctx worker.HatchetContext, input I) (interface{}, error)) *task.OnFailureTaskDeclaration[I] {

	// Use reflection to validate the function type
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func ||
		fnType.NumIn() != 2 ||
		fnType.NumOut() != 2 ||
		!fnType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic("Invalid function type for on failure task: must be func(I, worker.HatchetContext) (*T, error)")
	}

	// Create a generic task function that wraps the specific one
	genericFn := func(ctx worker.HatchetContext, input I) (*any, error) {
		// Use reflection to call the specific function
		fnValue := reflect.ValueOf(fn)
		inputs := []reflect.Value{reflect.ValueOf(input), reflect.ValueOf(ctx)}
		results := fnValue.Call(inputs)

		// Handle errors
		if !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}

		// Return the output as any
		output := results[0].Interface()
		return &output, nil
	}

	// Initialize pointers only for non-zero values
	var retryBackoffFactor *float32
	var retryMaxBackoffSeconds *int32
	var executionTimeout *time.Duration
	var scheduleTimeout *time.Duration
	var retries *int32

	if opts.RetryBackoffFactor != 0 {
		retryBackoffFactor = &opts.RetryBackoffFactor
	}
	if opts.RetryMaxBackoffSeconds != 0 {
		retryMaxBackoffSeconds = &opts.RetryMaxBackoffSeconds
	}
	if opts.ExecutionTimeout != 0 {
		executionTimeout = &opts.ExecutionTimeout
	}
	if opts.ScheduleTimeout != 0 {
		scheduleTimeout = &opts.ScheduleTimeout
	}
	if opts.Retries != 0 {
		retries = &opts.Retries
	}

	taskDecl := &task.OnFailureTaskDeclaration[I]{
		Fn: genericFn,
		TaskShared: task.TaskShared{
			ExecutionTimeout:       executionTimeout,
			ScheduleTimeout:        scheduleTimeout,
			Retries:                retries,
			RetryBackoffFactor:     retryBackoffFactor,
			RetryMaxBackoffSeconds: retryMaxBackoffSeconds,
			RateLimits:             opts.RateLimits,
			WorkerLabels:           opts.WorkerLabels,
			Concurrency:            opts.Concurrency,
		},
	}

	w.OnFailureTask = taskDecl

	return taskDecl
}

// RunNoWait executes the workflow with the provided input without waiting for it to complete.
// Instead it returns a run ID that can be used to check the status of the workflow.
func (w *workflowDeclarationImpl[I, O]) RunNoWait(input I, opts ...RunOpts) (*v0Client.Workflow, error) {

	// TODO namespace

	runOpts := []v0Client.RunOptFunc{}

	if len(opts) > 0 {
		if opts[0].AdditionalMetadata != nil {
			fn := v0Client.WithRunMetadata(opts[0].AdditionalMetadata)
			runOpts = append(runOpts, fn)
		}
	}

	run, err := w.v0.Admin().RunWorkflow(w.Name, input, runOpts...)
	if err != nil {
		return nil, err
	}

	return run, nil
}

// RunAsChild executes the workflow as a child workflow with the provided input.
func (w *workflowDeclarationImpl[I, O]) RunAsChild(ctx worker.HatchetContext, input I, opts ...RunAsChildOpts) (*O, error) {

	runOpts := RunOpts{}
	var desiredWorker *string
	var key *string
	if len(opts) > 0 {
		if opts[0].AdditionalMetadata != nil {
			runOpts.AdditionalMetadata = opts[0].AdditionalMetadata
		}

		if opts[0].Sticky != nil {
			if !ctx.Worker().HasWorkflow(w.Name) {
				return nil, fmt.Errorf("cannot run with sticky: workflow %s is not registered on this worker", w.Name)
			}

			id := ctx.Worker().ID()
			desiredWorker = &id
		}

		if opts[0].Key != nil {
			key = opts[0].Key
		}
	}

	childOpts := &client.ChildWorkflowOpts{
		ParentId:        ctx.WorkflowRunId(),
		ParentStepRunId: ctx.StepRunId(),
		ChildIndex:      ctx.CurChildIndex(),
		ChildKey:        key,
		DesiredWorkerId: desiredWorker,
	}

	ctx.IncChildIndex()

	runOpts.childOpts = childOpts

	return w.Run(input, runOpts)
}

// Run executes the workflow with the provided input.
// It triggers a workflow run via the Hatchet client and waits for the result.
// Returns the workflow output and any error encountered during execution.
func (w *workflowDeclarationImpl[I, O]) Run(input I, opts ...RunOpts) (*O, error) {
	run, err := w.RunNoWait(input, opts...)

	if err != nil {
		return nil, err
	}

	workflowResult, err := run.Result()

	if err != nil {
		return nil, err
	}

	// Create a new output object
	var output O

	if w.outputKey != nil {
		// Extract task output using the StepOutput method for the specific output key
		err := workflowResult.StepOutput(*w.outputKey, &output)
		if err != nil {
			// Log the error
			fmt.Printf("Error extracting output for task %s: %v\n", *w.outputKey, err)
			return nil, err
		}
	} else {
		// Iterate through each task with a registered output setter
		for taskName, setter := range w.outputSetters {
			// Extract the specific task output using StepOutput
			var taskOutput interface{}

			// Use reflection to create the correct type for the task output
			for fieldName, fieldType := range getStructFields(reflect.TypeOf(output)) {
				if strings.EqualFold(fieldName, taskName) {
					taskOutput = reflect.New(fieldType).Interface()
					break
				}
			}

			if taskOutput == nil {
				continue // Skip if we couldn't find a matching field
			}

			// Extract task output using the StepOutput method
			err := workflowResult.StepOutput(taskName, &taskOutput)
			if err != nil {
				// Log the error but continue with other tasks
				fmt.Printf("Error extracting output for task %s: %v\n", taskName, err)
				continue
			}

			// Set the output value using the registered setter
			setter(&output, taskOutput)
		}
	}

	return &output, nil
}

// Helper function to get field names and types of a struct
func getStructFields(t reflect.Type) map[string]reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	fields := make(map[string]reflect.Type)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fields[field.Name] = field.Type
	}

	return fields
}

// Cron schedules the workflow to run on a regular basis using a cron expression.
func (w *workflowDeclarationImpl[I, O]) Cron(name string, cronExpr string, input I, opts ...RunOpts) (*rest.CronWorkflows, error) {
	// Process additional metadata from options
	var additionalMetadata map[string]interface{}
	if len(opts) > 0 && opts[0].AdditionalMetadata != nil {
		additionalMetadata = *opts[0].AdditionalMetadata
	}

	var inputMap map[string]interface{}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(inputBytes, &inputMap); err != nil {
		return nil, err
	}

	cronWorkflow, err := w.crons.Create(w.Name, features.CreateCronTrigger{
		Name:               name,
		Expression:         cronExpr,
		Input:              inputMap,
		AdditionalMetadata: additionalMetadata,
	})

	if err != nil {
		return nil, err
	}

	return cronWorkflow, nil
}

// Schedule schedules the workflow to run at a specific time.
func (w *workflowDeclarationImpl[I, O]) Schedule(triggerAt time.Time, input I, opts ...RunOpts) (*rest.ScheduledWorkflows, error) {
	// Process additional metadata from options
	var additionalMetadata map[string]interface{}
	if len(opts) > 0 && opts[0].AdditionalMetadata != nil {
		additionalMetadata = *opts[0].AdditionalMetadata
	}

	var inputMap map[string]interface{}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(inputBytes, &inputMap); err != nil {
		return nil, err
	}

	scheduledWorkflow, err := w.schedules.Create(w.Name, features.CreateScheduledRunTrigger{
		TriggerAt:          triggerAt,
		Input:              inputMap,
		AdditionalMetadata: additionalMetadata,
	})

	if err != nil {
		return nil, err
	}

	return scheduledWorkflow, nil
}

// Dump converts the workflow declaration into a protobuf request and function mappings.
// This is used to serialize the workflow for transmission to the Hatchet server.
// Returns the workflow definition as a protobuf request, the task functions, and the on-failure task function.
func (w *workflowDeclarationImpl[I, O]) Dump() (*contracts.CreateWorkflowVersionRequest, []NamedFunction, []NamedFunction, WrappedTaskFn) {
	taskOpts := make([]*contracts.CreateTaskOpts, len(w.tasks))
	for i, task := range w.tasks {
		taskOpts[i] = task.Dump(w.Name, w.TaskDefaults)
	}

	durableOpts := make([]*contracts.CreateTaskOpts, len(w.durableTasks))
	for i, task := range w.durableTasks {
		durableOpts[i] = task.Dump(w.Name, w.TaskDefaults)
	}

	tasksToRegister := append(taskOpts, durableOpts...)

	req := &contracts.CreateWorkflowVersionRequest{
		Tasks:         tasksToRegister,
		Name:          w.Name,
		EventTriggers: w.OnEvents,
		CronTriggers:  w.OnCron,
	}

	if w.Version != nil {
		req.Version = *w.Version
	}

	if w.Description != nil {
		req.Description = *w.Description
	}

	if w.Concurrency != nil {
		req.Concurrency = &contracts.Concurrency{
			Expression: w.Concurrency.Expression,
			MaxRuns:    w.Concurrency.MaxRuns,
		}

		if w.Concurrency.LimitStrategy != nil {
			strategy := *w.Concurrency.LimitStrategy
			strategyInt := contracts.ConcurrencyLimitStrategy_value[string(strategy)]
			strategyEnum := contracts.ConcurrencyLimitStrategy(strategyInt)
			req.Concurrency.LimitStrategy = &strategyEnum
		}
	}

	if w.OnFailureTask != nil {
		req.OnFailureTask = w.OnFailureTask.Dump(w.Name, w.TaskDefaults)
	}

	if w.StickyStrategy != nil {
		stickyStrategy := contracts.StickyStrategy(*w.StickyStrategy)
		req.Sticky = &stickyStrategy
	}

	// Create named function objects for regular tasks
	regularNamedFns := make([]NamedFunction, len(w.tasks))
	for i, task := range w.tasks {
		taskName := task.Name
		originalFn := w.taskFuncs[taskName]

		regularNamedFns[i] = NamedFunction{
			ActionID: taskOpts[i].Action,
			Fn: func(ctx worker.HatchetContext) (interface{}, error) {
				var input I
				err := ctx.WorkflowInput(&input)
				if err != nil {
					return nil, err
				}

				// Call the original function using reflection
				fnValue := reflect.ValueOf(originalFn)
				inputs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(input)}
				results := fnValue.Call(inputs)

				// Handle errors
				if !results[1].IsNil() {
					return nil, results[1].Interface().(error)
				}

				// Return the output
				return results[0].Interface(), nil
			},
		}
	}

	// Create named function objects for durable tasks
	durableNamedFns := make([]NamedFunction, len(w.durableTasks))
	for i, task := range w.durableTasks {
		taskName := task.Name
		originalFn := w.durableTaskFuncs[taskName]

		durableNamedFns[i] = NamedFunction{
			ActionID: durableOpts[i].Action,
			Fn: func(ctx worker.HatchetContext) (interface{}, error) {
				var input I
				err := ctx.WorkflowInput(&input)
				if err != nil {
					return nil, err
				}

				// Create a DurableHatchetContext from the HatchetContext
				durableCtx := worker.NewDurableHatchetContext(ctx)

				// Call the original function using reflection
				fnValue := reflect.ValueOf(originalFn)
				inputs := []reflect.Value{reflect.ValueOf(input), reflect.ValueOf(durableCtx)}
				results := fnValue.Call(inputs)

				// Handle errors
				if !results[1].IsNil() {
					return nil, results[1].Interface().(error)
				}

				// Return the output
				return results[0].Interface(), nil
			},
		}
	}

	var onFailureFn WrappedTaskFn
	if w.OnFailureTask != nil {
		onFailureFn = func(ctx worker.HatchetContext) (interface{}, error) {
			var input I
			err := ctx.WorkflowInput(&input)
			if err != nil {
				return nil, err
			}

			// Call the function using reflection
			fnValue := reflect.ValueOf(w.OnFailureTask.Fn)
			inputs := []reflect.Value{reflect.ValueOf(input), reflect.ValueOf(ctx)}
			results := fnValue.Call(inputs)

			// Handle errors
			if !results[1].IsNil() {
				return nil, results[1].Interface().(error)
			}

			// Get the result
			result := results[0].Interface()

			return result, nil
		}
	}

	return req, regularNamedFns, durableNamedFns, onFailureFn
}

// Get retrieves the current state of the workflow.
func (w *workflowDeclarationImpl[I, O]) Get() (*rest.Workflow, error) {
	workflow, err := w.workflows.Get(w.Name)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

// // IsPaused checks if the workflow is currently paused.
// func (w *workflowDeclarationImpl[I, O]) IsPaused() (bool, error) {
// 	paused, err := (*w.workflows).IsPaused(w.Name)
// 	if err != nil {
// 		return false, err
// 	}

// 	return paused, nil
// }

// // Pause pauses the assignment of new workflow runs.
// func (w *workflowDeclarationImpl[I, O]) Pause() error {
// 	_, err := (*w.workflows).Pause(w.Name)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Unpause resumes the assignment of workflow runs.
// func (w *workflowDeclarationImpl[I, O]) Unpause() error {
// 	_, err := (*w.workflows).Unpause(w.Name)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// Metrics retrieves metrics for this workflow.
func (w *workflowDeclarationImpl[I, O]) Metrics(opts ...rest.WorkflowGetMetricsParams) (*rest.WorkflowMetrics, error) {
	var options rest.WorkflowGetMetricsParams
	if len(opts) > 0 {
		options = opts[0]
	}

	metrics, err := w.metrics.GetWorkflowMetrics(w.Name, &options)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// QueueMetrics retrieves queue metrics for this workflow.
func (w *workflowDeclarationImpl[I, O]) QueueMetrics(opts ...rest.TenantGetQueueMetricsParams) (*rest.TenantGetQueueMetricsResponse, error) {
	var options rest.TenantGetQueueMetricsParams
	if len(opts) > 0 {
		options = opts[0]
	}

	// Ensure the workflow name is set
	if options.Workflows == nil {
		options.Workflows = &[]string{w.Name}
	} else {
		// Add this workflow to the list if not already present
		found := false
		for _, wf := range *options.Workflows {
			if wf == w.Name {
				found = true
				break
			}
		}
		if !found {
			*options.Workflows = append(*options.Workflows, w.Name)
		}
	}

	metrics, err := w.metrics.GetQueueMetrics(&options)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// RunChildWorkflow is a helper function to run a child workflow with full type safety
// It takes the parent context, the child workflow declaration, and input
// Returns the typed output of the child workflow
func RunChildWorkflow[I any, O any](
	ctx worker.HatchetContext,
	workflow WorkflowDeclaration[I, O],
	input I,
	opts ...RunOpts,
) (*O, error) {
	// Get the workflow name
	wfImpl, ok := workflow.(*workflowDeclarationImpl[I, O])
	if !ok {
		return nil, fmt.Errorf("invalid workflow declaration type")
	}

	// Set up additional metadata if provided
	var additionalMetadata *map[string]string
	if len(opts) > 0 && opts[0].AdditionalMetadata != nil {
		metadataStr := make(map[string]string)
		for k, v := range *opts[0].AdditionalMetadata {
			// Convert interface{} values to strings
			switch val := v.(type) {
			case string:
				metadataStr[k] = val
			default:
				// For non-string values, convert to JSON string
				bytes, err := json.Marshal(val)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal metadata value: %w", err)
				}
				metadataStr[k] = string(bytes)
			}
		}
		additionalMetadata = &metadataStr
	}

	// Spawn the child workflow
	childWorkflow, err := ctx.SpawnWorkflow(wfImpl.Name, input, &worker.SpawnWorkflowOpts{
		AdditionalMetadata: additionalMetadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to spawn child workflow: %w", err)
	}

	// Wait for the result
	workflowResult, err := childWorkflow.Result()
	if err != nil {
		return nil, fmt.Errorf("child workflow execution failed: %w", err)
	}

	// Create a new output object
	var output O

	// Iterate through each task with a registered output setter
	for taskName, setter := range wfImpl.outputSetters {
		// Extract the specific task output using StepOutput
		var taskOutput interface{}

		// Use reflection to create the correct type for the task output
		for fieldName, fieldType := range getStructFields(reflect.TypeOf(output)) {
			if strings.EqualFold(fieldName, taskName) {
				taskOutput = reflect.New(fieldType).Interface()
				break
			}
		}

		if taskOutput == nil {
			continue // Skip if we couldn't find a matching field
		}

		// Extract task output using the StepOutput method
		err := workflowResult.StepOutput(taskName, taskOutput)
		if err != nil {
			// Log the error but continue with other tasks
			fmt.Printf("Error extracting output for task %s: %v\n", taskName, err)
			continue
		}

		// Set the output value using the registered setter
		setter(&output, taskOutput)
	}

	return &output, nil
}
