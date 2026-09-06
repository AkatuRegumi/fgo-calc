import os
import json
import time
import sys
import subprocess
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")
except Exception:
    pass

TRAIT_NAME_OVERRIDES = {
    "2717": "中国地域",
}

# Change working directory to 'data' folder relative to this script
os.chdir(os.path.dirname(os.path.abspath(__file__))+'/data')

def _run_git(args):
    p = subprocess.run(
        ["git", "-c", "http.version=HTTP/1.1", *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if p.returncode != 0:
        raise RuntimeError((p.stdout or "git failed").strip())
    return (p.stdout or "").strip()

def fetch_git_repo():
    last_update = 0
    if os.path.exists("update.txt"):
        try:
            last_update = int(open("update.txt", "r", encoding="utf-8").read().strip())
        except Exception:
            last_update = 0
    if time.time() - last_update < 3600:
        print("[data] Chaldea Data fetch skipped (<1h cache).")
        return

    for attempt in range(3):
        try:
            if os.path.exists("chaldea-data"):
                _run_git(["-C", "chaldea-data", "pull", "--ff-only", "--quiet"])
            else:
                _run_git(["clone", "--depth", "1", "--quiet", "https://github.com/chaldea-center/chaldea-data", "chaldea-data"])
            head = _run_git(["-C", "chaldea-data", "rev-parse", "--short", "HEAD"])
            open("update.txt", "w", encoding="utf-8").write(str(int(time.time())))
            print(f"[data] Chaldea Data ready at {head}.")
            return
        except Exception as e:
            if attempt == 2:
                if os.path.isdir("chaldea-data"):
                    try:
                        head = _run_git(["-C", "chaldea-data", "rev-parse", "--short", "HEAD"])
                    except Exception:
                        head = "local cache"
                    print(f"[data][WARN] Chaldea Data update failed after 3 attempts; using existing {head} instead: {e}")
                    return
                raise RuntimeError(f"Failed to update Chaldea Data after 3 attempts: {e}") from e
            print(f"[data] Git fetch failed ({attempt + 1}/3): {e}")
            time.sleep(2 ** attempt)

fetch_git_repo()

def get_translation():
    if not os.path.exists("names"):
        os.makedirs("names")

    traits_raw = json.loads(open("chaldea-data/mappings/trait.json", "r", encoding="utf-8-sig").read())
    traits = {}
    for k, v in traits_raw.items():
        if isinstance(v, dict):
            traits[k] = v.get("CN") or v.get("JP") or v.get("NA") or str(k)
        else:
            traits[k] = v or str(k)
    traits.update(TRAIT_NAME_OVERRIDES)
    open("names/traits.json", "w", encoding="utf-8").write(json.dumps(traits, ensure_ascii=False, indent=4))

    ce_raw = json.loads(open("chaldea-data/mappings/ce_names.json", "r", encoding="utf-8-sig").read())
    ce = {}
    for k, v in ce_raw.items():
        if isinstance(v, dict):
            ce[k] = v.get("CN") or v.get("JP") or v.get("NA") or str(k)
        else:
            ce[k] = v or str(k)
    open("names/ce.json", "w", encoding="utf-8").write(json.dumps(ce, ensure_ascii=False, indent=4))

    servant_raw = json.loads(open("chaldea-data/mappings/svt_names.json", "r", encoding="utf-8-sig").read())
    servant = {}
    for k, v in servant_raw.items():
        if isinstance(v, dict):
            servant[k] = v.get("CN") or v.get("JP") or v.get("NA") or str(k)
        else:
            servant[k] = v or str(k)
    open("names/servant.json", "w", encoding="utf-8").write(json.dumps(servant, ensure_ascii=False, indent=4))

    costume_raw = json.loads(open("chaldea-data/mappings/costume_names.json", "r", encoding="utf-8-sig").read())
    costume = {}
    for k, v in costume_raw.items():
        if isinstance(v, dict):
            costume[k] = v.get("CN") or v.get("JP") or v.get("NA") or str(k)
        else:
            costume[k] = v or str(k)
    open("names/costume.json", "w", encoding="utf-8").write(json.dumps(costume, ensure_ascii=False, indent=4))

get_translation()

def find_files(name):
    files = []
    for root, dirs, filenames in os.walk("chaldea-data/dist"):
        for filename in filenames:
            if filename.startswith(name + ".") and filename.endswith(".json"):
                files.append(os.path.join(root, filename))
    return files

def translate(text, type):
    mapping = json.loads(open(f"names/{type}.json", "r", encoding="utf-8-sig").read())
    if str(text) in mapping and mapping[str(text)]:
        return mapping[str(text)]
    return text

def get_traits(trait_list):
    traits = []
    for trait in trait_list or []:
        try:
            if isinstance(trait, dict):
                tid = int(trait.get('id', 0) or 0)
            else:
                tid = int(trait or 0)
            if tid:
                traits.append(tid)
        except (TypeError, ValueError):
            continue
    return traits

attr_map = {
    "human": 202,
    "star": 203,
    "earth": 201,
    "sky": 200,
    "beast": 204,
}

# Event related functions
def load_events():
    files = ['chaldea-data/dist/wiki.events.1.json']
    events = []
    for file in files:
        if not os.path.exists(file):
            continue
        with open(file, 'r', encoding='utf-8-sig') as f:
            rawdata = json.load(f)
            for item in rawdata:
                if not isinstance(item, dict):
                    continue
                start_times = item.get('startTime') or {}
                end_times = item.get('endTime') or {}
                # This planner only evaluates JP/CN. Region-only wiki events must not
                # break the whole generator when Chaldea adds them.
                if 'JP' not in start_times or 'JP' not in end_times:
                    continue
                event = {
                    'id': item.get('id'),
                    'name': item.get('name') or str(item.get('id', '')),
                    'cn_name': item.get('mcLink') or item.get('name') or str(item.get('id', '')),
                    'type': 1,
                    'startTime_JP': start_times['JP'],
                    'endTime_JP': end_times['JP'],
                }
                if 'CN' in start_times and 'CN' in end_times:
                    event['type'] = 0
                    event['startTime_CN'] = start_times['CN']
                    event['endTime_CN'] = end_times['CN']
                events.append(event)
    return events

events = load_events()

def geteventbyid(id):
    for event in events:
        if event['id'] == id:
            return event
    return None

def is_running(id, location):
    now = datetime.now()
    # now = datetime.fromtimestamp(1741172401)
    event = geteventbyid(id)
    if not event:
        return False
    if location == 'CN':
        if event['type'] != 0:
            return False
        return now >= datetime.fromtimestamp(event['startTime_CN']) and now <= datetime.fromtimestamp(event['endTime_CN'])
    else:
        return now >= datetime.fromtimestamp(event['startTime_JP']) and now <= datetime.fromtimestamp(event['endTime_JP'])

def loadskills():
    filename = 'chaldea-data/dist/baseSkills.json'
    skills = {}
    if os.path.exists(filename):
        with open(filename, 'r', encoding='utf-8-sig') as f:
            rawdata = json.load(f)
            for data in rawdata:
                skills[data['id']] = data
    return skills

skills = loadskills()

def loadfuncs():
    filename = 'chaldea-data/dist/baseFunctions.json'
    funcs = {}
    if os.path.exists(filename):
        with open(filename, 'r', encoding='utf-8-sig') as f:
            rawdata = json.load(f)
            for data in rawdata:
                funcs[data['funcId']] = data
    return funcs
funcs = loadfuncs()

exclude_events = [80059, 80077, 80044, 80072]

def process_servant(test, available_costumes=None):
    data = {}
    data['id'] = int(test['id'])

    data['name'] = translate(test.get('name', test['id']), "servant")

    traits = get_traits(test.get('traits') or [])
    cost = int(test.get('cost', 0) or 0)
    faces = (((test.get('extraAssets') or {}).get('faces') or {}).get('ascension') or {})
    img = faces.get('1') or faces.get(1) or next(iter(faces.values()), '')

    # process traitAdd
    if 'traitAdd' in test:
        trait_adds = test['traitAdd']
        for trait_add in trait_adds:
            if not trait_add.get("eventId") and "endedAt" in trait_add:
                # if test['id'] == 604200: print(trait_add)
                ended_at = datetime.fromtimestamp(trait_add["endedAt"])
                if ended_at < datetime.now():
                    continue
                # traits = traits + get_traits(trait_add['trait'])
                traits = list(set(traits + get_traits(trait_add['trait'])))
                # sort traits
                traits.sort()


    data['diff'] = {}

    data['diff']['default'] = {
        'name': '默认',
        'traits': traits,
        'img': img,
        'cost': cost
    }

    data['diff']['asc1'] = {
        'name': "灵基再临1",
        'traits': traits,
        'img': faces.get('2') or faces.get(2) or img,
        'cost': cost
    }

    data['diff']['asc2'] = {
        'name': "灵基再临2",
        'traits': traits,
        'img': faces.get('3') or faces.get(3) or img,
        'cost': cost
    }

    data['diff']['asc3'] = {
        'name': "灵基再临3",
        'traits': traits,
        'img': faces.get('4') or faces.get(4) or img,
        'cost': cost
    }

    profile_costumes = available_costumes
    if profile_costumes is None:
        profile_costumes = (test.get('profile') or {}).get('costume') or {}
    costume_faces = (test.get('extraAssets') or {}).get('faces', {}).get('costume') or {}
    for key, costume in profile_costumes.items():
        if key in costume_faces:
            data['diff'][key] = {
                'name': translate(costume['name'], "costume"),
                'traits': traits,
                'img': costume_faces[key],
                'cost': cost
            }

    costume_map = {}
    if profile_costumes:
        for key, value in profile_costumes.items():
            costume_map[str(value['id'])] = str(key)

    ascension_add = test.get('ascensionAdd') or {}
    if 'overwriteCost' in ascension_add:
        oc = ascension_add['overwriteCost']
        if 'costume' in oc:
            for key, value in oc['costume'].items():
                costume_key = costume_map.get(str(key))
                if costume_key in data['diff']:
                    data['diff'][costume_key]['cost'] = value
        if 'ascension' in oc:
            for key, value in oc['ascension'].items():
                asc_key = f"asc{key}"
                if asc_key in data['diff']:
                    data['diff'][asc_key]['cost'] = value

    if 'individuality' in ascension_add:
        indiv = ascension_add['individuality']
        if 'ascension' in indiv:
            for key, value in indiv['ascension'].items():
                asc_key = f"asc{key}"
                if key == '0':
                    asc_key = 'default'
                if asc_key in data['diff']:
                    merged = list(set(data['diff'][asc_key]['traits'] + get_traits(value)))
                    merged.sort()
                    data['diff'][asc_key]['traits'] = merged
        if 'costume' in indiv:
            for key, value in indiv['costume'].items():
                if str(key) in data['diff']:
                    merged = list(set(data['diff'][str(key)]['traits'] + get_traits(value)))
                    merged.sort()
                    data['diff'][str(key)]['traits'] = merged

    if 'attribute' in ascension_add:
        attr_add = ascension_add['attribute']
        if 'ascension' in attr_add:
            for key, value in attr_add['ascension'].items():
                asc_key = f"asc{key}"
                if key == '0':
                    asc_key = 'default'
                if asc_key in data['diff']:
                    tmp = data['diff'][asc_key]['traits']
                    for attr in tmp:
                        if attr in [200, 201, 202, 203, 204]:
                            tmp.remove(attr)

                    mapped_attr = attr_map.get(value)
                    if mapped_attr is not None:
                        tmp.append(mapped_attr)
                    data['diff'][asc_key]['traits'] = tmp
        if 'costume' in attr_add:
            for key, value in attr_add['costume'].items():
                if str(key) in data['diff']:
                    tmp = data['diff'][str(key)]['traits']
                    for attr in tmp:
                        if attr in [200, 201, 202, 203, 204]:
                            tmp.remove(attr)

                    mapped_attr = attr_map.get(value)
                    if mapped_attr is not None:
                        tmp.append(mapped_attr)
                    data['diff'][str(key)]['traits'] = tmp

    diffs = []

    for k in list(data['diff'].keys()):
        diffflag = True
        for diffkey in diffs:
            if data['diff'][k]['traits'] == data['diff'][diffkey]['traits'] and data['diff'][k]['cost'] == data['diff'][diffkey]['cost']:
                del data['diff'][k]
                diffflag = False
                break
        if diffflag:
            diffs.append(k)

    # Process event bonuses
    data['event_bonuses'] = {"CN": [], "JP": []}
    data['event_party_bonuses'] = {"CN": [], "JP": []}
    data['event_extra_bonuses'] = {"CN": [], "JP": []}
    if "extraPassive" in test:
        for extra in test['extraPassive']:
            extras = extra.get('extraPassive') or []
            if not extras or not isinstance(extras[0], dict):
                continue
            extrainfo = extras[0]
            if 'eventId' not in extrainfo:
                continue
            event_id = extrainfo['eventId']

            # Check for CN
            if is_running(event_id, "CN") and event_id not in exclude_events:
                skill = skills.get(extra['id'])
                if skill:
                    for func in skill['functions']:
                        funcId = func['funcId']
                        base_func = funcs.get(funcId) or {}
                        if base_func.get('funcType') == "servantFriendshipUp":
                            event_info = geteventbyid(event_id)
                            event_name = event_info['cn_name'] if event_info and event_info['cn_name'] else (event_info['name'] if event_info else str(event_id))
                            target = data['event_party_bonuses'] if base_func.get('funcTargetType') == 'ptFull' else data['event_bonuses']
                            target["CN"].append({
                                'id': event_id,
                                'name': event_name,
                                'bonus': int((((func.get("svals") or [{}])[0]).get("RateCount", 0) or 0)) // 10
                            })
                            break

            # Check for JP
            if is_running(event_id, "JP") and event_id not in exclude_events:
                skill = skills.get(extra['id'])
                if skill:
                    for func in skill['functions']:
                        funcId = func['funcId']
                        base_func = funcs.get(funcId) or {}
                        if base_func.get('funcType') == "servantFriendshipUp":
                            event_info = geteventbyid(event_id)
                            event_name = event_info['cn_name'] if event_info and event_info['cn_name'] else (event_info['name'] if event_info else str(event_id))
                            target = data['event_party_bonuses'] if base_func.get('funcTargetType') == 'ptFull' else data['event_bonuses']
                            target["JP"].append({
                                'id': event_id,
                                'name': event_name,
                                'bonus': int((((func.get("svals") or [{}])[0]).get("RateCount", 0) or 0)) // 10
                            })
                            break

    return data

def load_event_detail():
    files = find_files("events")
    event_details = {}
    for file in files:
        with open(file, 'r', encoding='utf-8-sig') as f:
            rawdata = json.load(f)
            for data in rawdata:
                event_details[data['id']] = data
    return event_details

def apply_extra_event_bonuses(processed_servants):
    event_details = load_event_detail()
    for event_id, detail in event_details.items():
        for location in ['CN', 'JP']:
            if is_running(event_id, location):
                for campaign in detail.get("campaigns", []):
                    if campaign.get('target') == "questFriendship":
                        bonus_value = int(campaign.get('value', 0) or 0) // 10
                        target_ids = campaign.get('targetIds', [])
                        for s in processed_servants:
                            if s['id'] in target_ids:
                                s['event_extra_bonuses'][location].append({
                                    'id': event_id,
                                    'name': detail.get('name') or str(event_id),
                                    'bonus': bonus_value
                                })

processed = []

remove_list = [2501500, 1002100, 505600, 600710]

previous_servants = []
try:
    with open('servants.json', 'r', encoding='utf-8-sig') as f:
        previous_servants = json.load(f)
except (FileNotFoundError, json.JSONDecodeError, UnicodeDecodeError):
    previous_servants = []
previous_by_id = {
    int(item['id']): item for item in previous_servants
    if isinstance(item, dict) and 'id' in item
}
seen_ids = set()
process_failures = []
for file in find_files("servants"):
    raw = json.loads(open(file, "r", encoding="utf-8-sig").read())
    for servant in raw:
        if not isinstance(servant, dict) or 'id' not in servant:
            continue
        servant_id = int(servant['id'])
        if servant_id in remove_list or servant_id in seen_ids:
            continue
        seen_ids.add(servant_id)
        try:
            processed.append(process_servant(servant))
        except Exception as e:
            process_failures.append({
                'id': servant_id,
                'name': str(servant.get('name', servant_id)),
                'error': repr(e),
            })
            # Upstream schema changes should not silently delete a previously valid
            # servant from the user's local planner. Preserve the last good entry.
            if servant_id in previous_by_id:
                processed.append(previous_by_id[servant_id])

processed.sort(key=lambda item: int(item.get('id', 0)))
if not processed:
    raise RuntimeError('No servants could be generated from Chaldea Data.')
print(f"[data] Servants generated: {len(processed)}; preserved-on-error: {sum(1 for x in process_failures if x['id'] in previous_by_id)}; failures: {len(process_failures)}")

try:
    apply_extra_event_bonuses(processed)
except Exception as error:
    # Event metadata is auxiliary to the core Box/trait dataset. Keep the planner
    # update usable and surface the issue in the sync report instead of aborting.
    print(f"[WARN] Event bonus enrichment skipped: {error!r}")

def fetch_atlas_json(url, attempts=3):
    for attempt in range(attempts):
        try:
            req = urllib.request.Request(
                url,
                headers={'User-Agent': 'fgo-calc-local-box-data-sync/1.0'},
            )
            with urllib.request.urlopen(req, timeout=30) as response:
                return json.loads(response.read().decode('utf-8'))
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
            if attempt == attempts - 1:
                raise
            time.sleep(2 ** attempt)

def servant_form_signature(servant):
    return {
        key: {
            'traits': sorted(detail['traits']),
            'cost': detail['cost']
        }
        for key, detail in servant['diff'].items()
    }

def write_atomic(path, content):
    temp_path = f'{path}.tmp'
    with open(temp_path, 'w', encoding='utf-8') as file:
        file.write(content)
    os.replace(temp_path, path)

def load_cn_servants(jp_servants):
    basic_url = (
        'https://api.atlasacademy.io/basic/CN/servant/search'
        '?rarity=0&rarity=1&rarity=2&rarity=3&rarity=4&rarity=5'
    )
    cn_basic = fetch_atlas_json(basic_url)
    cn_basic_by_id = {servant['id']: servant for servant in cn_basic}
    cn_ids = set(cn_basic_by_id)
    jp_by_id = {servant['id']: servant for servant in jp_servants}
    available_ids = sorted(jp_by_id.keys() & cn_ids)
    unavailable_ids = sorted(jp_by_id.keys() - cn_ids)

    raw_cn = {}
    errors = []
    with ThreadPoolExecutor(max_workers=12) as executor:
        futures = {
            executor.submit(
                fetch_atlas_json,
                f'https://api.atlasacademy.io/nice/CN/svt/{servant_id}?lore=false'
            ): servant_id
            for servant_id in available_ids
        }
        for future in as_completed(futures):
            servant_id = futures[future]
            try:
                raw_cn[servant_id] = future.result()
            except Exception as error:
                errors.append(f'{servant_id}: {error}')
    if errors:
        raise RuntimeError('Failed to fetch CN servants: ' + '; '.join(errors))

    cn_differences = []
    for servant_id in available_ids:
        available_costumes = {
            key: {
                'id': costume['id'],
                'name': costume.get('shortName') or costume.get('name') or str(costume['id'])
            }
            for key, costume in (cn_basic_by_id[servant_id].get('costume') or {}).items()
            if costume.get('costumeCollectionNo', 0) > 0 or costume.get('shortName') or costume.get('name')
        }
        cn_servant = process_servant(raw_cn[servant_id], available_costumes)
        jp_servant = jp_by_id[servant_id]
        cn_servant['event_bonuses'] = jp_servant['event_bonuses']
        cn_servant['event_party_bonuses'] = jp_servant.get('event_party_bonuses', {"CN": [], "JP": []})
        cn_servant['event_extra_bonuses'] = jp_servant['event_extra_bonuses']
        if servant_form_signature(cn_servant) != servant_form_signature(jp_servant):
            cn_differences.append(cn_servant)
    return cn_differences, unavailable_ids

def load_id_lines(path):
    try:
        with open(path, 'r', encoding='utf-8-sig') as f:
            return {int(line.strip()) for line in f if line.strip()}
    except (FileNotFoundError, ValueError):
        return set()


def write_cn_sync_report(payload):
    write_atomic('cn_sync_report.json', json.dumps(payload, ensure_ascii=False, indent=2))


# JP is generated entirely from the locally cloned Chaldea Data repository.
# Commit it first so an optional Atlas CN refresh can never block JP Data Sync.
old_jp = previous_servants
old_jp_ids = {int(s['id']) for s in old_jp if isinstance(s, dict) and 'id' in s}
old_cn_unavailable = load_id_lines('cn_unavailable.txt')
write_atomic('servants.json', json.dumps(processed, ensure_ascii=False, indent=4))

try:
    cn_differences, cn_unavailable = load_cn_servants(processed)
    write_atomic('cn.json', json.dumps(cn_differences, ensure_ascii=False, indent=4))
    write_atomic('cn_unavailable.txt', ''.join(f'{servant_id}\n' for servant_id in cn_unavailable))
    write_cn_sync_report({
        'ok': True,
        'source': 'Atlas Academy CN',
        'fallback': False,
        'updatedAt': int(time.time()),
        'overrideCount': len(cn_differences),
        'unavailableCount': len(cn_unavailable),
    })
    print(f'[+] CN servant differences: {len(cn_differences)}, unavailable: {len(cn_unavailable)}')
except Exception as error:
    # Atlas is useful for CN release/form differences but is not required for the
    # JP planner. If it is blocked, rate-limited, or unreachable, keep the last
    # known CN snapshot. Any brand-new JP servants are conservatively marked as
    # CN-unavailable until a later successful Atlas refresh.
    new_jp_ids = {int(s['id']) for s in processed if isinstance(s, dict) and 'id' in s}
    new_ids = new_jp_ids - old_jp_ids
    fallback_unavailable = sorted((old_cn_unavailable & new_jp_ids) | new_ids)
    if not os.path.exists('cn.json'):
        write_atomic('cn.json', '[]')
    write_atomic('cn_unavailable.txt', ''.join(f'{servant_id}\n' for servant_id in fallback_unavailable))
    write_cn_sync_report({
        'ok': False,
        'source': 'Atlas Academy CN',
        'fallback': True,
        'updatedAt': int(time.time()),
        'error': str(error),
        'preservedPreviousSnapshot': True,
        'newJPIdsMarkedUnavailable': sorted(new_ids),
        'unavailableCount': len(fallback_unavailable),
    })
    print(f'[WARN] Atlas CN refresh unavailable; JP update remains valid and previous CN snapshot is preserved: {error}')


write_atomic('servant_sync_report.json', json.dumps({
    'servantCount': len(processed),
    'sourceRecordCount': len(seen_ids),
    'failureCount': len(process_failures),
    'preservedPreviousCount': sum(1 for x in process_failures if x['id'] in previous_by_id),
    'failures': process_failures[:100],
    'updatedAt': int(time.time()),
}, ensure_ascii=False, indent=2))

os.chdir('..')

print('[+] Servant info update done.')
