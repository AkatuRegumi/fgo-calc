import json
import os
from collections import defaultdict

ROOT = os.path.dirname(os.path.abspath(__file__))
DATA_DIR = os.path.join(ROOT, 'data')
os.chdir(DATA_DIR)


def load_json(path, default=None):
    try:
        with open(path, 'r', encoding='utf-8-sig') as f:
            return json.load(f)
    except FileNotFoundError:
        return default


def translate(text, kind):
    mapping = load_json(f'names/{kind}.json', {}) or {}
    value = mapping.get(str(text))
    return value if value else str(text)


def find_files(name):
    out = []
    dist = os.path.join('chaldea-data', 'dist')
    if not os.path.isdir(dist):
        return out
    for root, _, filenames in os.walk(dist):
        for filename in filenames:
            if filename.startswith(name + '.') and filename.endswith('.json'):
                out.append(os.path.join(root, filename))
    return sorted(out)


ces = []
for path in find_files('craftEssences'):
    data = load_json(path, []) or []
    if isinstance(data, list):
        ces.extend(data)

if not ces:
    raise RuntimeError('No craftEssences.*.json found under data/chaldea-data/dist. Run data.py first.')

ce_by_id = {int(ce['id']): ce for ce in ces if isinstance(ce, dict) and 'id' in ce}
base_functions_raw = load_json('chaldea-data/dist/baseFunctions.json', []) or []
base_functions = {
    int(f.get('funcId', f.get('id', 0))): f
    for f in base_functions_raw
    if isinstance(f, dict) and int(f.get('funcId', f.get('id', 0)) or 0) != 0
}


def trait_ids(value):
    result = []
    if value is None:
        return result
    if isinstance(value, (int, float)):
        return [int(value)] if int(value) != 0 else []
    if isinstance(value, dict):
        if 'id' in value:
            try:
                tid = int(value['id'])
                if tid:
                    result.append(tid)
            except (TypeError, ValueError):
                pass
        else:
            for v in value.values():
                result.extend(trait_ids(v))
        return result
    if isinstance(value, list):
        for item in value:
            result.extend(trait_ids(item))
    return result


