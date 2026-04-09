package controlplane

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"text/tabwriter"

	"github.com/vky5/faultlab/internal/cluster"
)

type CommandListenerConfig struct {
	Port      int
	AuthToken string
}

// StartCommandListener starts a simple command listener intended for CLI clients.
// It accepts plain text commands via POST /command.
func StartCommandListener(actor *Actor, cfg CommandListenerConfig) *http.Server {
	if cfg.Port <= 0 {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorizeCommandListener(r, cfg.AuthToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read command: %v", err), http.StatusBadRequest)
			return
		}

		cmd := strings.TrimSpace(string(body))
		if cmd == "" {
			http.Error(w, "empty command", http.StatusBadRequest)
			return
		}

		res, err := ExecuteRuntimeCommand(cmd, actor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if res == nil {
			_, _ = w.Write([]byte("ok\n"))
			return
		}
		_, _ = w.Write([]byte(formatCommandResult(cmd, res)))
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{Addr: addr, Handler: mux}
	log.Printf("control plane command listener on %s", addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("command listener stopped: %v", err)
		}
	}()

	return server
}

func authorizeCommandListener(r *http.Request, token string) bool {
	if strings.TrimSpace(token) == "" {
		return true
	}

	if r.Header.Get("X-Command-Token") == token {
		return true
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == token
	}

	return false
}

func formatCommandResult(rawCmd string, res interface{}) string {
	cmdName := commandName(rawCmd)

	if cmdName == "list-nodes" {
		if nodes, ok := res.([]cluster.Node); ok {
			return formatNodesTable(nodes)
		}
	}

	return fmt.Sprintf("%+v\n", res)
}

func commandName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}
	if parts[0] == "cp" {
		if len(parts) < 2 {
			return ""
		}
		return parts[1]
	}
	return parts[0]
}

func formatNodesTable(nodes []cluster.Node) string {
	if len(nodes) == 0 {
		return "no nodes found\n"
	}

	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tADDRESS\tPORT\tSTATUS\tCRASHED\tDROP\tDELAY_MS\tPARTITIONS")
	for _, n := range nodes {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%s\t%t\t%.2f\t%d\t%d\n",
			n.ID,
			n.Address,
			n.Port,
			n.Status,
			n.Fault.Crashed,
			n.Fault.DropRate,
			n.Fault.DelayMs,
			len(n.Fault.Partition),
		)
	}
	_ = tw.Flush()
	return b.String()
}
