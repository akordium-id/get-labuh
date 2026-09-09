package deploy

import "fmt"

type DeployError struct {
	Step   string
	Cause  error
	Output string
}

func (e *DeployError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("deploy error at step %s: %v", e.Step, e.Cause)
	}
	return fmt.Sprintf("deploy error at step %s: %s", e.Step, e.Output)
}

func (e *DeployError) Unwrap() error {
	return e.Cause
}
