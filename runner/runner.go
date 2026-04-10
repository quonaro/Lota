package runner

import (
	"fmt"
	"lota/config"
	"os"
	"os/exec"
)

// executeShell runs a script in shell with environment variables
func executeShell(script string, env []string) error {
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ExecuteCommand(cmd *config.Command, context InterpolationContext) error {
	env := VarsToEnv(context.Vars)

	// before hook
	if cmd.Before != "" {
		interpolatedBefore, err := Interpolate(cmd.Before, context)
		if err != nil {
			return fmt.Errorf("before hook interpolation failed: %w", err)
		}
		if err := executeShell(interpolatedBefore, env); err != nil {
			return fmt.Errorf("before hook failed: %w", err)
		}
	}

	// after hook via defer (always executes)
	defer func() {
		if cmd.After != "" {
			interpolatedAfter, err := Interpolate(cmd.After, context)
			if err != nil {
				fmt.Printf("after hook interpolation failed: %v\n", err)
				return
			}
			if err := executeShell(interpolatedAfter, env); err != nil {
				fmt.Printf("after hook failed: %v\n", err)
			}
		}
	}()

	// script
	if cmd.Script != "" {
		interpolatedScript, err := Interpolate(cmd.Script, context)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		return executeShell(interpolatedScript, env)
	}

	return nil
}
