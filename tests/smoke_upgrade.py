#!/usr/bin/env python3
"""Build an explicitly selected historical CLI and verify a real database upgrade."""
import argparse
import os
from pathlib import Path
import sqlite3
import subprocess
import tempfile

from smoke_clients import ROOT, run


def snapshot(database):
    with sqlite3.connect(database) as connection:
        return {table: connection.execute(f'SELECT * FROM {table} ORDER BY rowid').fetchall()
                for table in ('sessions', 'knowledge', 'facts', 'entity_metadata', 'entry_relations',
                              'memory_fts', 'vector_meta', 'embedding_queue', 'token_metrics')}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline', required=True, help='historical commit to build, e.g. 1f70bcc')
    args = parser.parse_args()
    baseline = run(['git', 'rev-parse', '--verify', '--end-of-options', args.baseline + '^{commit}'], os.environ, ROOT).strip()
    with tempfile.TemporaryDirectory(prefix='turbo-upgrade-') as temporary:
        base = Path(temporary).resolve()
        source = base / 'baseline'; source.mkdir()
        names = run(['git', 'ls-tree', '-r', '--name-only', baseline, '--', 'memory'], os.environ, ROOT).splitlines()
        for name in names:
            destination = source / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(subprocess.check_output(['git', 'show', baseline + ':' + name], cwd=ROOT))
            if destination.suffix == '.sh':
                destination.chmod(0o700)
        run(['go', 'build', '-o', source / 'memory/memorydb', './cmd/memorydb'], os.environ, source / 'memory', timeout=90)
        run(['bash', ROOT / 'scripts/build-memory.sh'], os.environ, ROOT, timeout=90)
        repo = base / 'repo'; repo.mkdir()
        home = base / 'home'; home.mkdir()
        env = dict(os.environ, HOME=str(home), MEMORY_REPO_ROOT='')
        run(['git', 'init', '-q', repo], env, repo)
        old = ['bash', source / 'memory/memory-db.sh']
        new = ['bash', ROOT / 'memory/memory-db.sh']
        for command in (['init'], ['init-metadata'], ['init-vector'], ['init-token-metrics'],
                        ['add-session', 'upgradeunique completed authentication', '["src/auth.go"]', '["Read"]', 'authentication'],
                        ['add-knowledge', 'src/auth', 'AuthService validates requests', 'Preserve callback state'],
                        ['add-fact', 'upgradeunique uses PKCE', 'decision'], ['add-token-metrics', '1', '2', '3', '1']):
            run(old + command, env, repo)
        database = repo / '.claude-memory/memory.db'
        with sqlite3.connect(database) as connection:
            connection.execute("UPDATE knowledge SET embedding=X'0000803F00000000'")
            connection.execute("INSERT INTO entry_relations(from_type,from_id,to_type,to_id,relation) VALUES ('session',1,'fact',1,'related_to')")
        before = snapshot(database)
        for _ in range(2):
            run(new + ['init'], env, repo)
            run(new + ['init-metadata'], env, repo)
        assert snapshot(database) == before, 'upgrade changed existing rows, IDs, indexes, or embeddings'
        with sqlite3.connect(database) as connection:
            assert connection.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
        assert 'upgradeunique' in run(new + ['query', 'upgradeunique'], env, repo)
        assert 'upgradeunique' in run(old + ['search', 'upgradeunique'], env, repo)
        destination = base / 'memory.md'
        run(new + ['export', destination], env, repo)
        assert '## session:1' in destination.read_text() and 'upgradeunique' in destination.read_text()
        print(f'Historical CLI {baseline[:7]} → current: repeated upgrade, row/ID/embedding preservation, integrity, old/new reads and export PASS')


if __name__ == '__main__':
    main()
