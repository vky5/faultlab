package controlplane

import (
	"fmt"
	"strings"
)

// DispatchRuntimeCommand executes a controlplane command and prints a user-facing result.
func DispatchRuntimeCommand(raw string, actor *Actor) error {
	res, err := ExecuteRuntimeCommand(raw, actor)
	if err != nil {
		return err
	}

	if res != nil {
		fmt.Printf("result: %+v\n", res)
	} else {
		fmt.Println("ok")
	}

	return nil
}

// ExecuteRuntimeCommand parses and executes a controlplane command.
func ExecuteRuntimeCommand(raw string, actor *Actor) (interface{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Fields(trimmed)
	if len(parts) > 0 && parts[0] == "cp" {
		remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))
		if remainder == "" {
			return nil, fmt.Errorf("usage: cp <controlplane-command...>")
		}
		trimmed = remainder
	}

	cmd, err := Parse(trimmed)
	if err != nil {
		return nil, err
	}

	actor.Submit(cmd)

	res, err := cmd.MapWait()
	if err != nil {
		return nil, err
	}

	return res, nil
}
