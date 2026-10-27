package commands

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"claude-turbo-search/memorydb/internal/db"
)

// Supersession points from the replacement to the original; originals stay readable in history.
func (a *App) activeEntry(kind, id string) string {
	if !a.DB.HasTable("entry_relations") {
		return "1"
	}
	return fmt.Sprintf(`NOT EXISTS (SELECT 1 FROM entry_relations er
WHERE er.relation='supersedes' AND er.to_type=%s AND er.to_id=%s)`, kind, id)
}

func (a *App) ensureRelations() error {
	if !a.dbExists() {
		return errors.New("no memory database found. run 'memory-db.sh init' first")
	}
	schema, err := os.ReadFile(a.MetadataSchema)
	if err != nil {
		return err
	}
	_, err = a.DB.RunSQL(string(schema))
	return err
}

func (a *App) CmdConsolidate() error {
	if err := a.ensureRelations(); err != nil {
		return err
	}
	// Exact duplicates only: topic overlap and substring matches do not establish equivalence.
	sql := `BEGIN IMMEDIATE;
INSERT OR IGNORE INTO entry_relations(from_type,from_id,to_type,to_id,relation)
SELECT 'session', MAX(new.id), 'session', old.id, 'supersedes'
FROM sessions old JOIN sessions new ON new.id > old.id
 AND new.summary=old.summary AND new.files_touched IS old.files_touched
 AND new.tools_used IS old.tools_used AND new.topics IS old.topics
WHERE NOT EXISTS (SELECT 1 FROM entry_relations WHERE relation='supersedes' AND to_type='session' AND to_id=old.id)
 AND NOT EXISTS (SELECT 1 FROM entry_relations WHERE relation='supersedes' AND to_type='session' AND to_id=new.id)
GROUP BY old.id;
SELECT changes();
INSERT OR IGNORE INTO entry_relations(from_type,from_id,to_type,to_id,relation)
SELECT 'fact', MAX(new.id), 'fact', old.id, 'supersedes'
FROM facts old JOIN facts new ON new.id > old.id
 AND new.fact=old.fact AND new.category IS old.category
WHERE NOT EXISTS (SELECT 1 FROM entry_relations WHERE relation='supersedes' AND to_type='fact' AND to_id=old.id)
 AND NOT EXISTS (SELECT 1 FROM entry_relations WHERE relation='supersedes' AND to_type='fact' AND to_id=new.id)
GROUP BY old.id;
SELECT changes();
COMMIT;`
	out, err := a.DB.RunSQL(sql)
	if err != nil {
		return err
	}
	total := 0
	for _, n := range strings.Fields(out) {
		count, err := strconv.Atoi(n)
		if err != nil {
			return err
		}
		total += count
	}
	fmt.Printf("Consolidation complete: %d exact duplicates superseded; no entries deleted.\n", total)
	return nil
}

func entryArgs(args []string, count int) (string, []int, error) {
	if len(args) != count+1 || (args[0] != "session" && args[0] != "fact") {
		return "", nil, errors.New("expected session|fact followed by positive entry IDs")
	}
	ids := make([]int, count)
	for i, value := range args[1:] {
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 {
			return "", nil, errors.New("entry IDs must be positive integers")
		}
		ids[i] = id
	}
	return args[0], ids, nil
}

func (a *App) CmdSupersede(args []string) error {
	kind, ids, err := entryArgs(args, 2)
	if err != nil {
		return err
	}
	if ids[0] == ids[1] {
		return errors.New("replacement must be a different entry")
	}
	if err := a.ensureRelations(); err != nil {
		return err
	}
	// Both entries must be active, preventing cycles and competing replacements.
	sql := fmt.Sprintf(`BEGIN IMMEDIATE;
INSERT INTO entry_relations(from_type,from_id,to_type,to_id,relation)
SELECT '%[1]s', %[3]d, '%[1]s', %[2]d, 'supersedes'
WHERE EXISTS (SELECT 1 FROM %[1]ss WHERE id=%[2]d)
 AND EXISTS (SELECT 1 FROM %[1]ss WHERE id=%[3]d)
 AND NOT EXISTS (SELECT 1 FROM entry_relations WHERE relation='supersedes'
 AND to_type='%[1]s' AND to_id IN (%[2]d,%[3]d));
SELECT changes(); COMMIT;`, kind, ids[0], ids[1])
	out, err := a.DB.RunSQL(sql)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "1" {
		return errors.New("both entries must exist and be active")
	}
	fmt.Printf("%s:%d superseded by %s:%d; original preserved.\n", kind, ids[0], kind, ids[1])
	return nil
}

func (a *App) CmdRestore(args []string) error {
	kind, ids, err := entryArgs(args, 1)
	if err != nil {
		return err
	}
	if err := a.ensureRelations(); err != nil {
		return err
	}
	n, err := a.DB.ScalarInt(fmt.Sprintf("SELECT COUNT(*) FROM %ss WHERE id=%d;", kind, ids[0]))
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("entry not found; previously deleted entries cannot be restored")
	}
	_, err = a.DB.RunSQL(fmt.Sprintf(`DELETE FROM entry_relations
WHERE relation='supersedes' AND to_type='%s' AND to_id=%d;`, db.SQLQuote(kind), ids[0]))
	if err == nil {
		fmt.Printf("%s:%d restored to retrieval; replacement entries retained.\n", kind, ids[0])
	}
	return err
}