def walk_dicts(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk_dicts(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk_dicts(child)


def effect_from_svals(node):
    rates = []
    directs = []
    for sval in node.get('svals') or []:
        if not isinstance(sval, dict):
            continue
        if 'RateCount' in sval:
            try:
                rates.append(float(sval['RateCount']) / 10.0)
            except (TypeError, ValueError):
                pass
        # Extremely old direct-bond CEs use a fixed value rather than a rate.
        for key in ('Value', 'Count', 'AddCount'):
            if key in sval and 'RateCount' not in sval:
                try:
                    v = float(sval[key])
                    if 0 < v <= 1000:
                        directs.append(v)
                except (TypeError, ValueError):
                    pass
    if rates:
        return max(rates)
    if directs:
        # Project convention: negative filter value means a fixed bond amount.
        return -max(directs)
    return None


def node_func_type(node):
    ftype = node.get('funcType')
    if ftype:
        return ftype
    try:
        fid = int(node.get('funcId', 0) or 0)
    except (TypeError, ValueError):
        fid = 0
    if fid and fid in base_functions:
        return base_functions[fid].get('funcType')
    return None


def node_target_traits(node):
    candidates = []
    # functvals is Atlas/Chaldea's normal target-trait field.
    for key in ('functvals', 'tvals'):
        vals = trait_ids(node.get(key))
        if vals:
            candidates.extend(vals)
    # Some newer functions supply an overwrite target list. Flattening is safe for
    # the simple AND-trait bond filters currently used by these CEs.
    if not candidates:
        candidates.extend(trait_ids(node.get('overWriteTvalsList')))
    try:
        fid = int(node.get('funcId', 0) or 0)
    except (TypeError, ValueError):
        fid = 0
    base = base_functions.get(fid, {}) if fid else {}
    if not candidates:
        candidates.extend(trait_ids(base.get('functvals')))
    if not candidates:
        candidates.extend(trait_ids(base.get('overWriteTvalsList')))
    return sorted(set(candidates))


def has_event_scope(node):
    for key in ('eventId', 'eventID'):
        try:
            if int(node.get(key, 0) or 0) != 0:
                return True
        except (TypeError, ValueError):
            pass
    return False


def discover_bond_filters(raw_ce):
    grouped = defaultdict(list)
    for node in walk_dicts(raw_ce.get('skills', raw_ce)):
        if node_func_type(node) != 'servantFriendshipUp':
            continue
        if has_event_scope(node):
            continue
        effect = effect_from_svals(node)
        if effect is None or effect == 0:
            continue
        traits = tuple(node_target_traits(node))
        grouped[traits].append(effect)

    filters = []
    for traits, effects in grouped.items():
        # CE skill arrays normally contain base and MLB values. The optimizer is
        # built around the fully-limit-broken effect, so choose the strongest one.
        positives = [v for v in effects if v > 0]
        negatives = [v for v in effects if v < 0]
        if positives:
            effect = max(positives)
        elif negatives:
            effect = min(negatives)  # larger fixed amount -> more negative
        else:
            continue
        filters.append((list(traits), effect))
    filters.sort(key=lambda item: (item[0], -item[1]))
    return filters


def ce_image(raw):
    equip = (((raw.get('extraAssets') or {}).get('equipFace') or {}).get('equip') or {})
    if isinstance(equip, dict) and equip:
        # Highest key is normally the final asset variant.
        key = sorted(equip, key=lambda x: int(x) if str(x).isdigit() else str(x))[-1]
        return equip[key]
    return ''


def make_ce(raw, filters, server=None):
    data = {
        'id': int(raw['id']),
        'name': translate(raw.get('name', raw['id']), 'ce'),
        'img': ce_image(raw),
        'cost': int(raw.get('cost', 0) or 0),
        'filters': [[traits, effect] for traits, effect in filters],
    }
    if server:
        data['server'] = server
    return data


previous_ce_data = load_json('ces.json', []) or []
previous_ce_by_id = {
    int(item['id']): item for item in previous_ce_data
    if isinstance(item, dict) and 'id' in item
}

# Stable legacy contract from upstream. Existing known CEs keep their reviewed
# filters so an upstream schema change cannot silently alter historical results.
LEGACY = {
    9408220: [([103], 20)],
    9408060: [([104], 20)],
    9407850: [([300, 303], 20)],
    9407740: [([2654], 20)],
    9308100: [([2883], 10)],
    9407480: [([2821], 20)],
    9406740: [([], 2.5)],
    9406200: [([], 2.5)],
    9406010: [([], 2.5)],
    9405170: [([], 5)],
    9404360: [([], 5)],
    9404180: [([], 5)],
    9403990: [([], 5)],
    9403520: [([], 5)],
    9401970: [([], 10)],
    9400980: [([], -50)],
    9408390: [([2780], 20)],
    9408590: [([300, 2], 20)],
    9408990: [([301, 2858], 20)],
    9408800: [([304], 20), ([203], 20)],
    9311320: [([], 2)],
    9311450: [([], 2)],
    9409210: [([302], 20)],
    9409320: [([100], 20)],
    9409490: [([105], 20)],
}
# Upstream 2026-09-06 moved 9408800 into the common pool and added 9409490 as JP-only.
LEGACY_JP_ONLY = {9408990, 9311320, 9311450, 9409210, 9409320, 9409490}

ce_data = []
for ce_id, filters in LEGACY.items():
    raw = ce_by_id.get(ce_id)
    if not raw:
        # Chaldea can reshuffle/split dist files between releases. Never silently
        # delete an already-reviewed bond CE just because the current source snapshot
        # does not expose that entity in the expected split.
        previous = previous_ce_by_id.get(ce_id)
        if previous:
            ce_data.append(previous)
        continue
    ce_data.append(make_ce(raw, filters, 'JP' if ce_id in LEGACY_JP_ONLY else None))

# Automatically discover newly released bond CEs from the machine-readable skill
# functions. New discoveries are JP-only until explicitly present in the reviewed
# legacy list; this prevents a fresh JP CE from leaking into the CN solver early.
new_ids = []
parsed_ids = []
for ce_id, raw in sorted(ce_by_id.items()):
    filters = discover_bond_filters(raw)
    if not filters:
        continue
    parsed_ids.append(ce_id)
    if ce_id in LEGACY:
        continue
    # Avoid material/enemy/equip records that happen to share function structures.
    try:
        collection_no = int(raw.get('collectionNo', 0) or 0)
    except (TypeError, ValueError):
        collection_no = 0
    if collection_no <= 0:
        continue
    ce_data.append(make_ce(raw, filters, 'JP'))
    new_ids.append(ce_id)

# Preserve previously auto-detected bond CEs across transient source split/schema
# changes. A later successful parse of the same ID replaces the preserved record.
current_ids = {int(ce['id']) for ce in ce_data}
for old_id, old_ce in previous_ce_by_id.items():
    if old_id in current_ids:
        continue
    filters = old_ce.get('filters') if isinstance(old_ce, dict) else None
    if isinstance(filters, list) and filters:
        ce_data.append(old_ce)

# De-duplicate defensively and keep deterministic ordering.
by_id = {ce['id']: ce for ce in ce_data}
ce_data = [by_id[k] for k in sorted(by_id)]

with open('ces.json.tmp', 'w', encoding='utf-8') as f:
    json.dump(ce_data, f, ensure_ascii=False, indent=4)
os.replace('ces.json.tmp', 'ces.json')

new_entries = [
    {
        'id': ce['id'],
        'name': ce.get('name', str(ce['id'])),
        'filters': ce.get('filters', []),
        'server': ce.get('server', ''),
    }
    for ce in ce_data
    if ce['id'] in set(new_ids)
]
report = {
    'ceCount': len(ce_data),
    'machineDetectedBondCeCount': len(parsed_ids),
    'newAutoDetectedIds': new_ids,
    'newAutoDetected': new_entries,
    'legacyCount': len(LEGACY),
    'preservedPreviousCount': sum(1 for ce in ce_data if int(ce['id']) in previous_ce_by_id and int(ce['id']) not in ce_by_id),
}
with open('ce_sync_report.json.tmp', 'w', encoding='utf-8') as f:
    json.dump(report, f, ensure_ascii=False, indent=2)
os.replace('ce_sync_report.json.tmp', 'ce_sync_report.json')

print(f"[+] CE update done: {len(ce_data)} bond CEs; auto-new={new_ids or 'none'}")
