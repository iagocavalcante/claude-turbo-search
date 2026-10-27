#!/usr/bin/env python3
"""Installed-client lifecycle checks in temporary profiles; no real model requests."""
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import threading
import time
import uuid

from smoke_clients import ROOT, request, run


def seed_activity(repo, session, file='src/native-smoke.go'):
    directory = repo / '.claude-memory/activity'
    directory.mkdir(exist_ok=True)
    path = directory / (hashlib.sha256(session.encode()).hexdigest() + '.jsonl')
    path.write_text(json.dumps({'time': '2026-10-27T00:00:00Z', 'tool': 'Read',
                                'files': [file]}) + '\n')


def checkpoints(repo):
    with sqlite3.connect(repo / '.claude-memory/memory.db') as connection:
        return connection.execute('SELECT source_agent,source_session,capture_event FROM checkpoints').fetchall()


def codex_lifecycle(env, repo):
    home = Path(env['CODEX_HOME'])
    (home / 'config.toml').write_text(
        'model_provider = "fixture"\nmodel = "fixture-model"\n'
        '[model_providers.fixture]\nname = "Local fixture"\n'
        'base_url = "http://127.0.0.1:9/v1"\nwire_api = "responses"\n'
        '[features]\nhooks = true\n[projects.' + json.dumps(str(repo)) + ']\ntrust_level = "trusted"\n')
    with tempfile.TemporaryFile() as errors:
        proc = subprocess.Popen(['codex', 'app-server', '--stdio'], env=env, cwd=repo,
                                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors, bufsize=0)
        try:
            request(proc, 'initialize', {'clientInfo': {'name': 'turbo_lifecycle_smoke', 'version': '1'},
                    'capabilities': {'experimentalApi': True}}, 1)
            proc.stdin.write(b'{"method":"initialized"}\n'); proc.stdin.flush()
            hooks = request(proc, 'hooks/list', {'cwds': [str(repo)]}, 2)['data'][0]['hooks']
            assert hooks and all(h['trustStatus'] == 'untrusted' for h in hooks), hooks
            expected = json.loads((home / 'hooks.json').read_text())['hooks']
            commands = {hook['command'] for groups in expected.values() for group in groups for hook in group['hooks']}
            assert {h['command'] for h in hooks} == commands
            assert all(Path(h['sourcePath']).resolve() == (home / 'hooks.json').resolve() for h in hooks)
            # Trust only this test's exact generated definitions, never personal hook sources.
            state = {h['key']: {'enabled': True, 'trusted_hash': h['currentHash']} for h in hooks}
            request(proc, 'config/batchWrite', {'edits': [{'keyPath': 'hooks.state', 'value': state,
                    'mergeStrategy': 'replace'}], 'reloadUserConfig': True}, 3)
            trusted = request(proc, 'hooks/list', {'cwds': [str(repo)]}, 4)['data'][0]['hooks']
            assert all(h['trustStatus'] == 'trusted' for h in trusted), trusted
            started = request(proc, 'thread/start', {'cwd': str(repo), 'experimentalRawEvents': False}, 5)
            thread = started['thread']['id']
            seed_activity(repo, thread)
            request(proc, 'thread/compact/start', {'threadId': thread}, 6)
            deadline = time.monotonic() + 5
            while ('codex', thread, 'PreCompact') not in checkpoints(repo) and time.monotonic() < deadline:
                time.sleep(0.05)
            assert ('codex', thread, 'PreCompact') in checkpoints(repo), checkpoints(repo)
            print('Codex native PreCompact dispatch (provider restricted to loopback): PASS', flush=True)
            seed_activity(repo, thread, 'src/end-smoke.go')
            request(proc, 'thread/archive', {'threadId': thread}, 7)
            assert ('codex', thread, 'SessionEnd') in checkpoints(repo), checkpoints(repo)
            print('Codex hook discovery, exact-definition trust and native SessionEnd dispatch: PASS', flush=True)
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill(); proc.wait()
            proc.stdin.close(); proc.stdout.close()


