// Files: documents (CSV, Markdown, PDF, ...) flowing through a workflow.
//
// Files given in WorkflowExecutionInput.Files are visible to every step as
// StepInput.Files. A step publishes files by returning them in
// StepOutput.Files; they are appended to the input of every later step and
// collected in WorkflowRunResponse.Files (input files first, then produced
// files in order). Containers (Parallel, Condition, Loop, Router, Steps)
// forward their members' files; a Parallel merges them in declaration order.
// (The workflow runner does not accept a *v2.Steps as a top-level step yet,
// so this example chains plain steps after the Parallel.)
//
// Runs offline (no model needed):
//
//	go run ./cookbook/workflow_prompt/files
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	// Parallel: two exporters each produce one file from the input CSV.
	export := v2.NewParallel(v2.WithParallelName("export"), v2.WithParallelSteps(
		mustStep("to_markdown", func(in *v2.StepInput) (*v2.StepOutput, error) {
			csv := findFile(in.Files, "sales.csv")
			md := "| " + strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(string(csv.Data)), ",", " | "), "\n", " |\n| ") + " |\n"
			return &v2.StepOutput{Content: "markdown ready", Files: []models.File{{Name: "sales.md", MimeType: "text/markdown", Data: []byte(md)}}}, nil
		}),
		mustStep("to_summary", func(in *v2.StepInput) (*v2.StepOutput, error) {
			lines := strings.Split(strings.TrimSpace(string(findFile(in.Files, "sales.csv").Data)), "\n")
			txt := fmt.Sprintf("%d rows", len(lines)-1)
			return &v2.StepOutput{Content: txt, Files: []models.File{{Name: "summary.txt", MimeType: "text/plain", Data: []byte(txt)}}}, nil
		}),
	))

	// Sequential steps: each one sees the input files plus the files of every
	// earlier step (here: sales.csv, sales.md, summary.txt).
	assemble := mustStep("assemble", func(in *v2.StepInput) (*v2.StepOutput, error) {
		fmt.Println("assemble sees:", names(in.Files))
		body := "# Sales report\n\n" + string(findFile(in.Files, "sales.md").Data) +
			"\n" + string(findFile(in.Files, "summary.txt").Data) + "\n"
		return &v2.StepOutput{Content: "report assembled", Files: []models.File{{Name: "report.md", MimeType: "text/markdown", Data: []byte(body)}}}, nil
	})
	check := mustStep("check", func(in *v2.StepInput) (*v2.StepOutput, error) {
		fmt.Println("check sees:   ", names(in.Files))
		return &v2.StepOutput{Content: "report.md present: " + fmt.Sprint(findFile(in.Files, "report.md") != nil)}, nil
	})

	wf := v2.NewWorkflow(v2.WithWorkflowName("files demo"), v2.WithWorkflowSteps([]interface{}{export, assemble, check}))

	resp, err := wf.Run(context.Background(), &v2.WorkflowExecutionInput{
		Message: "Build the sales report",
		Files: []models.File{{
			Name:     "sales.csv",
			MimeType: "text/csv",
			Data:     []byte("region,amount\nnorth,120\nsouth,95\n"),
		}},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nworkflow files:", names(resp.Files))
	fmt.Printf("\n%s", findFile(resp.Files, "report.md").Data)
}

func mustStep(name string, fn v2.ExecutorFunc) *v2.Step {
	step, err := v2.NewStep(v2.WithName(name), v2.WithExecutor(fn))
	if err != nil {
		log.Fatal(err)
	}
	return step
}

func findFile(files []models.File, name string) *models.File {
	for i := range files {
		if files[i].Name == name {
			return &files[i]
		}
	}
	return nil
}

func names(files []models.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}
