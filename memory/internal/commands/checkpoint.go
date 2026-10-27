package commands

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"claude-turbo-search/memorydb/internal/db"
)

var checkpointSensitive = regexp.MustCompile(`(?i)(^|[/._ -])(secrets?|credentials?|passwords?|tokens?|private[-_]?keys?)([/._ -]|$)`)
var checkpointTool = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,150}$`)

type checkpointPayload struct {
	SourceAgent   string   `json:"source_agent"`
	SourceSession string   `json:"source_session"`
	Fingerprint   string   `json:"fingerprint"`
	Event         string   `json:"event"`
	EventCount    int      `json:"event_count"`
	Files         []string `json:"files"`
	Tools         []string `json:"tools"`
}

func (a *App) ensureCheckpoints() error {
	schema, err := os.ReadFile(filepath.Join(a.ScriptDir, "schema-checkpoint.sql"))
	if err != nil {
		return err
	}
	_, err = a.DB.RunSQL(string(schema))
	return err
}

func (a *App) saveCheckpoint(input io.Reader) error {
	if !a.dbExists() {
		return errors.New("no memory database found; initialize memory outside the hook first")
	}
	data, err := io.ReadAll(io.LimitReader(input, 16385))
	if err != nil {
		return err
	}
	if len(data) > 16384 {
		return errors.New("checkpoint exceeds 16 KiB")
	}
	var p checkpointPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("expected exactly one checkpoint JSON object")
	}
	digest, err := hex.DecodeString(p.Fingerprint)
	if err != nil || len(digest) != 32 || !validIdentity(p.SourceSession) ||
		(p.SourceAgent != "claude" && p.SourceAgent != "codex") ||
		(p.Event != "PreCompact" && p.Event != "SessionEnd") || p.EventCount < 1 || p.EventCount > 200 ||
		len(p.Files) > 32 || len(p.Tools) > 32 {
		return errors.New("invalid checkpoint identity, event, fingerprint or activity bounds")
	}
	for _, value := range append(append([]string{}, p.Files...), p.Tools...) {
		if len(value) > 256 || strings.ContainsAny(value, "\x00\r\n") {
			return errors.New("checkpoint paths and tool names must be bounded single-line strings")
		}
	}
	for _, file := range p.Files {
		if filepath.IsAbs(file) || strings.Contains(file, "\\") || checkpointSensitive.MatchString(file) {
			return errors.New("checkpoint paths must be repository-relative")
		}
		switch strings.ToLower(filepath.Ext(file)) {
		case ".pem", ".key", ".p12", ".pfx":
			return errors.New("checkpoint paths must not name private key files")
		}
		for _, part := range strings.Split(file, "/") {
			if part == "" || strings.HasPrefix(part, ".") {
				return errors.New("checkpoint paths must not contain hidden or parent components")
			}
		}
	}
	for _, tool := range p.Tools {
		if !checkpointTool.MatchString(tool) {
			return errors.New("invalid checkpoint tool name")
		}
	}
	if err := a.ensureCheckpoints(); err != nil {
		return err
	}
	if err := a.ensureRelations(); err != nil {
		return err
	}
	files, _ := json.Marshal(p.Files)
	tools, _ := json.Marshal(p.Tools)
	summary := fmt.Sprintf("Automatic activity checkpoint: observed %d tool events in a window of at most 200. Tools: %s. Files: %s. Task outcome and next steps were not inferred.", p.EventCount, strings.Join(p.Tools, ", "), strings.Join(p.Files, ", "))
	// Keep source snapshots immutable. Replayed boundaries do not insert another session.
	sql := fmt.Sprintf(`BEGIN IMMEDIATE;
CREATE TEMP TABLE checkpoint_previous AS
 SELECT session_id FROM checkpoints WHERE source_agent='%[1]s' AND source_session='%[2]s'
 ORDER BY session_id DESC LIMIT 1;
INSERT INTO sessions(summary,files_touched,tools_used,topics)
 SELECT '%[4]s','%[5]s','%[6]s','automatic-checkpoint'
 WHERE NOT EXISTS (SELECT 1 FROM checkpoints WHERE source_agent='%[1]s' AND source_session='%[2]s' AND fingerprint='%[3]s');
CREATE TEMP TABLE checkpoint_new AS SELECT last_insert_rowid() AS id WHERE changes()=1;
INSERT INTO checkpoints(session_id,source_agent,source_session,fingerprint,capture_event)
 SELECT id,'%[1]s','%[2]s','%[3]s','%[7]s' FROM checkpoint_new;
INSERT OR IGNORE INTO entry_relations(from_type,from_id,to_type,to_id,relation)
 SELECT 'session',n.id,'session',p.session_id,'supersedes' FROM checkpoint_new n CROSS JOIN checkpoint_previous p;
COMMIT;`, db.SQLQuote(p.SourceAgent), db.SQLQuote(p.SourceSession), db.SQLQuote(strings.ToLower(p.Fingerprint)),
		db.SQLQuote(summary), db.SQLQuote(string(files)), db.SQLQuote(string(tools)), db.SQLQuote(p.Event))
	_, err = a.DB.RunSQL(sql)
	return err
}
