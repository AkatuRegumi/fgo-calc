import argparse
import json
import os
import shutil
import subprocess
import sys
import time
import urllib.request

ROOT = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.join(ROOT, 'data')
META_PATH = os.path.join(DATA, 'source_meta.json')
ATLAS_INFO = 'https://api.atlasacademy.io/info'

CHALDEA_REPO = 'https://github.com/chaldea-center/chaldea-data'

GENERATED_FILES = [
    'servants.json', 'ces.json', 'cn.json', 'cn_unavailable.txt',
    'ce_sync_report.json', 'cn_sync_report.json', 'servant_sync_report.json',
    'source_meta.json',
    os.path.join('names', 'traits.json'),
    os.path.join('names', 'ce.json'),
    os.path.join('names', 'servant.json'),
    os.path.join('names', 'costume.json'),
]

def snapshot_generated_files():
    snapshot = {}
    for rel in GENERATED_FILES:
        path = os.path.join(DATA, rel)
        if os.path.isfile(path):
            with open(path, 'rb') as f:
                snapshot[rel] = f.read()
        else:
            snapshot[rel] = None
    return snapshot

def snapshot_json_count(snapshot, rel):
    raw = snapshot.get(rel)
    if not raw:
        return 0
    try:
        value = json.loads(raw.decode('utf-8-sig'))
        return len(value) if isinstance(value, list) else 0
    except Exception:
        return 0

def restore_generated_files(snapshot):
    for rel, content in snapshot.items():
        path = os.path.join(DATA, rel)
        if content is None:
            try:
                os.remove(path)
            except FileNotFoundError:
                pass
            continue
        os.makedirs(os.path.dirname(path), exist_ok=True)
        tmp = path + '.restore-tmp'
        with open(tmp, 'wb') as f:
            f.write(content)
        os.replace(tmp, path)


def git_output(args, timeout=20):
    if shutil.which('git') is None:
        return None
    try:
        p = subprocess.run(
            ['git', '-c', 'http.version=HTTP/1.1', *args],
            cwd=ROOT,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding='utf-8',
            errors='replace',
            timeout=timeout,
        )
        if p.returncode == 0:
            value = p.stdout.strip()
            return value or None
    except (OSError, subprocess.TimeoutExpired):
        pass
    return None


def remote_chaldea_head():
    raw = git_output(['ls-remote', CHALDEA_REPO, 'HEAD'])
    if not raw:
        return None
    return raw.split()[0] if raw.split() else None


def local_chaldea_head():
    repo_dir = os.path.join(DATA, 'chaldea-data')
    if not os.path.isdir(repo_dir):
        return None
    return git_output(['-C', repo_dir, 'rev-parse', 'HEAD'])


def fetch_atlas_info(timeout=5):
    req = urllib.request.Request(ATLAS_INFO, headers={'User-Agent': 'fgo-calc-local-box-data-sync/1.0'})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode('utf-8'))


def try_fetch_atlas_info(timeout=5):
    try:
        return fetch_atlas_info(timeout=timeout), None
    except Exception as exc:
        return None, str(exc)


def load_json(path, default=None):
    try:
        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return default


def stable_jp_info(info):
    if isinstance(info, dict):
        for key in ('JP', 'jp'):
            if key in info:
                return info[key]
        # Some API versions expose a regions list/object under another key.
        for key in ('regions', 'data'):
            value = info.get(key)
            if isinstance(value, dict):
                for region_key in ('JP', 'jp'):
                    if region_key in value:
                        return value[region_key]
    return info


