package audit

import (
	"encoding/csv"
	"os"
	"sync"
	"time"
)

type Entry struct {
	Timestamp    time.Time
	Status       string
	CommandPath  string
	RawCommand   string
	Jira         string
	ActorType    string
	ActorID      string
	AuthRealm    string
	ChangeKind   string
	TargetRealms string
	Duration     string
}

var (
	mu      sync.Mutex
	csvPath = "kc_audit.csv"
)

func Append(e Entry) error {
	mu.Lock()
	defer mu.Unlock()

	fileExists := true
	if _, err := os.Stat(csvPath); err != nil {
		if os.IsNotExist(err) {
			fileExists = false
		} else {
			return err
		}
	}

	f, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)

	if !fileExists {
		header := []string{
			"timestamp",
			"status",
			"command_path",
			"raw_command",
			"jira",
			"actor_type",
			"actor_id",
			"auth_realm",
			"change_kind",
			"target_realms",
			"duration",
		}
		if err := w.Write(header); err != nil {
			return err
		}
	}

	record := []string{
		e.Timestamp.Format(time.RFC3339),
		e.Status,
		e.CommandPath,
		e.RawCommand,
		e.Jira,
		e.ActorType,
		e.ActorID,
		e.AuthRealm,
		e.ChangeKind,
		e.TargetRealms,
		e.Duration,
	}

	if err := w.Write(record); err != nil {
		return err
	}

	w.Flush()
	return w.Error()
}
