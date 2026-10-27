package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"claude-turbo-search/memorydb/internal/db"
)

type handoffPayload struct {
	SourceAgent   string   `json:"source_agent"`
	SourceSession string   `json:"source_session"`
	Objective     string   `json:"objective"`
	Completed     []string `json:"completed"`
	Failed        []string `json:"failed_approaches"`
	Questions     []string `json:"open_questions"`
	NextSteps     []string `json:"next_steps"`
}

func (a *App) ensureHandoffs() error {
	schema, err := os.ReadFile(filepath.Join(a.ScriptDir, "schema-handoff.sql"))
	if err != nil {
		return err
	}
	_, err = a.DB.RunSQL(string(schema))
	return err
}

func (a *App) CmdHandoff(args []string) error {
	if !a.dbExists() {
		return errors.New("no memory database found. run 'memory-db.sh init' first")
	}
	switch arg(args, 0) {
	case "save":
		if len(args) != 1 {
			return errors.New("handoff save reads one JSON object from stdin")
		}
		id, err := a.saveHandoff(os.Stdin)
		if err != nil {
			return err
		}
		return a.printHandoffs(fmt.Sprintf("id=%d", id))
	case "list":
		if len(args) != 1 {
			return errors.New("usage: handoff list")
		}
		if !a.DB.HasTable("handoffs") {
			fmt.Println("[]")
			return nil
		}
		return a.printHandoffs("status='open'")
	case "show", "accept":
		id, err := strconv.Atoi(arg(args, 1))
		if err != nil || id <= 0 {
			return errors.New("handoff ID must be a positive integer")
		}
		if args[0] == "show" && len(args) == 2 {
			if !a.DB.HasTable("handoffs") {
				return errors.New("handoff not found")
			}
			return a.printHandoffs(fmt.Sprintf("id=%d", id))
		}
		if args[0] == "accept" && len(args) == 3 {
			if err := a.acceptHandoff(id, args[2]); err != nil {
				return err
			}
			return a.printHandoffs(fmt.Sprintf("id=%d", id))
		}
	}
	return errors.New("usage: handoff {save|list|show ID|accept ID AGENT:SESSION}")
}

func validIdentity(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 256 && !strings.ContainsAny(value, "\x00\r\n")
}

func (a *App) saveHandoff(input io.Reader) (int, error) {
	data, err := io.ReadAll(io.LimitReader(input, 16385))
	if err != nil {
		return 0, err
	}
	if len(data) > 16384 {
		return 0, errors.New("handoff exceeds 16 KiB")
	}
	var payload handoffPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return 0, fmt.Errorf("invalid handoff JSON: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return 0, errors.New("expected exactly one handoff JSON object")
	}
	if !validIdentity(payload.SourceAgent) || !validIdentity(payload.SourceSession) || strings.TrimSpace(payload.Objective) == "" {
		return 0, errors.New("source_agent, source_session and objective are required")
	}
	fields := []string{payload.Objective}
	for _, list := range [][]string{payload.Completed, payload.Failed, payload.Questions, payload.NextSteps} {
		if len(list) > 32 {
			return 0, errors.New("handoff lists may contain at most 32 entries")
		}
		fields = append(fields, list...)
	}
	for _, field := range fields {
		if len(field) > 4000 || strings.ContainsRune(field, '\x00') {
			return 0, errors.New("handoff fields must be at most 4000 bytes and contain no NUL")
		}
	}
	if err := a.ensureHandoffs(); err != nil {
		return 0, err
	}
	gitValue := func(args ...string) string {
		out, _ := exec.Command("git", append([]string{"-C", a.RepoRoot}, args...)...).Output()
		return strings.TrimSpace(string(out))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	sql := fmt.Sprintf(`BEGIN IMMEDIATE;
INSERT INTO handoffs(source_agent,source_session,worktree,branch,git_commit,body)
VALUES ('%s','%s','%s','%s','%s','%s')
ON CONFLICT(source_agent,source_session) DO UPDATE SET
 body=excluded.body, branch=excluded.branch, git_commit=excluded.git_commit,
 worktree=excluded.worktree, updated_at=CURRENT_TIMESTAMP
WHERE handoffs.status='open';
SELECT id FROM handoffs WHERE source_agent='%s' AND source_session='%s' AND status='open';
COMMIT;`, db.SQLQuote(payload.SourceAgent), db.SQLQuote(payload.SourceSession), db.SQLQuote(a.RepoRoot),
		db.SQLQuote(gitValue("symbolic-ref", "--short", "HEAD")), db.SQLQuote(gitValue("rev-parse", "--verify", "HEAD")),
		db.SQLQuote(string(body)), db.SQLQuote(payload.SourceAgent), db.SQLQuote(payload.SourceSession))
	out, err := a.DB.RunSQL(sql)
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, errors.New("handoff already accepted; its saved checkpoint is immutable")
	}
	return id, nil
}

func (a *App) acceptHandoff(id int, claimant string) error {
	if !validIdentity(claimant) {
		return errors.New("a non-empty accepting agent/session identity is required (max 256 bytes)")
	}
	if !a.DB.HasTable("handoffs") {
		return errors.New("handoff not found")
	}
	out, err := a.DB.RunSQL(fmt.Sprintf(`BEGIN IMMEDIATE;
UPDATE handoffs SET status='accepted', accepted_by='%s', updated_at=CURRENT_TIMESTAMP
WHERE id=%d AND status='open';
SELECT changes(); COMMIT;`, db.SQLQuote(claimant), id))
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "1" {
		return errors.New("handoff not found or already accepted; use handoff show ID to inspect it")
	}
	return nil
}

func (a *App) printHandoffs(where string) error {
	out, err := a.DB.Run("-json", "SELECT * FROM handoffs WHERE "+where+" ORDER BY id DESC LIMIT 20;")
	if err != nil {
		return err
	}
	rows := []map[string]any{}
	if strings.TrimSpace(out) != "" {
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			return err
		}
	}
	for _, row := range rows {
		body, ok := row["body"].(string)
		if !ok {
			return errors.New("invalid stored handoff")
		}
		var payload handoffPayload
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			return err
		}
		delete(row, "body")
		row["payload"] = payload
	}
	return json.NewEncoder(os.Stdout).Encode(rows)
}
