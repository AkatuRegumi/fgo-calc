import os
import json
import sys
import time
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta

TRAIT_NAME_OVERRIDES = {
    "2717": "中国地域",
}

# Change working directory to 'data' folder relative to this script
os.chdir(os.path.dirname(os.path.abspath(__file__))+'/data')

def fetch_git_repo():
    last_update = 0
    if os.path.exists("update.txt"):
        last_update = int(open("update.txt").read().strip())
    if time.time() - last_update < 3600:
        print("Last update was less than 1 hour ago. Skipping fetch.")
        return

    try:
        if os.path.exists("chaldea-data"):
            os.chdir("chaldea-data")
            res = os.system("git pull")
            os.chdir("..")
        else:
            res = os.system("git clone https://github.com/chaldea-center/chaldea-data")
        if res == 0: 
            open("update.txt", "w").write(str(int(time.time())))
            print("Git repo fetched successfully.")
    except Exception as e:
        print(f"Error fetching git repo: {e}")
        print("Retrying in 10 seconds...")
        time.sleep(10)
        fetch_git_repo()

fetch_git_repo()

def get_translation():
    if not os.path.exists("names"):
        os.makedirs("names")

    traits_raw = json.loads(open("chaldea-data/mappings/trait.json", "r").read())
    traits = {}
    for k, v in traits_raw.items():
        traits[k] = v["CN"]
    traits.update(TRAIT_NAME_OVERRIDES)
    open("names/traits.json", "w").write(json.dumps(traits, ensure_ascii=False, indent=4))

    ce_raw = json.loads(open("chaldea-data/mappings/ce_names.json", "r").read())
    ce = {}
    for k, v in ce_raw.items():
        ce[k] = v["CN"]
    open("names/ce.json", "w").write(json.dumps(ce, ensure_ascii=False, indent=4))

    servant_raw = json.loads(open("chaldea-data/mappings/svt_names.json", "r").read())
    servant = {}
    for k, v in servant_raw.items():
        servant[k] = v["CN"]
    open("names/servant.json", "w").write(json.dumps(servant, ensure_ascii=False, indent=4))

    costume_raw = json.loads(open("chaldea-data/mappings/costume_names.json", "r").read())
    costume = {}
    for k, v in costume_raw.items():
        costume[k] = v["CN"]
    open("names/costume.json", "w").write(json.dumps(costume, ensure_ascii=False, indent=4))

get_translation()

def find_files(name):
    files = []
    for root, dirs, filenames in os.walk("chaldea-data/dist"):
        for filename in filenames:
            if filename.startswith(name + ".") and filename.endswith(".json"):
                files.append(os.path.join(root, filename))
    return files

def translate(text, type):
    mapping = json.loads(open(f"names/{type}.json", "r").read())
    if str(text) in mapping:
        return mapping[str(text)]
    return text

def get_traits(trait_list):
    traits = []
    for trait in trait_list:
        traits.append(trait['id'])
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
        with open(file, 'r', encoding='utf-8') as f:
            rawdata = json.load(f)
            for data in rawdata:
                event = {}
                event['id'] = data['id']
                event['name'] = data['name']
                event['cn_name'] = data['name']
                if 'mcLink' in data:
                    event['cn_name'] = data['mcLink']
                if not 'startTime' in data:
                    continue
                # if 'JP' not in data['startTime']:
                #     continue
                event['type'] = 1 # japan only
                event['startTime_JP'] = data['startTime']['JP']
                event['endTime_JP'] = data['endTime']['JP']
                if 'CN' in data['startTime']:
                    event['type'] = 0 # together japan and cn
                    event['startTime_CN'] = data['startTime']['CN']
                    event['endTime_CN'] = data['endTime']['CN']
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
        with open(filename, 'r', encoding='utf-8') as f:
            rawdata = json.load(f)
            for data in rawdata:
                skills[data['id']] = data
    return skills

skills = loadskills()

def loadfuncs():
    filename = 'chaldea-data/dist/baseFunctions.json'
    funcs = {}
    if os.path.exists(filename):
        with open(filename, 'r', encoding='utf-8') as f:
            rawdata = json.load(f)
            for data in rawdata:
                funcs[data['funcId']] = data
    return funcs
funcs = loadfuncs()

exclude_events = [80059, 80077, 80044, 80072]

