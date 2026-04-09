package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	clrReset  = "\033[0m"
	clrBold   = "\033[1m"
	clrRed    = "\033[31m"
	clrGreen  = "\033[32m"
	clrYellow = "\033[33m"
	clrCyan   = "\033[36m"
)

func main() {
	host := flag.String("host", "localhost", "control plane command listener host")
	port := flag.Int("port", 9091, "control plane command listener port")
	token := flag.String("token", "", "optional command listener auth token")
	timeout := flag.Duration("timeout", 5*time.Second, "request timeout")
	interactive := flag.Bool("interactive", false, "run in interactive mode (continuous input)")
	flag.Parse()

	command := strings.TrimSpace(strings.Join(flag.Args(), " "))

	if *interactive || command == "" {
		runInteractive(*host, *port, *token, *timeout)
		return
	}

	if isLocalHelpCommand(command) {
		printHelp()
		return
	}

	if err := sendCommand(*host, *port, *token, *timeout, command); err != nil {
		printError(err)
		os.Exit(1)
	}
}

func runInteractive(host string, port int, token string, timeout time.Duration) {
	printInfo(fmt.Sprintf("cpcli connected to %s:%d", host, port))
	fmt.Println("Type commands and press Enter. Type 'exit' or 'quit' to stop.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(clrCyan + "cpcli> " + clrReset)
		if !scanner.Scan() {
			fmt.Println()
			return
		}

		command := strings.TrimSpace(scanner.Text())
		if command == "" {
			continue
		}
		if command == "exit" || command == "quit" {
			return
		}
		if isLocalHelpCommand(command) {
			printHelp()
			continue
		}

		if err := sendCommand(host, port, token, timeout, command); err != nil {
			printError(err)
		}
	}
}

func isLocalHelpCommand(command string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(command))
	return trimmed == "help" || trimmed == "--help" || trimmed == "-h" || trimmed == "?"
}

func printHelp() {
	fmt.Println(clrBold + clrYellow + "cpcli Help" + clrReset)
	fmt.Println("")
	fmt.Println(clrBold + "How to use" + clrReset)
	fmt.Println("  Interactive mode: run cpcli with no command")
	fmt.Println("  One-shot mode: run cpcli with a command")
	fmt.Println("")
	fmt.Println(clrBold + "Interactive shortcuts" + clrReset)
	fmt.Println("  help, ?, -h, --help  show this help")
	fmt.Println("  exit, quit          leave interactive mode")
	fmt.Println("")
	fmt.Println(clrBold + "Cluster commands" + clrReset)
	fmt.Println("  new-cluster <cluster-id> [--protocol <gossip|raft>]")
	fmt.Println("  list-clusters")
	fmt.Println("  set-protocol <cluster-id> <gossip|raft>")
	fmt.Println("")
	fmt.Println(clrBold + "Node lifecycle commands" + clrReset)
	fmt.Println("  start-node <node-id> <port> [--cluster-id <id>] [--host <host>] [--peers <csv>] [--cp-host <host>] [--cp-port <port>]")
	fmt.Println("  stop-node <node-id>")
	fmt.Println("  list-node-procs")
	fmt.Println("  add-node <cluster-id> <node-id> <host> <port>")
	fmt.Println("  remove-node <cluster-id> <node-id>")
	fmt.Println("  list-nodes <cluster-id>")
	fmt.Println("")
	fmt.Println(clrBold + "Fault commands" + clrReset)
	fmt.Println("  set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]")
	fmt.Println("  fault-crash <cluster-id> <node-id>")
	fmt.Println("  fault-recover <cluster-id> <node-id>")
	fmt.Println("  fault-drop <cluster-id> <node-id> <drop-rate:0..1>")
	fmt.Println("  fault-delay <cluster-id> <node-id> <delay-ms:int>")
	fmt.Println("  fault-partition <cluster-id> <node-id> <peer-id> <enabled:true|false>")
	fmt.Println("")
	fmt.Println(clrBold + "KV commands" + clrReset)
	fmt.Println("  kv-put <cluster-id> <node-id> <key> <value>")
	fmt.Println("  kv-get <cluster-id> <node-id> <key>")
}

func sendCommand(host string, port int, token string, timeout time.Duration, command string) error {

	url := fmt.Sprintf("http://%s:%d/command", host, port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(command))
	if err != nil {
		return fmt.Errorf("request build failed: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("X-Command-Token", token)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	text := strings.TrimSpace(string(body))
	if resp.StatusCode >= 400 {
		if text == "" {
			text = resp.Status
		}
		return fmt.Errorf("%s", text)
	}

	if text != "" {
		printSuccess(text)
	}

	return nil
}

func printInfo(msg string) {
	fmt.Println(clrCyan + msg + clrReset)
}

func printSuccess(msg string) {
	fmt.Println(clrGreen + msg + clrReset)
}

func printError(err error) {
	fmt.Fprintln(os.Stderr, clrRed+err.Error()+clrReset)
}
