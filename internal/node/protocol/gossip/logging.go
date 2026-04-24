package gossip

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultGossipLogDir = "logs"
	gossipLogDirEnv     = "FAULTLAB_GOSSIP_LOG_DIR"
)

var gossipFileLogMu sync.Mutex

func appendGossipFileLog(msg string) error {
	now := time.Now()
	logDir := os.Getenv(gossipLogDirEnv)
	if logDir == "" {
		logDir = defaultGossipLogDir
	}

	gossipFileLogMu.Lock()
	defer gossipFileLogMu.Unlock()

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("gossip-%s.log", now.Format("2006-01-02")))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s [gossip] %s\n", now.Format("2006-01-02 15:04:05.000"), msg)
	return err
}