def process_servant(test, available_costumes=None):
    data = {}
    data['id'] = test['id']

    data['name'] = translate(test['name'], "servant")
    # data['img'] = test['extraAssets']['faces']['costume']

    traits = get_traits(test['traits'])
    cost = test['cost']
    img = test['extraAssets']['faces']['ascension']['1']

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
        'img': test['extraAssets']['faces']['ascension']['2'],
        'cost': cost
    }

    data['diff']['asc2'] = {
        'name': "灵基再临2",
        'traits': traits,
        'img': test['extraAssets']['faces']['ascension']['3'],
        'cost': cost
    }

    data['diff']['asc3'] = {
        'name': "灵基再临3",
        'traits': traits,
        'img': test['extraAssets']['faces']['ascension']['4'],
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

    if 'overwriteCost' in test['ascensionAdd']:
        oc = test['ascensionAdd']['overwriteCost']
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

    if 'individuality' in test['ascensionAdd']:
        indiv = test['ascensionAdd']['individuality']
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

    if 'attribute' in test['ascensionAdd']:
        attr_add = test['ascensionAdd']['attribute']
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
                    tmp.append(attr_map[value])
                    data['diff'][asc_key]['traits'] = tmp
        if 'costume' in attr_add:
            for key, value in attr_add['costume'].items():
                if str(key) in data['diff']:
                    tmp = data['diff'][str(key)]['traits']
                    for attr in tmp:
                        if attr in [200, 201, 202, 203, 204]:
                            tmp.remove(attr)
                    tmp.append(attr_map[value])
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
    data['event_extra_bonuses'] = {"CN": [], "JP": []}
    if "extraPassive" in test:
        for extra in test['extraPassive']:
            extrainfo = extra['extraPassive'][0]
            if not 'eventId' in extrainfo:
                continue
            event_id = extrainfo['eventId']
            
            # Check for CN
            if is_running(event_id, "CN") and event_id not in exclude_events:
                skill = skills.get(extra['id'])
                if skill:
                    for func in skill['functions']:
                        funcId = func['funcId']
                        if funcs[funcId]['funcType'] == "servantFriendshipUp":
                            event_info = geteventbyid(event_id)
                            event_name = event_info['cn_name'] if event_info and event_info['cn_name'] else (event_info['name'] if event_info else str(event_id))
                            data['event_bonuses']["CN"].append({
                                'id': event_id, 
                                'name': event_name, 
                                'bonus': func["svals"][0]["RateCount"] // 10
                            })
                            break

            # Check for JP
            if is_running(event_id, "JP") and event_id not in exclude_events:
                skill = skills.get(extra['id'])
                if skill:
                    for func in skill['functions']:
                        funcId = func['funcId']
                        if funcs[funcId]['funcType'] == "servantFriendshipUp":
                            event_info = geteventbyid(event_id)
                            event_name = event_info['cn_name'] if event_info and event_info['cn_name'] else (event_info['name'] if event_info else str(event_id))
                            data['event_bonuses']["JP"].append({
                                'id': event_id, 
                                'name': event_name, 
                                'bonus': func["svals"][0]["RateCount"] // 10
                            })
                            break

    return data

def load_event_detail():
    files = find_files("events")
    event_details = {}
    for file in files:
        with open(file, 'r', encoding='utf-8') as f:
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
                    if campaign['target'] == "questFriendship":
                        bonus_value = campaign['value'] // 10
                        target_ids = campaign.get('targetIds', [])
                        for s in processed_servants:
                            if s['id'] in target_ids:
                                s['event_extra_bonuses'][location].append({
                                    'id': event_id,
                                    'name': detail['name'],
                                    'bonus': bonus_value
                                })

processed = []

remove_list = [2501500, 1002100, 505600, 600710]

for file in find_files("servants"):
    raw = json.loads(open(file, "r").read())
    for servant in raw:
        try:
            if servant['id'] in remove_list:
                continue
            processed.append(process_servant(servant))
        except Exception as e:
            print(f"Error processing servant {servant['name']}: {e}")

apply_extra_event_bonuses(processed)

def fetch_atlas_json(url, attempts=3):
    for attempt in range(attempts):
        try:
            response = requests.get(url, timeout=30)
            response.raise_for_status()
            return response.json()
        except requests.RequestException:
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
    with open(temp_path, 'w') as file:
        file.write(content)
    os.replace(temp_path, path)

def show_download_progress(completed, total, last_reported=-1):
    ratio = completed / total if total else 1
    width = 30
    filled = round(width * ratio)
    bar = '█' * filled + '░' * (width - filled)
    message = f'[Atlas CN] [{bar}] {ratio:>6.1%} {completed}/{total}'
    if sys.stdout.isatty():
        print(f'\r{message}', end='\n' if completed == total else '', flush=True)
        return last_reported

    reported = int(ratio * 10)
    if reported > last_reported or completed == total:
        print(message, flush=True)
    return reported

def load_cn_servants(jp_servants):
    basic_url = (
        'https://api.atlasacademy.io/basic/CN/servant/search'
        '?rarity=0&rarity=1&rarity=2&rarity=3&rarity=4&rarity=5'
    )
    print('[Atlas CN] Fetching available servant list...', flush=True)
    cn_basic = fetch_atlas_json(basic_url)
    cn_basic_by_id = {servant['id']: servant for servant in cn_basic}
    cn_ids = set(cn_basic_by_id)
    jp_by_id = {servant['id']: servant for servant in jp_servants}
    available_ids = sorted(jp_by_id.keys() & cn_ids)
    unavailable_ids = sorted(jp_by_id.keys() - cn_ids)

    raw_cn = {}
    errors = []
    completed = 0
    last_reported = show_download_progress(0, len(available_ids))
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
            completed += 1
            last_reported = show_download_progress(completed, len(available_ids), last_reported)
    if errors:
        raise RuntimeError('Failed to fetch CN servants: ' + '; '.join(errors))

    cn_differences = []
    for servant_id in available_ids:
        available_costumes = {
            key: {
                'id': costume['id'],
                'name': costume['shortName']
            }
            for key, costume in (cn_basic_by_id[servant_id].get('costume') or {}).items()
            if costume.get('costumeCollectionNo', 0) > 0 or costume.get('shortName')
        }
        cn_servant = process_servant(raw_cn[servant_id], available_costumes)
        jp_servant = jp_by_id[servant_id]
        cn_servant['event_bonuses'] = jp_servant['event_bonuses']
        cn_servant['event_extra_bonuses'] = jp_servant['event_extra_bonuses']
        if servant_form_signature(cn_servant) != servant_form_signature(jp_servant):
            cn_differences.append(cn_servant)
    return cn_differences, unavailable_ids

cn_differences, cn_unavailable = load_cn_servants(processed)
write_atomic('servants.json', json.dumps(processed, ensure_ascii=False, indent=4))
write_atomic('cn.json', json.dumps(cn_differences, ensure_ascii=False, indent=4))
write_atomic('cn_unavailable.txt', ''.join(f'{servant_id}\n' for servant_id in cn_unavailable))
print(f'[+] CN servant differences: {len(cn_differences)}, unavailable: {len(cn_unavailable)}')

os.chdir('..')

print('[+] Servant info update done.')
