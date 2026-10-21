#!/usr/bin/env python3
"""Opt-in installed-client/QMD smoke test, isolated from all personal configuration."""
import importlib.util
import json
import os
from pathlib import Path
import selectors
import shutil
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('turbo', ROOT / 'scripts/turbo.py')
turbo = importlib.util.module_from_spec(spec); spec.loader.exec_module(turbo)


def run(args, env, cwd, timeout=45):
    result = subprocess.run([str(a) for a in args], env=env, cwd=cwd, capture_output=True, text=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError(f'{args}:\n{result.stderr}\n{result.stdout}')
    return result.stdout


def request(proc, method, params, request_id):
    proc.stdin.write((json.dumps({'jsonrpc': '2.0', 'id': request_id, 'method': method, 'params': params}) + '\n').encode())
    proc.stdin.flush()
    deadline = time.monotonic() + 20
    with selectors.DefaultSelector() as selector:
        selector.register(proc.stdout, selectors.EVENT_READ)
        while time.monotonic() < deadline:
            if not selector.select(max(0, deadline - time.monotonic())):
                break
            line = proc.stdout.readline()
            if not line:
                break
            message = json.loads(line)
            if message.get('id') == request_id:
                assert 'error' not in message, message
                return message['result']
    raise RuntimeError(f'MCP response timeout: {method}')


def main():
    for client in ('claude', 'codex', 'qmd'):
        if not shutil.which(client):
            raise RuntimeError(f'{client} must be installed for this opt-in check')
    with tempfile.TemporaryDirectory(prefix='turbo-smoke-') as temporary:
        base = Path(temporary)
        env = dict(os.environ, HOME=str(base / 'home'), CODEX_HOME=str(base / 'codex'),
                   CLAUDE_CONFIG_DIR=str(base / 'claude'), XDG_CONFIG_HOME=str(base / 'config'),
                   XDG_CACHE_HOME=str(base / 'cache'), MEMORY_REPO_ROOT='')
        for name in ('HOME', 'CODEX_HOME', 'CLAUDE_CONFIG_DIR', 'XDG_CONFIG_HOME', 'XDG_CACHE_HOME'):
            Path(env[name]).mkdir(parents=True)
        first = base / 'one' / 'project'; first.mkdir(parents=True)
        second = base / 'two' / 'project'; second.mkdir(parents=True)
        for repo, marker in ((first, 'turboalphaunique'), (second, 'turbobetaunique')):
            run(['git', 'init', '-q'], env, repo)
            (repo / 'README.md').write_text(f'# Reference\n\n{marker} authentication details.\n')
            print(run(['python3', ROOT / 'scripts/turbo.py', 'index'], env, repo).strip())
            output = json.loads(run(['python3', ROOT / 'scripts/turbo.py', 'search', marker], env, repo))
            assert output and output[0]['source'] == 'qmd', output
        for client in ('claude', 'codex'):
            print(run([client, '--version'], env, first).strip())
        print(run(['bash', ROOT / 'scripts/setup-mcp.sh', '--client', 'both'], env, first).strip())
        print(run(['bash', ROOT / 'scripts/setup-mcp.sh', '--client', 'both'], env, first).strip())
        for client in ('claude', 'codex'):
            registered = run([client, 'mcp', 'get', 'qmd'], env, first)
            assert 'qmd-mcp.sh' in registered, registered
        run(['bash', ROOT / 'scripts/setup-hooks.sh', '--client', 'both', '--rag', '--memory'], env, first)
        with tempfile.TemporaryFile() as errors:
            proc = subprocess.Popen(['bash', str(ROOT / 'scripts/qmd-mcp.sh')], env=env, cwd=first,
                                    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors, bufsize=0)
            try:
                request(proc, 'initialize', {'protocolVersion': '2024-11-05', 'capabilities': {},
                        'clientInfo': {'name': 'turbo-smoke', 'version': '1'}}, 1)
                proc.stdin.write(b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n'); proc.stdin.flush()
                tools = request(proc, 'tools/list', {}, 2)['tools']
                print('MCP tools:', ', '.join(t['name'] for t in tools))
                search = next((t for t in tools if t['name'] == 'search'), None)
                if search is None:
                    search = next(t for t in tools if t['name'] == 'query')
                    assert 'searches' in search['inputSchema']['properties'], search
                def arguments(query):
                    return ({'query': query} if search['name'] == 'search' else
                            {'searches': [{'type': 'lex', 'query': query}], 'rerank': False})
                result = request(proc, 'tools/call', {'name': search['name'], 'arguments': arguments('turboalphaunique')}, 3)
                assert not result.get('isError'), result
                assert 'turboalphaunique' in json.dumps(result).lower(), result
                other = request(proc, 'tools/call', {'name': search['name'], 'arguments': arguments('turbobetaunique')}, 4)
                assert not other.get('isError'), other
                assert 'turbobetaunique authentication' not in json.dumps(other).lower(), other
                print('MCP initialize, tools/list, search and repository isolation: PASS')
            finally:
                proc.terminate()
                try: proc.wait(timeout=5)
                except subprocess.TimeoutExpired: proc.kill(); proc.wait()
                proc.stdin.close(); proc.stdout.close()
        # Validate a clean package, excluding ignored memory/cache/private local files.
        package = base / 'package'; package.mkdir()
        tracked = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT)
        for name in tracked.decode().split('\0'):
            if not name or not (ROOT / name).is_file(): continue
            destination = package / name; destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / name, destination)
        validation = json.loads(run(['claude', 'plugin', 'validate', package, '--json'], env, first))
        assert validation['success'], validation
        print('Claude plugin validation: PASS')
        print(run(['codex', 'plugin', 'marketplace', 'add', package, '--json'], env, first).strip())
        print(run(['codex', 'plugin', 'add', 'turbo-search@claude-turbo-search-dev', '--json'], env, first).strip())
        print('Codex local plugin installation: PASS')


if __name__ == '__main__':
    main()