def fingerprint(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'))


def status_payload(remote_info, remote_chaldea=None, atlas_error=None):
    local = load_json(META_PATH, {}) or {}
    local_jp = local.get('atlasJP')
    remote_jp = stable_jp_info(remote_info) if remote_info is not None else None
    local_chaldea = local.get('chaldeaCommit') or local_chaldea_head()
    # Chaldea Data is the actual generator source. Prefer its commit as the
    # authoritative update gate so an Atlas release that lands a little earlier
    # cannot be marked as "synced" before Chaldea Data catches up.
    if remote_chaldea:
        update_available = not local_chaldea or local_chaldea != remote_chaldea
        update_signal = 'chaldea-commit'
    elif remote_jp is not None:
        # Git/network unavailable during a lightweight check: Atlas is a useful
        # fallback signal, but it is intentionally optional.
        update_available = local_jp is None or fingerprint(local_jp) != fingerprint(remote_jp)
        update_signal = 'atlas-fallback'
    else:
        # Both update probes are unavailable. Keep the local dataset usable and
        # report an offline status instead of failing the whole application.
        update_available = False
        update_signal = 'offline'
    return {
        'ok': True,
        'source': 'Atlas Academy + Chaldea Data',
        'connected': bool(remote_chaldea or remote_jp is not None),
        'atlasAvailable': remote_jp is not None,
        'atlasError': atlas_error or '',
        'hasLocalSourceMeta': bool(local),
        'updateAvailable': update_available,
        'updateSignal': update_signal,
        'lastSyncedAt': int(local.get('syncedAt', 0) or 0),
        'atlasJP': remote_jp,
        'localAtlasJP': local_jp,
        'chaldeaCommit': local_chaldea,
        'remoteChaldeaCommit': remote_chaldea,
        'ceSync': load_json(os.path.join(DATA, 'ce_sync_report.json'), {}) or {},
        'cnSync': load_json(os.path.join(DATA, 'cn_sync_report.json'), {}) or {},
        'servantSync': load_json(os.path.join(DATA, 'servant_sync_report.json'), {}) or {},
    }


def run_step(script):
    env = os.environ.copy()
    env['PYTHONUTF8'] = '1'
    env['PYTHONIOENCODING'] = 'utf-8'
    p = subprocess.run(
        [sys.executable, os.path.join(ROOT, script)],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding='utf-8',
        errors='replace',
        env=env,
    )
    log = p.stdout or ''
    log_name = os.path.join(DATA, f'data_sync_{os.path.splitext(script)[0]}.log')
    try:
        with open(log_name, 'w', encoding='utf-8') as f:
            f.write(log)
    except OSError:
        pass
    if p.returncode != 0:
        lines = log.splitlines()
        tail = '\n'.join(lines[-120:])
        raise RuntimeError(
            f'{script} failed (exit {p.returncode}). '
            f'Full log: {log_name}\n{tail}'
        )
    return log


def atomic_json(path, data):
    tmp = path + '.tmp'
    with open(tmp, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    os.replace(tmp, path)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--check', action='store_true')
    ap.add_argument('--json', action='store_true')
    args = ap.parse_args()

    try:
        remote, atlas_error = try_fetch_atlas_info()
        remote_chaldea = remote_chaldea_head()
        if args.check:
            payload = status_payload(remote, remote_chaldea, atlas_error)
        else:
            if shutil.which('git') is None:
                raise RuntimeError('Git was not found in PATH. Data sync currently uses Chaldea Data git as the compact generator source.')
            snapshot = snapshot_generated_files()
            try:
                # An explicit in-app update is authoritative: force data.py to pull instead of using its 1h cache.
                with open(os.path.join(DATA, 'update.txt'), 'w', encoding='utf-8') as f:
                    f.write('0')
                servant_log = run_step('data.py')
                ce_log = run_step('ce.py')
                servants = load_json(os.path.join(DATA, 'servants.json'), []) or []
                ces = load_json(os.path.join(DATA, 'ces.json'), []) or []
                if not servants or not ces:
                    raise RuntimeError(f'Generated dataset is unexpectedly empty: servants={len(servants)}, CEs={len(ces)}')
                previous_servant_count = snapshot_json_count(snapshot, 'servants.json')
                previous_ce_count = snapshot_json_count(snapshot, 'ces.json')
                # FGO's historical servant and bond-CE catalogs are append-only for
                # this planner. A sudden shrink means our parser/source contract broke,
                # not that dozens of owned units disappeared from the game.
                if previous_servant_count and len(servants) < previous_servant_count:
                    raise RuntimeError(
                        f'Servant catalog unexpectedly shrank {previous_servant_count} -> {len(servants)}; '
                        'rolled back to the last good dataset. See data/data_sync_data.log.'
                    )
                if previous_ce_count and len(ces) < previous_ce_count:
                    raise RuntimeError(
                        f'Bond CE catalog unexpectedly shrank {previous_ce_count} -> {len(ces)}; '
                        'rolled back to the last good dataset. See data/data_sync_ce.log.'
                    )
                chaldea_commit = local_chaldea_head()
                meta = {
                    'source': 'Atlas Academy + Chaldea Data',
                    'syncedAt': int(time.time()),
                    'atlasJP': stable_jp_info(remote) if remote is not None else (load_json(META_PATH, {}) or {}).get('atlasJP'),
                    'atlasInfo': remote,
                    'atlasAvailable': remote is not None,
                    'atlasError': atlas_error or '',
                    'chaldeaCommit': chaldea_commit,
                    'servantCount': len(servants),
                    'ceCount': len(ces),
                    'ceSync': load_json(os.path.join(DATA, 'ce_sync_report.json'), {}) or {},
                    'cnSync': load_json(os.path.join(DATA, 'cn_sync_report.json'), {}) or {},
                    'servantSync': load_json(os.path.join(DATA, 'servant_sync_report.json'), {}) or {},
                }
                atomic_json(META_PATH, meta)
            except Exception:
                restore_generated_files(snapshot)
                raise
            # Re-read the source HEAD after the pull in case the remote advanced
            # while the update was running.
            remote_chaldea = remote_chaldea_head() or chaldea_commit
            payload = status_payload(remote, remote_chaldea, atlas_error)
            payload.update({
                'updated': True,
                'servantCount': len(servants),
                'ceCount': len(ces),
                'logTail': (servant_log + '\n' + ce_log)[-4000:],
            })

        if args.json:
            print(json.dumps(payload, ensure_ascii=False))
        else:
            print(json.dumps(payload, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:
        payload = {'ok': False, 'error': str(exc)}
        if args.json:
            print(json.dumps(payload, ensure_ascii=False))
        else:
            print(json.dumps(payload, ensure_ascii=False, indent=2))
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
