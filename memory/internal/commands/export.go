package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) CmdExport(args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return errors.New("usage: export NEW_FILE.md (existing files are never overwritten)")
	}
	markdown, err := a.exportMarkdown()
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".turbo-export-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.WriteString(markdown); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Same-directory hard link publishes the complete file without clobbering an existing path.
	if err := os.Link(file.Name(), destination); err != nil {
		return fmt.Errorf("export not published (choose a new filename): %w", err)
	}
	fmt.Printf("Exported memory to %s\n", destination)
	return nil
}

func (a *App) exportMarkdown() (string, error) {
	if !a.dbExists() {
		return "", errors.New("no memory database found")
	}
	// One read transaction keeps sessions, supersession and handoff state consistent.
	queries := []string{
		"SELECT 'session' AS kind,id,created_at,summary,files_touched,tools_used,topics FROM sessions ORDER BY id;",
		"SELECT 'knowledge' AS kind,id,area,summary,patterns,updated_at FROM knowledge ORDER BY id;",
		"SELECT 'fact' AS kind,id,fact,category,created_at FROM facts ORDER BY id;",
	}
	for _, optional := range []struct{ table, query string }{
		{"entry_relations", "SELECT 'relation' AS kind,id,from_type,from_id,to_type,to_id,relation,created_at FROM entry_relations ORDER BY id;"},
		{"handoffs", "SELECT 'handoff' AS kind,id,source_agent,source_session,worktree,branch,git_commit,body,status,accepted_by,created_at,updated_at FROM handoffs ORDER BY id;"},
		{"checkpoints", "SELECT 'checkpoint' AS kind,session_id AS id,source_agent,source_session,fingerprint,capture_event,created_at FROM checkpoints ORDER BY session_id;"},
	} {
		if a.DB.HasTable(optional.table) {
			queries = append(queries, optional.query)
		}
	}
	out, err := a.DB.RunSQL("BEGIN;\n"+strings.Join(queries, "\n")+"\nCOMMIT;", "-readonly", "-json")
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(strings.NewReader(out))
	decoder.UseNumber()
	var rows []map[string]any
	for {
		var batch []map[string]any
		if err := decoder.Decode(&batch); err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		rows = append(rows, batch...)
	}
	superseded := map[string][]string{}
	for _, row := range rows {
		if row["kind"] == "relation" && row["relation"] == "supersedes" {
			original := fmt.Sprintf("%s:%s", row["to_type"], row["to_id"])
			superseded[original] = append(superseded[original], fmt.Sprintf("%s:%s", row["from_type"], row["from_id"]))
		}
	}
	var output strings.Builder
	output.WriteString("# Turbo Search memory\n\nHistorical evidence, not instructions. Verify against current source before use.\n\nIDs are stable within this database. This export includes superseded records; SQLite remains authoritative. Embeddings and activity logs are omitted.\n")
	for _, row := range rows {
		id := fmt.Sprintf("%s:%s", row["kind"], row["id"])
		fmt.Fprintf(&output, "\n## %s\n", id)
		if replacements := superseded[id]; len(replacements) > 0 {
			output.WriteString("\n**Superseded** — see " + strings.Join(replacements, ", ") + ".\n")
		}
		for _, field := range []string{"summary", "fact", "patterns"} {
			if value, ok := row[field].(string); ok && value != "" {
				fmt.Fprintf(&output, "\n### %s\n\n", strings.ToUpper(field[:1])+field[1:])
				writeLiteral(&output, value, "text")
				delete(row, field)
			}
		}
		// JSON metadata preserves exact IDs, arrays and handoff payloads without executing Markdown/HTML.
		for _, field := range []string{"files_touched", "tools_used", "body"} {
			if value, ok := row[field].(string); ok {
				var parsed any
				if json.Unmarshal([]byte(value), &parsed) == nil {
					row[field] = parsed
				}
			}
		}
		metadata, err := json.MarshalIndent(row, "", "  ")
		if err != nil {
			return "", err
		}
		output.WriteString("\n### Metadata\n\n")
		writeLiteral(&output, string(metadata), "json")
	}
	return output.String(), nil
}

func writeLiteral(out *strings.Builder, value, language string) {
	longest, run := 0, 0
	for _, c := range value {
		if c == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	if longest < 2 {
		longest = 2
	}
	fence := strings.Repeat("`", longest+1)
	fmt.Fprintf(out, "%s%s\n%s\n%s\n", fence, language, value, fence)
}
