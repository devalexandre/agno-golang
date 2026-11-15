// Built-in workflow functions, StepProgress events and file artifacts.
//
// Runs offline (no model needed):
//
//	go run ./cookbook/workflow_prompt/builtin_functions
package main

import (
	"context"
	"fmt"
	"log"

	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
	"github.com/devalexandre/agno-golang/agno/workflow/v2/functions"
)

func main() {
	// Stands in for a gathering agent that answers with prose around JSON.
	gather, _ := v2.NewStep(v2.WithName("gather"), v2.WithExecutor(func(in *v2.StepInput) (*v2.StepOutput, error) {
		in.ReportProgress("querying sources", map[string]interface{}{"sources": 2})
		return &v2.StepOutput{Content: `Found these [1]:
[{"city": "Lisbon", "population": 545000, "url": "https://example.com/lisbon"},
 {"city": "Porto", "population": 232000, "url": "https://example.com/porto"}]`}, nil
	}))

	steps := []interface{}{
		gather,
		mustStep("extract_json", functions.ExtractJSON),
		// Parallel members' files are merged into the parallel output.
		v2.NewParallel(v2.WithParallelName("export"), v2.WithParallelSteps(
			mustStep("json_to_csv", functions.JSONToCSV),
			mustStep("urls", functions.ExtractURLs),
		)),
		mustStep("csv", func(in *v2.StepInput) (*v2.StepOutput, error) {
			// Feed the CSV produced inside the parallel step to the table renderer.
			return &v2.StepOutput{Content: in.GetStepContent("export").(map[string]interface{})["json_to_csv"]}, nil
		}),
		mustStep("table", functions.CSVToMarkdownTable),
		mustStep("publish", functions.ContentToFile),
	}

	wf := v2.NewWorkflow(v2.WithWorkflowName("builtin functions"), v2.WithWorkflowSteps(steps))
	wf.OnEvent(v2.StepProgressEvent, func(e *v2.WorkflowRunResponseEvent) {
		p := e.Data.(*v2.StepProgress)
		fmt.Printf("[progress] %v: %s %v\n", e.Metadata["step_name"], p.Content, p.Data)
	})

	resp, err := wf.Run(context.Background(), "Largest cities in Portugal")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Content)
	for _, f := range resp.Files {
		fmt.Printf("file: %s (%s, %d bytes)\n", f.Name, f.MimeType, len(f.Data))
	}
}

func mustStep(name string, fn v2.ExecutorFunc) *v2.Step {
	step, err := v2.NewStep(v2.WithName(name), v2.WithExecutor(fn), v2.WithMaxRetries(0))
	if err != nil {
		log.Fatal(err)
	}
	return step
}
