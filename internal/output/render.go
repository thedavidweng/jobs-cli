package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/thedavidweng/jobs-cli/internal/safety"
)

type Renderer struct {
	Stdout io.Writer
	Stderr io.Writer
	JSON   bool
	Pretty bool
}

func NewRenderer(stdout, stderr io.Writer, jsonMode, pretty bool) *Renderer {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Renderer{
		Stdout: stdout,
		Stderr: stderr,
		JSON:   jsonMode,
		Pretty: pretty,
	}
}

func (r *Renderer) RenderSuccess(env *Envelope) {
	if r.JSON {
		fmt.Fprintln(r.Stdout, r.marshal(env))
	}
}

func (r *Renderer) RenderError(env *ErrorEnvelope) {
	if r.JSON {
		fmt.Fprintln(r.Stdout, r.marshal(env))
		return
	}
	fmt.Fprintf(r.Stderr, "Error: %s\n", env.Error.Message)
}

func (r *Renderer) RenderPlan(plan safety.Plan) {
	if r.JSON {
		return
	}
	fmt.Fprintf(r.Stdout, "Dry run (%s): no changes were made.\n", plan.Command)
	if len(plan.PlannedMutations) == 0 {
		fmt.Fprintln(r.Stdout, "  no mutations planned")
		return
	}
	fmt.Fprintf(r.Stdout, "Would perform %d mutation(s):\n", len(plan.PlannedMutations))
	for i, mutation := range plan.PlannedMutations {
		fmt.Fprintf(r.Stdout, "  [%d] %s\n", i+1, mutation.Action)
		if mutation.Target != "" {
			fmt.Fprintf(r.Stdout, "      target:   %s\n", mutation.Target)
		}
		if mutation.Provider != "" {
			fmt.Fprintf(r.Stdout, "      provider: %s\n", mutation.Provider)
		}
		if mutation.ResourceID != "" {
			fmt.Fprintf(r.Stdout, "      resource: %s\n", mutation.ResourceID)
		}
		keys := make([]string, 0, len(mutation.Details))
		for key := range mutation.Details {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(r.Stdout, "      %s: %v\n", key, mutation.Details[key])
		}
	}
}

func (r *Renderer) PrintDiagnostic(msg string) {
	fmt.Fprintln(r.Stderr, msg)
}

func (r *Renderer) marshal(v any) string {
	var data []byte
	if r.Pretty {
		data, _ = json.MarshalIndent(v, "", "  ")
	} else {
		data, _ = json.Marshal(v)
	}
	return string(data)
}
