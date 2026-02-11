#!/usr/bin/env python3
"""Knowledge Graph TUI Viewer for Claude Turbo Search memory database.

Renders entity relationships, timelines, and statistics from the
SQLite memory database as an interactive TUI using Rich (with
plain-text fallback).
"""

import argparse
import os
import sqlite3
import subprocess
import sys
from collections import defaultdict

# --- Rich dependency handling ---
RICH_AVAILABLE = False
try:
    from rich.console import Console
    from rich.table import Table
    from rich.panel import Panel
    from rich.tree import Tree
    from rich.text import Text
    from rich import box

    RICH_AVAILABLE = True
except ImportError:
    try:
        subprocess.check_call(
            [sys.executable, "-m", "pip", "install", "rich", "--quiet", "--user"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        from rich.console import Console
        from rich.table import Table
        from rich.panel import Panel
        from rich.tree import Tree
        from rich.text import Text
        from rich import box

        RICH_AVAILABLE = True
    except Exception:
        RICH_AVAILABLE = False


# ── MemoryDB ────────────────────────────────────────────────────────────────


class MemoryDB:
    """Read-only interface to the memory SQLite database."""

    def __init__(self, db_path: str):
        self.db_path = db_path
        if not os.path.isfile(db_path):
            raise FileNotFoundError(
                f"Database not found: {db_path}\n"
                "Run /turbo-index or /remember first to create it."
            )
        self.conn = sqlite3.connect(db_path)
        self.conn.row_factory = sqlite3.Row
        self.tables = self._ensure_tables_exist()

    def _ensure_tables_exist(self) -> dict:
        """Check which tables are present."""
        rows = self.conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table';"
        ).fetchall()
        names = {r["name"] for r in rows}
        return {
            "sessions": "sessions" in names,
            "knowledge": "knowledge" in names,
            "facts": "facts" in names,
            "entity_metadata": "entity_metadata" in names,
            "entry_relations": "entry_relations" in names,
            "memory_fts": "memory_fts" in names,
        }

    def _has_entities(self) -> bool:
        return self.tables.get("entity_metadata", False)

    def _has_relations(self) -> bool:
        return self.tables.get("entry_relations", False)

    # ── Stats ───────────────────────────────────────────────────────────

    def get_stats(self) -> dict:
        """Return row counts for each table."""
        stats = {}
        for tbl in ("sessions", "knowledge", "facts", "entity_metadata", "entry_relations"):
            if self.tables.get(tbl, False):
                row = self.conn.execute(f"SELECT COUNT(*) as c FROM {tbl}").fetchone()
                stats[tbl] = row["c"]
            else:
                stats[tbl] = 0
        return stats

    # ── Entities ────────────────────────────────────────────────────────

    def get_entities(self, limit: int = 50) -> dict:
        """Return entities grouped by type, with reference counts."""
        if not self._has_entities():
            return {}
        rows = self.conn.execute(
            """
            SELECT entity, entity_type, COUNT(*) as ref_count
            FROM entity_metadata
            GROUP BY entity, entity_type
            ORDER BY ref_count DESC
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
        grouped = defaultdict(list)
        for r in rows:
            grouped[r["entity_type"]].append(
                {"entity": r["entity"], "refs": r["ref_count"]}
            )
        return dict(grouped)

    def get_top_entities(self, limit: int = 15) -> list:
        """Return the most-referenced entities."""
        if not self._has_entities():
            return []
        rows = self.conn.execute(
            """
            SELECT entity, entity_type, COUNT(*) as ref_count
            FROM entity_metadata
            GROUP BY entity, entity_type
            ORDER BY ref_count DESC
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Relations ───────────────────────────────────────────────────────

    def get_relations(self) -> list:
        """Return entry_relations rows with human-readable labels."""
        if not self._has_relations():
            return []
        rows = self.conn.execute(
            """
            SELECT
                er.from_type, er.from_id, er.to_type, er.to_id, er.relation,
                CASE er.from_type
                    WHEN 'session'   THEN (SELECT SUBSTR(summary, 1, 60) FROM sessions WHERE id = er.from_id)
                    WHEN 'knowledge' THEN (SELECT area FROM knowledge WHERE id = er.from_id)
                    WHEN 'fact'      THEN (SELECT SUBSTR(fact, 1, 60) FROM facts WHERE id = er.from_id)
                END as from_label,
                CASE er.to_type
                    WHEN 'session'   THEN (SELECT SUBSTR(summary, 1, 60) FROM sessions WHERE id = er.to_id)
                    WHEN 'knowledge' THEN (SELECT area FROM knowledge WHERE id = er.to_id)
                    WHEN 'fact'      THEN (SELECT SUBSTR(fact, 1, 60) FROM facts WHERE id = er.to_id)
                END as to_label
            FROM entry_relations er
            ORDER BY er.created_at DESC
            LIMIT 100
            """
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Co-occurrences ──────────────────────────────────────────────────

    def get_co_occurrences(self, limit: int = 100) -> list:
        """Find implicit edges: entities that share the same source."""
        if not self._has_entities():
            return []
        rows = self.conn.execute(
            """
            SELECT
                a.entity as entity_a, a.entity_type as type_a,
                b.entity as entity_b, b.entity_type as type_b,
                COUNT(*) as shared_sources
            FROM entity_metadata a
            JOIN entity_metadata b
                ON a.source_type = b.source_type
                AND a.source_id = b.source_id
                AND a.id < b.id
            GROUP BY a.entity, b.entity
            HAVING shared_sources >= 2
            ORDER BY shared_sources DESC
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Timeline ────────────────────────────────────────────────────────

    def get_timeline(self, limit: int = 30) -> list:
        """Return sessions and knowledge entries ordered by date."""
        entries = []
        if self.tables.get("sessions"):
            rows = self.conn.execute(
                "SELECT id, created_at, summary, 'session' as type FROM sessions ORDER BY created_at DESC LIMIT ?",
                (limit,),
            ).fetchall()
            entries.extend(dict(r) for r in rows)
        if self.tables.get("knowledge"):
            rows = self.conn.execute(
                "SELECT id, updated_at as created_at, area || ': ' || summary as summary, 'knowledge' as type FROM knowledge ORDER BY updated_at DESC LIMIT ?",
                (limit,),
            ).fetchall()
            entries.extend(dict(r) for r in rows)
        entries.sort(key=lambda e: e.get("created_at", ""), reverse=True)
        return entries[:limit]

    # ── Entity Detail ───────────────────────────────────────────────────

    def get_entity_detail(self, entity_name: str) -> dict:
        """Drill into a specific entity: sources, relations, co-occurring."""
        detail = {"entity": entity_name, "sources": [], "relations": [], "co_occurring": []}
        if not self._has_entities():
            return detail

        # Sources
        rows = self.conn.execute(
            """
            SELECT em.entity_type, em.source_type, em.source_id,
                CASE em.source_type
                    WHEN 'session'   THEN (SELECT SUBSTR(summary, 1, 80) FROM sessions WHERE id = em.source_id)
                    WHEN 'knowledge' THEN (SELECT area || ': ' || SUBSTR(summary, 1, 60) FROM knowledge WHERE id = em.source_id)
                    WHEN 'fact'      THEN (SELECT SUBSTR(fact, 1, 80) FROM facts WHERE id = em.source_id)
                END as context
            FROM entity_metadata em
            WHERE em.entity LIKE ?
            ORDER BY em.created_at DESC
            """,
            (f"%{entity_name}%",),
        ).fetchall()
        detail["sources"] = [dict(r) for r in rows]

        # Relations involving this entity's sources
        if self._has_relations() and detail["sources"]:
            source_conds = " OR ".join(
                f"(from_type='{s['source_type']}' AND from_id={s['source_id']})"
                f" OR (to_type='{s['source_type']}' AND to_id={s['source_id']})"
                for s in detail["sources"]
            )
            if source_conds:
                rel_rows = self.conn.execute(
                    f"""
                    SELECT from_type, from_id, to_type, to_id, relation
                    FROM entry_relations
                    WHERE {source_conds}
                    LIMIT 20
                    """
                ).fetchall()
                detail["relations"] = [dict(r) for r in rel_rows]

        # Co-occurring entities
        co_rows = self.conn.execute(
            """
            SELECT DISTINCT b.entity, b.entity_type, COUNT(*) as shared
            FROM entity_metadata a
            JOIN entity_metadata b
                ON a.source_type = b.source_type
                AND a.source_id = b.source_id
                AND a.entity != b.entity
            WHERE a.entity LIKE ?
            GROUP BY b.entity, b.entity_type
            ORDER BY shared DESC
            LIMIT 20
            """,
            (f"%{entity_name}%",),
        ).fetchall()
        detail["co_occurring"] = [dict(r) for r in co_rows]
        return detail

    def close(self):
        self.conn.close()


# ── RichRenderer ────────────────────────────────────────────────────────────


class RichRenderer:
    """Renders views using the Rich library."""

    TYPE_COLORS = {
        "file": "cyan",
        "concept": "yellow",
        "package": "green",
        "function": "magenta",
        "session": "blue",
        "knowledge": "green",
        "fact": "yellow",
    }

    def __init__(self, db: MemoryDB):
        self.db = db
        self.console = Console()

    def _type_badge(self, type_name: str) -> Text:
        color = self.TYPE_COLORS.get(type_name, "white")
        return Text(f" {type_name} ", style=f"bold {color} on {color}", end="")

    def _type_text(self, type_name: str) -> Text:
        color = self.TYPE_COLORS.get(type_name, "white")
        return Text(type_name, style=f"bold {color}")

    # ── Graph View ──────────────────────────────────────────────────────

    def render_graph(self):
        self.console.rule("[bold cyan]Entity Relationship Graph[/]")
        self.console.print()

        entities = self.db.get_entities(limit=50)
        if not entities:
            self.console.print(
                Panel(
                    "[dim]No entities found.\nRun /remember to populate entity metadata.[/]",
                    title="Empty Graph",
                    border_style="dim",
                )
            )
            return

        # Entity tree
        tree = Tree("[bold]Entities[/]", guide_style="dim")
        for etype, items in sorted(entities.items()):
            color = self.TYPE_COLORS.get(etype, "white")
            branch = tree.add(f"[bold {color}]{etype}[/] ({len(items)})")
            for item in items[:15]:
                ref_label = f"[dim]x{item['refs']}[/]" if item["refs"] > 1 else ""
                branch.add(f"{item['entity']} {ref_label}")
            if len(items) > 15:
                branch.add(f"[dim]... and {len(items) - 15} more[/]")
        self.console.print(tree)
        self.console.print()

        # Relations table
        relations = self.db.get_relations()
        if relations:
            table = Table(
                title="Relations",
                box=box.ROUNDED,
                title_style="bold",
                show_lines=False,
            )
            table.add_column("From", style="cyan", max_width=35)
            table.add_column("Relation", style="yellow", justify="center")
            table.add_column("To", style="green", max_width=35)
            for rel in relations[:30]:
                from_lbl = rel.get("from_label") or f"{rel['from_type']}:{rel['from_id']}"
                to_lbl = rel.get("to_label") or f"{rel['to_type']}:{rel['to_id']}"
                table.add_row(
                    str(from_lbl)[:35],
                    rel["relation"],
                    str(to_lbl)[:35],
                )
            self.console.print(table)
        else:
            self.console.print("[dim]No explicit relations found.[/]")

        # Co-occurrences
        co_occ = self.db.get_co_occurrences(limit=20)
        if co_occ:
            self.console.print()
            table = Table(
                title="Co-occurrences (shared sources)",
                box=box.SIMPLE,
                title_style="bold",
            )
            table.add_column("Entity A", style="cyan")
            table.add_column("", justify="center")
            table.add_column("Entity B", style="green")
            table.add_column("Shared", justify="right", style="yellow")
            for co in co_occ[:20]:
                table.add_row(
                    co["entity_a"],
                    "<->",
                    co["entity_b"],
                    str(co["shared_sources"]),
                )
            self.console.print(table)

    # ── Timeline View ───────────────────────────────────────────────────

    def render_timeline(self):
        self.console.rule("[bold blue]Timeline[/]")
        self.console.print()

        entries = self.db.get_timeline(limit=30)
        if not entries:
            self.console.print(
                Panel(
                    "[dim]No timeline entries.\nRun /remember to record sessions.[/]",
                    title="Empty Timeline",
                    border_style="dim",
                )
            )
            return

        table = Table(box=box.ROUNDED, show_lines=False)
        table.add_column("Date", style="dim", width=19)
        table.add_column("Type", width=12, justify="center")
        table.add_column("Summary", ratio=1)

        for entry in entries:
            date_str = entry.get("created_at", "")[:19]
            etype = entry.get("type", "")
            color = self.TYPE_COLORS.get(etype, "white")
            type_text = Text(etype, style=f"bold {color}")
            summary = str(entry.get("summary", ""))[:100]
            table.add_row(date_str, type_text, summary)

        self.console.print(table)

        # Activity sparkline
        if entries:
            self._render_activity_sparkline(entries)

    def _render_activity_sparkline(self, entries: list):
        """Show a simple activity bar by date."""
        from collections import Counter

        dates = Counter()
        for e in entries:
            d = e.get("created_at", "")[:10]
            if d:
                dates[d] += 1
        if not dates:
            return
        sorted_dates = sorted(dates.items())
        max_count = max(dates.values())
        self.console.print()
        self.console.print("[bold]Activity[/]")
        bar_chars = " _.-=*#@"
        for date, count in sorted_dates[-14:]:
            idx = min(int(count / max(max_count, 1) * (len(bar_chars) - 1)), len(bar_chars) - 1)
            bar = bar_chars[idx] * count + bar_chars[idx] * (3 - count) if count < 4 else bar_chars[idx] * 4
            self.console.print(f"  [dim]{date}[/] [green]{bar}[/] {count}")

    # ── Stats View ──────────────────────────────────────────────────────

    def render_stats(self):
        self.console.rule("[bold green]Statistics[/]")
        self.console.print()

        stats = self.db.get_stats()

        # Counts panel
        counts_table = Table(box=box.SIMPLE, show_header=False, padding=(0, 2))
        counts_table.add_column("Table", style="bold")
        counts_table.add_column("Count", justify="right", style="cyan")
        table_labels = {
            "sessions": "Sessions",
            "knowledge": "Knowledge Areas",
            "facts": "Facts",
            "entity_metadata": "Entity Refs",
            "entry_relations": "Relations",
        }
        for tbl, label in table_labels.items():
            counts_table.add_row(label, str(stats.get(tbl, 0)))

        self.console.print(Panel(counts_table, title="Counts", border_style="green", width=40))
        self.console.print()

        # Entity categories
        entities = self.db.get_entities(limit=100)
        if entities:
            cat_table = Table(box=box.SIMPLE, show_header=False, padding=(0, 2))
            cat_table.add_column("Type", style="bold")
            cat_table.add_column("Count", justify="right", style="yellow")
            for etype, items in sorted(entities.items()):
                color = self.TYPE_COLORS.get(etype, "white")
                cat_table.add_row(Text(etype, style=f"bold {color}"), str(len(items)))
            self.console.print(
                Panel(cat_table, title="Entity Categories", border_style="yellow", width=40)
            )
            self.console.print()

        # Top entities bar chart
        top = self.db.get_top_entities(limit=15)
        if top:
            self.console.print("[bold]Top Entities[/]")
            max_refs = max(e["ref_count"] for e in top) if top else 1
            for e in top:
                bar_len = max(1, int(e["ref_count"] / max(max_refs, 1) * 30))
                color = self.TYPE_COLORS.get(e["entity_type"], "white")
                bar = Text("=" * bar_len, style=color)
                name = Text(f" {e['entity']}", style="bold")
                count = Text(f" ({e['ref_count']})", style="dim")
                line = Text()
                line.append(bar)
                line.append(name)
                line.append(count)
                self.console.print(f"  ", end="")
                self.console.print(line)
        elif sum(stats.values()) == 0:
            self.console.print(
                Panel(
                    "[dim]No data yet. Run /remember to start building the knowledge graph.[/]",
                    border_style="dim",
                )
            )

    # ── Explore View ────────────────────────────────────────────────────

    def render_explore(self, entity_name: str):
        self.console.rule(f"[bold magenta]Explore: {entity_name}[/]")
        self.console.print()

        detail = self.db.get_entity_detail(entity_name)
        if not detail["sources"] and not detail["relations"] and not detail["co_occurring"]:
            self.console.print(
                Panel(
                    f"[dim]No data found for entity matching '{entity_name}'.\n"
                    "Try a different name or check /memory-stats.[/]",
                    border_style="dim",
                )
            )
            return

        tree = Tree(f"[bold]{entity_name}[/]", guide_style="dim")

        # Sources
        if detail["sources"]:
            sources_branch = tree.add("[bold cyan]Appears in[/]")
            for s in detail["sources"]:
                ctx = s.get("context") or f"{s['source_type']}:{s['source_id']}"
                color = self.TYPE_COLORS.get(s["source_type"], "white")
                sources_branch.add(
                    f"[{color}]{s['source_type']}[/] #{s['source_id']}: {ctx}"
                )

        # Relations
        if detail["relations"]:
            rel_branch = tree.add("[bold yellow]Relations[/]")
            for r in detail["relations"]:
                rel_branch.add(
                    f"{r['from_type']}:{r['from_id']} "
                    f"--[{r['relation']}]--> "
                    f"{r['to_type']}:{r['to_id']}"
                )

        # Co-occurring
        if detail["co_occurring"]:
            co_branch = tree.add("[bold green]Co-occurs with[/]")
            for c in detail["co_occurring"]:
                color = self.TYPE_COLORS.get(c["entity_type"], "white")
                shared = f"[dim](shared {c['shared']})[/]"
                co_branch.add(f"[{color}]{c['entity']}[/] {shared}")

        self.console.print(tree)

    # ── Full View ───────────────────────────────────────────────────────

    def render_full(self):
        self.console.print()
        self.console.print(
            Panel(
                "[bold]Knowledge Graph Viewer[/]\n[dim]Memory database TUI[/]",
                border_style="bright_blue",
                width=50,
            )
        )
        self.console.print()
        self.render_stats()
        self.console.print()
        self.render_graph()
        self.console.print()
        self.render_timeline()


# ── PlainRenderer ───────────────────────────────────────────────────────────


class PlainRenderer:
    """Fallback renderer using print() and box-drawing characters."""

    def __init__(self, db: MemoryDB):
        self.db = db

    @staticmethod
    def _box(title: str, lines: list, width: int = 60):
        print(f"\n+-{'-' * (width - 4)}-+")
        print(f"| {title:<{width - 4}} |")
        print(f"+-{'-' * (width - 4)}-+")
        for line in lines:
            print(f"| {line:<{width - 4}} |")
        print(f"+-{'-' * (width - 4)}-+")

    @staticmethod
    def _table(headers: list, rows: list, col_widths: list = None):
        if not col_widths:
            col_widths = [max(len(str(h)), max((len(str(r[i])) for r in rows), default=4)) + 2 for i, h in enumerate(headers)]
        header_line = "  ".join(str(h).ljust(w) for h, w in zip(headers, col_widths))
        print(f"  {header_line}")
        print(f"  {'  '.join('-' * w for w in col_widths)}")
        for row in rows:
            line = "  ".join(str(row[i])[:w].ljust(w) for i, w in enumerate(col_widths))
            print(f"  {line}")

    # ── Graph ───────────────────────────────────────────────────────────

    def render_graph(self):
        print("\n=== Entity Relationship Graph ===\n")
        entities = self.db.get_entities(limit=50)
        if not entities:
            print("  (No entities found. Run /remember to populate.)")
            return

        for etype, items in sorted(entities.items()):
            print(f"  [{etype}] ({len(items)})")
            for item in items[:15]:
                refs = f" x{item['refs']}" if item["refs"] > 1 else ""
                print(f"    - {item['entity']}{refs}")
            if len(items) > 15:
                print(f"    ... and {len(items) - 15} more")
            print()

        relations = self.db.get_relations()
        if relations:
            print("  Relations:")
            self._table(
                ["From", "Relation", "To"],
                [
                    (
                        str(r.get("from_label") or f"{r['from_type']}:{r['from_id']}")[:30],
                        r["relation"],
                        str(r.get("to_label") or f"{r['to_type']}:{r['to_id']}")[:30],
                    )
                    for r in relations[:30]
                ],
                [30, 15, 30],
            )

        co_occ = self.db.get_co_occurrences(limit=20)
        if co_occ:
            print("\n  Co-occurrences:")
            self._table(
                ["Entity A", "", "Entity B", "Shared"],
                [(c["entity_a"][:25], "<->", c["entity_b"][:25], str(c["shared_sources"])) for c in co_occ],
                [25, 5, 25, 8],
            )

    # ── Timeline ────────────────────────────────────────────────────────

    def render_timeline(self):
        print("\n=== Timeline ===\n")
        entries = self.db.get_timeline(limit=30)
        if not entries:
            print("  (No timeline entries. Run /remember to record sessions.)")
            return

        self._table(
            ["Date", "Type", "Summary"],
            [
                (
                    e.get("created_at", "")[:19],
                    e.get("type", ""),
                    str(e.get("summary", ""))[:60],
                )
                for e in entries
            ],
            [19, 12, 60],
        )

    # ── Stats ───────────────────────────────────────────────────────────

    def render_stats(self):
        print("\n=== Statistics ===\n")
        stats = self.db.get_stats()
        labels = {
            "sessions": "Sessions",
            "knowledge": "Knowledge Areas",
            "facts": "Facts",
            "entity_metadata": "Entity Refs",
            "entry_relations": "Relations",
        }
        lines = [f"{label}: {stats.get(tbl, 0)}" for tbl, label in labels.items()]
        self._box("Counts", lines, 40)

        entities = self.db.get_entities(limit=100)
        if entities:
            cat_lines = [f"{etype}: {len(items)}" for etype, items in sorted(entities.items())]
            self._box("Entity Categories", cat_lines, 40)

        top = self.db.get_top_entities(limit=15)
        if top:
            print("\n  Top Entities:")
            max_refs = max(e["ref_count"] for e in top)
            for e in top:
                bar_len = max(1, int(e["ref_count"] / max(max_refs, 1) * 25))
                bar = "=" * bar_len
                print(f"  {bar} {e['entity']} ({e['ref_count']})")
        elif sum(stats.values()) == 0:
            print("  (No data yet. Run /remember to start building the graph.)")

    # ── Explore ─────────────────────────────────────────────────────────

    def render_explore(self, entity_name: str):
        print(f"\n=== Explore: {entity_name} ===\n")
        detail = self.db.get_entity_detail(entity_name)
        if not detail["sources"] and not detail["relations"] and not detail["co_occurring"]:
            print(f"  (No data found for '{entity_name}'.)")
            return

        if detail["sources"]:
            print(f"  Appears in:")
            for s in detail["sources"]:
                ctx = s.get("context") or f"{s['source_type']}:{s['source_id']}"
                print(f"    [{s['source_type']}] #{s['source_id']}: {ctx}")

        if detail["relations"]:
            print(f"\n  Relations:")
            for r in detail["relations"]:
                print(
                    f"    {r['from_type']}:{r['from_id']} "
                    f"--[{r['relation']}]--> "
                    f"{r['to_type']}:{r['to_id']}"
                )

        if detail["co_occurring"]:
            print(f"\n  Co-occurs with:")
            for c in detail["co_occurring"]:
                print(f"    {c['entity']} [{c['entity_type']}] (shared {c['shared']})")

    # ── Full ────────────────────────────────────────────────────────────

    def render_full(self):
        print()
        print("+" + "=" * 48 + "+")
        print("|   Knowledge Graph Viewer                       |")
        print("|   Memory database TUI                          |")
        print("+" + "=" * 48 + "+")
        self.render_stats()
        self.render_graph()
        self.render_timeline()


# ── CLI ─────────────────────────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(
        description="Knowledge Graph TUI Viewer for Claude memory database.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""\
examples:
  %(prog)s --db memory.db                  Show all views
  %(prog)s --db memory.db graph            Entity graph
  %(prog)s --db memory.db timeline         Session timeline
  %(prog)s --db memory.db stats            Statistics dashboard
  %(prog)s --db memory.db explore auth     Drill into 'auth' entity
""",
    )
    parser.add_argument(
        "--db",
        required=True,
        help="Path to the SQLite memory database",
    )
    parser.add_argument(
        "command",
        nargs="?",
        default="full",
        choices=["full", "graph", "timeline", "stats", "explore"],
        help="View to display (default: full)",
    )
    parser.add_argument(
        "entity",
        nargs="?",
        default=None,
        help="Entity name for explore command",
    )
    parser.add_argument(
        "--plain",
        action="store_true",
        help="Force plain-text output (no Rich)",
    )

    args = parser.parse_args()

    # Validate explore requires entity
    if args.command == "explore" and not args.entity:
        parser.error("explore command requires an entity name")

    try:
        db = MemoryDB(args.db)
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

    # Check entity_metadata availability
    if not db.tables.get("entity_metadata") and args.command in ("graph", "explore"):
        print(
            "Entity metadata not initialized.\n"
            "Run `memory-db.sh init-metadata` to create entity tables.",
            file=sys.stderr,
        )
        sys.exit(1)

    # Pick renderer
    if args.plain or not RICH_AVAILABLE:
        renderer = PlainRenderer(db)
    else:
        renderer = RichRenderer(db)

    try:
        if args.command == "graph":
            renderer.render_graph()
        elif args.command == "timeline":
            renderer.render_timeline()
        elif args.command == "stats":
            renderer.render_stats()
        elif args.command == "explore":
            renderer.render_explore(args.entity)
        else:
            renderer.render_full()
    finally:
        db.close()


if __name__ == "__main__":
    main()
