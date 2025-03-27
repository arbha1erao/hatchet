package v1_workflows

import (
	"github.com/hatchet-dev/hatchet/pkg/client/create"
	v1 "github.com/hatchet-dev/hatchet/pkg/v1"
	"github.com/hatchet-dev/hatchet/pkg/v1/factory"
	"github.com/hatchet-dev/hatchet/pkg/v1/workflow"
	"github.com/hatchet-dev/hatchet/pkg/worker"
)

type DurableEventInput struct {
	Message string
}

type EventData struct {
	Message string
}

type DurableEventOutput struct {
	Data EventData
}

func DurableEvent(hatchet v1.HatchetClient) workflow.WorkflowDeclaration[DurableEventInput, DurableEventOutput] {
	durableEventWorkflow := factory.NewDurableTask(
		create.StandaloneTask{
			Name: "durable-sleep",
		},
		func(ctx worker.DurableHatchetContext, input DurableEventInput) (*DurableEventOutput, error) {
			eventData, err := ctx.WaitForEvent("user:update", "")

			if err != nil {
				return nil, err
			}

			v := EventData{}
			err = eventData.Unmarshal(&v)

			if err != nil {
				return nil, err
			}

			return &DurableEventOutput{
				Data: v,
			}, nil
		},
		hatchet,
	)

	return durableEventWorkflow
}
