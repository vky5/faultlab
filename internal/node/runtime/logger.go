package runtime

import (
	"context"
	"fmt"
	"time"
)

// Logger wraps local formatting and asynchronously reports standard node logs to the ControlPlane
type Logger struct {
	nodeID string
	cp     CPSession
	prefix string
}

func NewLogger(nodeID, prefix string, cp CPSession) *Logger {
	return &Logger{
		nodeID: nodeID,
		cp:     cp,
		prefix: prefix,
	}
}

func (l *Logger) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	
	// Print locally
	if l.prefix != "" {
		fmt.Printf("[%s] %s\n", l.prefix, msg)
	} else {
		fmt.Print(msg)
		if len(msg) > 0 && msg[len(msg)-1] != '\n' {
			fmt.Println()
		}
	}

	// Send to Control Plane asynchronously so performance isn't impacted
	if l.cp != nil {
		// remove trailing newlines for RPC
		cleanMsg := msg
		if len(cleanMsg) > 0 && cleanMsg[len(cleanMsg)-1] == '\n' {
			cleanMsg = cleanMsg[:len(cleanMsg)-1]
		}
		
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = l.cp.ReportLog(ctx, "INFO", cleanMsg)
		}()
	}
}
