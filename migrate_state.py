#!/usr/bin/env python3
"""一次性迁移：清洗 users.db 中残留的旧版 state，使其适应新版逻辑。

处理规则（对 users.state 与 history.state 中的每个 state JSON）：
1. 删除 selections.allowTraits（旧版幽灵字段，新版已无 UI 入口）。
2. 若 allowTraits 非空且精确匹配某个戴冠战职阶集合 -> config.crownWar=true，
   selections.crownClass=对应职阶（保留原查询语义，避免 ceLimit>=6 被后端拒绝）。
3. 若 allowTraits 覆盖全部职阶 -> 视为无限制，直接删除。
4. 其余自定义集合 -> 仅删除；此时若未勾戴冠战且 ceLimit>=6，钳制 ceLimit 到 5，
   避免触发后端 "戴冠战必须进行职阶筛选"。
5. 迁移后 crownWar=true 且 costLimit>118 的，钳制到 118（前端 +12 后不超过 130 上限）。
6. 被打动的行写入 version=2。

用法：
    python3 migrate_state.py --db data/users.db          # dry-run，只打印统计
    python3 migrate_state.py --db data/users.db --apply  # 实际写入（单事务）
"""
import argparse
import json
import sqlite3

CROWN_CLASSES = {
    'Saber': [100],
    'Archer': [102],
    'Lancer': [101],
    'Rider': [103],
    'Caster': [104],
    'Assassin': [105],
    'Berserker': [106],
    'EX1': [108, 110, 115, 107],
    'EX2': [109, 117, 120, 132, 129, 124],
    'Beast': [132, 129, 124],
}
ALL_CLASS_TRAITS = set()
for traits in CROWN_CLASSES.values():
    ALL_CLASS_TRAITS.update(traits)

TRAITSET_TO_CLASS = {tuple(sorted(traits)): value for value, traits in CROWN_CLASSES.items()}

STATE_VERSION = 2
MAX_CE_LIMIT = 6
MAX_COST = 130
CROWN_COST_BONUS = 12


def migrate_state(state: dict, stats: dict) -> bool:
    """就地修改 state，返回是否有改动。"""
    changed = False
    selections = state.get('selections')
    config = state.get('config')
    if not isinstance(selections, dict) or not isinstance(config, dict):
        return False

    if 'allowTraits' in selections:
        traits = selections.pop('allowTraits')
        changed = True
        if traits:
            key = tuple(sorted(traits))
            if set(traits) >= ALL_CLASS_TRAITS:
                stats['full_class_set_dropped'] += 1
            elif key in TRAITSET_TO_CLASS:
                config['crownWar'] = True
                selections['crownClass'] = TRAITSET_TO_CLASS[key]
                stats['converted_to_crown'] += 1
            else:
                stats['custom_set_dropped'] += 1

    crown = config.get('crownWar') is True

    try:
        ce_limit = int(config.get('ceLimit') or 0)
    except (TypeError, ValueError):
        ce_limit = 0
    if not crown and ce_limit >= MAX_CE_LIMIT:
        config['ceLimit'] = str(MAX_CE_LIMIT - 1)
        stats['ce_limit_clamped'] += 1
        changed = True

    if crown:
        try:
            cost_limit = int(config.get('costLimit') or 0)
        except (TypeError, ValueError):
            cost_limit = 0
        if cost_limit + CROWN_COST_BONUS > MAX_COST:
            config['costLimit'] = str(MAX_COST - CROWN_COST_BONUS)
            stats['cost_limit_clamped'] += 1
            changed = True
        if not selections.get('crownClass'):
            stats['crown_without_class'] += 1

    if changed:
        state['version'] = STATE_VERSION
    return changed


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--db', required=True)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()

    db = sqlite3.connect(args.db)
    db.execute('PRAGMA busy_timeout = 10000')
    stats = {
        'converted_to_crown': 0,
        'full_class_set_dropped': 0,
        'custom_set_dropped': 0,
        'ce_limit_clamped': 0,
        'cost_limit_clamped': 0,
        'crown_without_class': 0,
        'unparsable': 0,
    }
    updates = {'users': 0, 'history': 0}

    db.execute('BEGIN IMMEDIATE')
    try:
        for table, key_col in [('users', 'username'), ('history', 'id')]:
            rows = db.execute(
                f"SELECT {key_col}, state FROM {table} WHERE state IS NOT NULL AND state != ''"
            ).fetchall()
            for key, raw in rows:
                try:
                    state = json.loads(raw)
                except (TypeError, ValueError):
                    stats['unparsable'] += 1
                    continue
                if not isinstance(state, dict):
                    stats['unparsable'] += 1
                    continue
                if migrate_state(state, stats):
                    updates[table] += 1
                    if args.apply:
                        db.execute(
                            f"UPDATE {table} SET state = ? WHERE {key_col} = ?",
                            (json.dumps(state, ensure_ascii=False, separators=(',', ':')), key),
                        )
        if args.apply:
            db.commit()
            print('已写入。')
        else:
            db.rollback()
            print('dry-run，未写入。')
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()

    print(f"users 改动 {updates['users']} 行, history 改动 {updates['history']} 行")
    for name, count in stats.items():
        if count:
            print(f"  {name}: {count}")


if __name__ == '__main__':
    main()