class LocalMessages(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        payload = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        self.server.requests.append(self.path)
        message = {'id': 'msg_fixture', 'type': 'message', 'role': 'assistant',
                   'model': payload.get('model', 'fixture-model'),
                   'content': [{'type': 'text', 'text': 'Lifecycle smoke complete.'}],
                   'stop_reason': 'end_turn', 'stop_sequence': None,
                   'usage': {'input_tokens': 1, 'output_tokens': 1}}
        if not payload.get('stream'):
            body = json.dumps(message).encode()
            self.send_response(200); self.send_header('Content-Type', 'application/json')
        else:
            events = [
                {'type': 'message_start', 'message': dict(message, content=[], stop_reason=None)},
                {'type': 'content_block_start', 'index': 0, 'content_block': {'type': 'text', 'text': ''}},
                {'type': 'content_block_delta', 'index': 0, 'delta': {'type': 'text_delta', 'text': 'Lifecycle smoke complete.'}},
                {'type': 'content_block_stop', 'index': 0},
                {'type': 'message_delta', 'delta': {'stop_reason': 'end_turn', 'stop_sequence': None}, 'usage': {'output_tokens': 1}},
                {'type': 'message_stop'},
            ]
            body = ''.join('event: ' + event['type'] + '\ndata: ' + json.dumps(event) + '\n\n' for event in events).encode()
            self.send_response(200); self.send_header('Content-Type', 'text/event-stream')
        self.send_header('Content-Length', str(len(body))); self.end_headers()
        self.wfile.write(body)


def claude_lifecycle(env, repo):
    session = str(uuid.uuid4())
    seed_activity(repo, session)
    server = ThreadingHTTPServer(('127.0.0.1', 0), LocalMessages)
    server.requests = []
    thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
    isolated = {k: v for k, v in env.items() if not k.startswith(('ANTHROPIC_', 'CLAUDE_CODE_USE_', 'CLAUDE_CODE_OAUTH'))}
    isolated.update(ANTHROPIC_API_KEY='local-fixture-not-a-real-key',
                    ANTHROPIC_BASE_URL=f'http://127.0.0.1:{server.server_port}',
                    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC='1')
    try:
        options = ['--model', 'fixture-model', '--tools', '', '--strict-mcp-config',
                   '--mcp-config', '{"mcpServers":{}}', '--setting-sources', 'user', '--output-format', 'json']
        output = run(['claude', '-p', 'Run the lifecycle smoke check.', '--session-id', session,
                      *options], isolated, repo, timeout=45)
        result = json.loads(output)
        assert not result.get('is_error'), result
        assert server.requests, 'Claude did not use the local response fixture'
        assert ('claude', session, 'SessionEnd') in checkpoints(repo), checkpoints(repo)
        print('Claude native SessionEnd dispatch with local response fixture: PASS', flush=True)
        seed_activity(repo, session, 'src/compact-smoke.go')
        output = run(['claude', '-p', '/compact', '--resume', session, *options], isolated, repo, timeout=45)
        assert not json.loads(output).get('is_error'), output
        assert ('claude', session, 'PreCompact') in checkpoints(repo), checkpoints(repo)
        print('Claude native PreCompact dispatch with local response fixture: PASS', flush=True)
    finally:
        server.shutdown(); server.server_close(); thread.join(timeout=5)


def main():
    run(['bash', ROOT / 'scripts/build-memory.sh'], os.environ, ROOT)
    with tempfile.TemporaryDirectory(prefix='turbo-lifecycle-') as temporary:
        base = Path(temporary).resolve()
        repo = base / 'repo'; repo.mkdir()
        inherited = {k: v for k, v in os.environ.items()
                     if not k.startswith(('OPENAI_', 'ANTHROPIC_', 'CLAUDE_CODE_', 'CODEX_'))}
        env = dict(inherited, HOME=str(base / 'home'), CODEX_HOME=str(base / 'codex'),
                   CLAUDE_CONFIG_DIR=str(base / 'claude'), XDG_CONFIG_HOME=str(base / 'config'),
                   XDG_CACHE_HOME=str(base / 'cache'), MEMORY_REPO_ROOT='')
        for name in ('HOME', 'CODEX_HOME', 'CLAUDE_CONFIG_DIR', 'XDG_CONFIG_HOME', 'XDG_CACHE_HOME'):
            Path(env[name]).mkdir()
        run(['git', 'init', '-q', repo], env, repo)
        run(['bash', ROOT / 'memory/memory-db.sh', 'init'], env, repo)
        run(['bash', ROOT / 'scripts/setup-hooks.sh', '--client', 'both', '--memory'], env, repo)
        codex_lifecycle(env, repo)
        claude_lifecycle(env, repo)


if __name__ == '__main__':
    main()
