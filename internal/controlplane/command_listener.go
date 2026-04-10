package controlplane

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"sort"
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

// authorizeCommandListener checks if the incoming request has the correct auth token for command execution.
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

	if cmdName == "list-clusters" {
		if out, ok := formatClustersTable(res); ok {
			return out
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

//used in handling list node result
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

// used in handling the cluster list command result
func formatClustersTable(res interface{}) (string, bool) {
	v := reflect.ValueOf(res)
	if !v.IsValid() {
		return "", false
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return "", false
	}
	if v.Len() == 0 {
		return "no clusters found\n", true
	}

	items := make([]reflect.Value, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if item.Kind() == reflect.Ptr {
			if item.IsNil() {
				continue
			}
			item = item.Elem()
		}
		if item.Kind() != reflect.Struct {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return "", false
	}

	sort.Slice(items, func(i, j int) bool {
		return readStringField(items[i], "ID") < readStringField(items[j], "ID")
	})

	var b bytes.Buffer
	summary := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(summary, "CLUSTER_ID\tPROTOCOL\tNODES")
	for _, c := range items {
		nodesField, _ := readField(c, "Nodes")
		nodeCount := 0
		if nodesField.IsValid() && nodesField.Kind() == reflect.Slice {
			nodeCount = nodesField.Len()
		}
		_, _ = fmt.Fprintf(summary, "%s\t%s\t%d\n", readStringField(c, "ID"), readStringField(c, "Protocol"), nodeCount)
	}
	_ = summary.Flush()

	for _, c := range items {
		clusterID := readStringField(c, "ID")
		protocol := readStringField(c, "Protocol")
		nodesField, _ := readField(c, "Nodes")

		_, _ = fmt.Fprintf(&b, "\n[%s] protocol=%s\n", clusterID, protocol)
		if !nodesField.IsValid() || nodesField.Kind() != reflect.Slice || nodesField.Len() == 0 {
			_, _ = fmt.Fprintln(&b, "  no nodes")
			continue
		}

		nodeRows := make([]reflect.Value, 0, nodesField.Len())
		for i := 0; i < nodesField.Len(); i++ {
			n := nodesField.Index(i)
			if n.Kind() == reflect.Ptr {
				if n.IsNil() {
					continue
				}
				n = n.Elem()
			}
			if n.Kind() == reflect.Struct {
				nodeRows = append(nodeRows, n)
			}
		}

		sort.Slice(nodeRows, func(i, j int) bool {
			return readStringField(nodeRows[i], "ID") < readStringField(nodeRows[j], "ID")
		})

		nodesTW := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(nodesTW, "ID\tADDRESS\tPORT\tSTATUS\tCRASHED\tDROP\tDELAY_MS\tPARTITIONS")
		for _, n := range nodeRows {
			faultField, _ := readField(n, "Fault")
			_, _ = fmt.Fprintf(
				nodesTW,
				"%s\t%s\t%d\t%s\t%t\t%.2f\t%d\t%d\n",
				readStringField(n, "ID"),
				readStringField(n, "Address"),
				readIntField(n, "Port"),
				readStringField(n, "Status"),
				readBoolField(faultField, "Crashed"),
				readFloatField(faultField, "DropRate"),
				readIntField(faultField, "DelayMs"),
				readSliceLenField(faultField, "Partition"),
			)
		}
		_ = nodesTW.Flush()
	}

	return b.String(), true
}

func readField(v reflect.Value, fieldName string) (reflect.Value, bool) {
	if !v.IsValid() {
		return reflect.Value{}, false
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	f := v.FieldByName(fieldName)
	if !f.IsValid() {
		return reflect.Value{}, false
	}
	return f, true
}

func readStringField(v reflect.Value, fieldName string) string {
	f, ok := readField(v, fieldName)
	if !ok || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

func readIntField(v reflect.Value, fieldName string) int {
	f, ok := readField(v, fieldName)
	if !ok {
		return 0
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(f.Int())
	default:
		return 0
	}
}

func readFloatField(v reflect.Value, fieldName string) float64 {
	f, ok := readField(v, fieldName)
	if !ok {
		return 0
	}
	switch f.Kind() {
	case reflect.Float32, reflect.Float64:
		return f.Float()
	default:
		return 0
	}
}

func readBoolField(v reflect.Value, fieldName string) bool {
	f, ok := readField(v, fieldName)
	if !ok || f.Kind() != reflect.Bool {
		return false
	}
	return f.Bool()
}

func readSliceLenField(v reflect.Value, fieldName string) int {
	f, ok := readField(v, fieldName)
	if !ok || f.Kind() != reflect.Slice {
		return 0
	}
	return f.Len()
}
