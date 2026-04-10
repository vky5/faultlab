/*
This file contains functions to parse and execute controlplane commands
and it is executed by command_listener.go when a command is received via the command listener HTTP endpoint. 
The commands are intended for CLI clients to interact with the control plane at runtime, for example to query cluster state or trigger actions like protocol swap.
controlplane binary cant execute it directly
*/

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
