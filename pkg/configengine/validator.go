package configengine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"netora-dns/pkg/model"
)

// CommandRunner abstracts shell execution for unbound-checkconf and systemctl.
type CommandRunner interface {
	Run(ctx context.Context, command string, args ...string) (output string, err error)
}

// LocalCommandRunner executes commands on the local machine.
type LocalCommandRunner struct{}

func (l *LocalCommandRunner) Run(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Validator handles pre-flight semantic and syntax checks.
type Validator struct {
	runner CommandRunner
}

// NewValidator creates a validator instance with the specified runner.
func NewValidator(runner CommandRunner) *Validator {
	if runner == nil {
		runner = &LocalCommandRunner{}
	}
	return &Validator{runner: runner}
}

// ValidationResult summarizes syntax and semantic check results.
type ValidationResult struct {
	Valid          bool     `json:"valid"`
	SyntaxChecked  bool     `json:"syntax_checked"`
	SemanticErrors []string `json:"semantic_errors,omitempty"`
	CheckconfOutput string   `json:"checkconf_output,omitempty"`
}

// ValidateState runs semantic validation.
func (v *Validator) ValidateState(state *model.DesiredState) *ValidationResult {
	res := &ValidationResult{Valid: true}
	errs := model.ValidateDesiredState(state)
	if len(errs) > 0 {
		res.Valid = false
		for _, e := range errs {
			res.SemanticErrors = append(res.SemanticErrors, e.Error())
		}
	}
	return res
}

// ValidateSyntaxWithCheckconf invokes unbound-checkconf on a generated configuration file.
func (v *Validator) ValidateSyntaxWithCheckconf(ctx context.Context, confFilePath string) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := v.runner.Run(ctxTimeout, "unbound-checkconf", confFilePath)
	if err != nil {
		return output, fmt.Errorf("unbound-checkconf syntax error: %w (output: %s)", err, output)
	}
	return output, nil
}
