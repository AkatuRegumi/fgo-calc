let ALL_DATA = { servants: [], craftEssences: [], traits: {} };
let SELECTIONS = {
    includeSvt: new Map(),
    excludeSvt: new Set(),
    includeCe: new Set(),
    excludeCe: new Set(),
    supportLockCe: new Set(),
    excludeSupportCe: new Set(),
    allowTraits: new Set(),
    selectedEvents: new Set(),
    ownedSvt: new Set(),
    ownedCe: new Set(),
    bond10Svt: new Set(),
    bond15Svt: new Set()
};
let SERVANT_PROFILES = new Map();
let BOX_IMPORT_META = { source: '', importedAt: '', matched: 0, skipped: 0 };

function createDefaultServantProfile(id) {
    return {
        id,
        bondRank: 0,
        bondRankMax: 0,
        bondTotal: 0,
        bondNext: 0,
        bondThresholds: {},
        targetRank: 10,
        role: 'normal',
        priority: 'auto',
        fixed: false,
        imported: false
    };
}

function getServantProfile(id) {
    const current = SERVANT_PROFILES.get(id);
    if (current) return current;
    const created = createDefaultServantProfile(id);
    SERVANT_PROFILES.set(id, created);
    return created;
}

function updateServantProfile(id, patch, persist = true) {
    const current = {...getServantProfile(id), ...patch, id};
    if (!['normal', 'driver', 'passenger'].includes(current.role)) current.role = 'normal';
    if (!['auto', 'high', 'low'].includes(current.priority)) current.priority = 'auto';
    current.targetRank = [10, 15].includes(Number(current.targetRank)) ? Number(current.targetRank) : 10;
    current.fixed = !!current.fixed;
    SERVANT_PROFILES.set(id, current);
    if (persist) {
        saveState();
        updateOwnershipSummary();
    }
    return current;
}

function getProfileTargetTotal(profile) {
    const thresholds = profile?.bondThresholds || {};
    const target = Number(profile?.targetRank || 10);
    const value = Number(thresholds[target] ?? thresholds[String(target)] ?? 0);
    return Number.isFinite(value) && value > 0 ? value : 0;
}

function getProfileRemaining(profile) {
    const targetTotal = getProfileTargetTotal(profile);
    if (targetTotal > 0) return Math.max(0, targetTotal - Number(profile.bondTotal || 0));
    if (Number(profile.bondNext || 0) > 0) return Number(profile.bondNext);
    return null;
}

function formatNumber(value) {
    const n = Number(value);
    return Number.isFinite(n) ? Math.round(n).toLocaleString('zh-CN') : '-';
}

function getActiveServants() {
    if (document.getElementById('server-select')?.value !== 'CN') {
        return ALL_DATA.servants;
    }
    const overrides = new Map((ALL_DATA.cnServants || []).map(servant => [servant.id, servant]));
    const unavailable = new Set(ALL_DATA.cnUnavailable || []);
    return ALL_DATA.servants
        .filter(servant => !unavailable.has(servant.id))
        .map(servant => overrides.get(servant.id) || servant);
}

function showFlash(message, type = 'info', duration = 4000) {
    const container = document.getElementById('flash-container');
    const flash = document.createElement('div');
    const icon = type === 'success' ? 'circle-check' : type === 'error' ? 'circle-alert' : 'info';
    flash.className = 'flash-message';
    flash.dataset.type = type;
    flash.setAttribute('role', type === 'error' ? 'alert' : 'status');
    flash.innerHTML = `<i data-lucide="${icon}"></i><span></span>`;
    flash.querySelector('span').textContent = message;

    const close = document.createElement('button');
    close.className = 'flash-close';
    close.type = 'button';
    close.title = '关闭';
    close.setAttribute('aria-label', '关闭通知');
    close.innerHTML = '<i data-lucide="x"></i>';
    flash.appendChild(close);
    container.appendChild(flash);
    refreshIcons();

    let removed = false;
    const remove = () => {
        if (removed) return;
        removed = true;
        flash.classList.add('is-leaving');
        flash.addEventListener('animationend', () => flash.remove(), {once: true});
    };
    close.onclick = remove;
    if (duration > 0) setTimeout(remove, duration);
}

const STAR_TRAITS = {
    0: 400, 1: 401, 2: 402, 3: 403, 4: 404, 5: 405
};

const CLASS_TRAITS = [
    { name: 'Saber', trait: 100 },
    { name: 'Archer', trait: 102 },
    { name: 'Lancer', trait: 101 },
    { name: 'Rider', trait: 103 },
    { name: 'Caster', trait: 104 },
    { name: 'Assassin', trait: 105 },
    { name: 'Berserker', trait: 106 },
    { name: 'Ruler', trait: 108 },
    { name: 'Avenger', trait: 110 },
    { name: 'MoonCancer', trait: 115 },
    { name: 'Shielder', trait: 107 },
    { name: 'Alterego', trait: 109 },
    { name: 'Foreigner', trait: 117 },
    { name: 'Pretender', trait: 120 },
    { name: 'Beast', trait: [132, 129, 124] }
];

let FILTER_STATE = {
    stars: new Set(),
    classes: new Set()
};
let EXCLUDE_SVT_CLASS_FILTER = new Set();

const STORAGE_KEY = 'fgo-calc-state';
const COLLAPSE_SECTION_CONFIG = {
    includeSvt: { containerId: 'include-svt-list', label: '必选从者' },
    excludeSvt: { containerId: 'exclude-svt-panel', label: '排除从者' },
    includeCe: { containerId: 'include-ce-list', label: '必选礼装' },
    excludeCe: { containerId: 'exclude-ce-list', label: '排除礼装' }
};
let UI_STATE = {
    collapsedSections: {
        includeSvt: false,
        excludeSvt: false,
        includeCe: false,
        excludeCe: false
    }
};

let OWNERSHIP_CONFIG = {
    useOwnedServants: false,
    useOwnedCes: false,
    excludeBond10: true,
    // 绊15默认允许参与求解：自身按已满羁绊0收益，作为「梦火の導き」+25%全队辅助。
    excludeBond15: false
};

function applySectionCollapse(key) {
    const section = COLLAPSE_SECTION_CONFIG[key];
    if (!section) return;
    const container = document.getElementById(section.containerId);
    const toggle = document.querySelector(`.collapse-toggle[data-collapse-key="${key}"]`);
    if (!container || !toggle) return;

    const isCollapsed = !!UI_STATE.collapsedSections[key];
    container.classList.toggle('is-collapsed', isCollapsed);
    toggle.classList.toggle('is-collapsed', isCollapsed);
    toggle.setAttribute('aria-expanded', String(!isCollapsed));

    const actionText = isCollapsed ? '展开' : '收起';
    const tip = `${actionText}${section.label}`;
    toggle.title = tip;
    toggle.setAttribute('aria-label', tip);
}

function toggleSectionCollapse(key) {
    if (!(key in UI_STATE.collapsedSections)) return;
    UI_STATE.collapsedSections[key] = !UI_STATE.collapsedSections[key];
    applySectionCollapse(key);
    saveState();
}

function initCollapseToggles() {
    document.querySelectorAll('.collapse-toggle').forEach(toggle => {
        const key = toggle.dataset.collapseKey;
        if (!key || !(key in UI_STATE.collapsedSections)) return;
        toggle.addEventListener('click', () => toggleSectionCollapse(key));
        applySectionCollapse(key);
    });
}

function renderExcludeSvtClassFilters() {
    const container = document.getElementById('exclude-svt-class-filters');
    if (!container) return;
    container.innerHTML = '';

    const label = document.createElement('span');
    label.className = 'exclude-svt-filter-label';
    label.textContent = '职阶筛选:';
    container.appendChild(label);

    const bronzeAll = document.createElement('img');
    bronzeAll.src = '/static/fgo-icon/铜卡All.png';
    bronzeAll.className = 'exclude-filter-icon-btn';
    bronzeAll.title = '全部取消';
    bronzeAll.onclick = () => {
        EXCLUDE_SVT_CLASS_FILTER.clear();
        renderExcludeSvtClassFilters();
        renderSelectionList('svt', 'exclude');
        saveState();
    };
    container.appendChild(bronzeAll);

    const goldAll = document.createElement('img');
    goldAll.src = '/static/fgo-icon/金卡All.png';
    goldAll.className = 'exclude-filter-icon-btn';
    goldAll.title = '全选';
    goldAll.onclick = () => {
        CLASS_TRAITS.forEach(c => {
            if (Array.isArray(c.trait)) c.trait.forEach(t => EXCLUDE_SVT_CLASS_FILTER.add(t));
            else EXCLUDE_SVT_CLASS_FILTER.add(c.trait);
        });
        renderExcludeSvtClassFilters();
        renderSelectionList('svt', 'exclude');
        saveState();
    };
    container.appendChild(goldAll);

    CLASS_TRAITS.forEach(({name, trait}) => {
        const isSelected = Array.isArray(trait) ? trait.every(t => EXCLUDE_SVT_CLASS_FILTER.has(t)) : EXCLUDE_SVT_CLASS_FILTER.has(trait);
        const img = document.createElement('img');
        img.className = 'exclude-filter-icon-btn';
        img.src = `/static/fgo-icon/${isSelected ? '金卡' : '铜卡'}${name}.png`;
        img.title = name;
        img.onclick = () => {
            if (isSelected) {
                if (Array.isArray(trait)) trait.forEach(t => EXCLUDE_SVT_CLASS_FILTER.delete(t));
                else EXCLUDE_SVT_CLASS_FILTER.delete(trait);
            } else {
                if (Array.isArray(trait)) trait.forEach(t => EXCLUDE_SVT_CLASS_FILTER.add(t));
                else EXCLUDE_SVT_CLASS_FILTER.add(trait);
            }
            renderExcludeSvtClassFilters();
            renderSelectionList('svt', 'exclude');
            saveState();
        };
        container.appendChild(img);
    });
}

function getSupportLimitValue() {
    const raw = parseInt(document.getElementById('support-limit').value, 10);
    if (Number.isNaN(raw)) return 1;
    return Math.max(0, raw);
}

function isSupportCandidateCe(ce) {
    if (!ce) return false;
    const server = document.getElementById('server-select').value;
    if (ce.server === 'JP' && server !== 'JP') {
        return false;
    }
    if (ce.id === 9403520 || ce.id === 9401970) {
        return true;
    }
    const filters = ce.filters || [];
    return filters.some(f => {
        const effect = typeof f.effect === 'number' ? f.effect : f.Effect;
        return Number(effect) >= 20;
    });
}

function refreshSupportLockLimitText() {
    const el = document.getElementById('support-lock-limit-text');
    if (!el) return;
    el.textContent = String(getSupportLimitValue());
}

function enforceSupportLockLimit(showAlert = false) {
    const limit = getSupportLimitValue();
    if (SELECTIONS.supportLockCe.size <= limit) return false;
    const ids = Array.from(SELECTIONS.supportLockCe);
    SELECTIONS.supportLockCe = new Set(ids.slice(0, limit));
    renderSelectionList('ce', 'supportLock');
    if (showAlert) {
        alert(`锁定助战礼装最多只能保留 ${limit} 个，已自动截断。`);
    }
    return true;
}

function isCeVisibleOnServer(ce) {
    if (!ce) return false;
    const server = document.getElementById('server-select').value;
    return ce.server !== 'JP' || server === 'JP';
}

function pruneInvalidSelectionsByServer() {
    let changed = false;
    const server = document.getElementById('server-select').value;

    for (const setKey of ['supportLockCe', 'excludeSupportCe', 'includeCe', 'excludeCe']) {
        for (const id of Array.from(SELECTIONS[setKey])) {
            const ce = ALL_DATA.craftEssences.find(item => item.id === id);
            if (!ce || (ce.server === 'JP' && server !== 'JP')) {
                SELECTIONS[setKey].delete(id);
                changed = true;
            }
        }
    }

    const availableServants = new Set(getActiveServants().map(servant => servant.id));
    for (const id of Array.from(SELECTIONS.includeSvt.keys())) {
        const servant = getActiveServants().find(item => item.id === id);
        const diffKey = SELECTIONS.includeSvt.get(id);
        if (!servant || !servant.diff[diffKey]) {
            SELECTIONS.includeSvt.delete(id);
            changed = true;
        }
    }
    for (const id of Array.from(SELECTIONS.excludeSvt)) {
        if (!availableServants.has(id)) {
            SELECTIONS.excludeSvt.delete(id);
            changed = true;
        }
    }

    if (changed) {
        renderSelectionList('ce', 'include');
        renderSelectionList('ce', 'exclude');
        renderSelectionList('ce', 'supportLock');
        renderSelectionList('ce', 'excludeSupport');
        renderSelectionList('svt', 'include');
        renderSelectionList('svt', 'exclude');
    }

    return changed;
}

function getSelectionKey(type, list) {
    if (type === 'ce' && list === 'supportLock') {
        return 'supportLockCe';
    }
    if (type === 'ce' && list === 'excludeSupport') {
        return 'excludeSupportCe';
    }
    return `${list}${type.charAt(0).toUpperCase() + type.slice(1)}`;
}

function getSelectionContainerId(type, list) {
    if (type === 'ce' && list === 'supportLock') {
        return 'support-lock-ce-list';
    }
    if (type === 'ce' && list === 'excludeSupport') {
        return 'exclude-support-ce-list';
    }
    return `${list}-${type}-list`;
}

function getVisibleCraftEssences() {
    return ALL_DATA.craftEssences.filter(isCeVisibleOnServer);
}

function updateOwnershipSummary() {
    const activeServants = getActiveServants();
    const activeServantIds = new Set(activeServants.map(item => item.id));
    const visibleCes = getVisibleCraftEssences();
    const visibleCeIds = new Set(visibleCes.map(item => item.id));

    const ownedSvtCount = Array.from(SELECTIONS.ownedSvt).filter(id => activeServantIds.has(id)).length;
    const bond10Count = Array.from(SELECTIONS.bond10Svt).filter(id => activeServantIds.has(id) && SELECTIONS.ownedSvt.has(id)).length;
    const bond15Ids = new Set(Array.from(SELECTIONS.bond15Svt).filter(id => activeServantIds.has(id) && SELECTIONS.ownedSvt.has(id)));
    SERVANT_PROFILES.forEach((profile, id) => {
        if (activeServantIds.has(id) && SELECTIONS.ownedSvt.has(id) && Number(profile.bondRank || 0) >= 15) bond15Ids.add(id);
    });
    const bond15Count = bond15Ids.size;
    const driverCount = Array.from(SELECTIONS.ownedSvt).filter(id => activeServantIds.has(id) && getServantProfile(id).role === 'driver').length;
    const passengerCount = Array.from(SELECTIONS.ownedSvt).filter(id => activeServantIds.has(id) && getServantProfile(id).role === 'passenger').length;
    const fixedCount = Array.from(SELECTIONS.ownedSvt).filter(id => activeServantIds.has(id) && getServantProfile(id).fixed).length;
    const ownedCeCount = Array.from(SELECTIONS.ownedCe).filter(id => visibleCeIds.has(id)).length;

    const svtSummary = document.getElementById('owned-svt-summary');
    const bondSummary = document.getElementById('owned-svt-bond-summary');
    const importSummary = document.getElementById('box-import-summary');
    const ceSummary = document.getElementById('owned-ce-summary');
    if (svtSummary) svtSummary.textContent = `${ownedSvtCount} / ${activeServants.length}`;
    if (bondSummary) bondSummary.textContent = `羁绊10：${bond10Count} · 羁绊15：${bond15Count} · 司机：${driverCount} · 老板：${passengerCount} · 固定：${fixedCount}`;
    if (importSummary) {
        if (BOX_IMPORT_META.importedAt) {
            const d = new Date(BOX_IMPORT_META.importedAt);
            const snapshot = BOX_IMPORT_META.snapshotAt ? new Date(BOX_IMPORT_META.snapshotAt) : null;
            const snapshotText = snapshot && !Number.isNaN(snapshot.getTime()) ? ` · 快照 ${snapshot.toLocaleString()}` : '';
            importSummary.textContent = `最近同步：${BOX_IMPORT_META.source || '批量导入'} · ${Number.isNaN(d.getTime()) ? BOX_IMPORT_META.importedAt : d.toLocaleString()} · ${BOX_IMPORT_META.matched || 0}骑${snapshotText}`;
        } else {
            importSummary.textContent = '尚未导入精确羁绊数据';
        }
    }
    if (ceSummary) ceSummary.textContent = `${ownedCeCount} / ${visibleCes.length}`;

    const svtToggle = document.getElementById('use-owned-servants');
    const ceToggle = document.getElementById('use-owned-ces');
    const bond10Toggle = document.getElementById('exclude-bond10');
    const bond15Toggle = document.getElementById('exclude-bond15');
    if (svtToggle) svtToggle.checked = OWNERSHIP_CONFIG.useOwnedServants;
    if (ceToggle) ceToggle.checked = OWNERSHIP_CONFIG.useOwnedCes;
    if (bond10Toggle) bond10Toggle.checked = OWNERSHIP_CONFIG.excludeBond10;
    if (bond15Toggle) bond15Toggle.checked = OWNERSHIP_CONFIG.excludeBond15;
}

function handleOwnershipToggle(kind, checked) {
    if (kind === 'svt') OWNERSHIP_CONFIG.useOwnedServants = checked;
    if (kind === 'ce') OWNERSHIP_CONFIG.useOwnedCes = checked;
    saveState();
    updateOwnershipSummary();
}

function handleBondExclusionToggle(level, checked) {
    if (level === 10) OWNERSHIP_CONFIG.excludeBond10 = checked;
    if (level === 15) OWNERSHIP_CONFIG.excludeBond15 = checked;
    saveState();
    updateOwnershipSummary();
}

function setServantBondStatus(id, status) {
    SELECTIONS.ownedSvt.add(id);
    SELECTIONS.bond10Svt.delete(id);
    SELECTIONS.bond15Svt.delete(id);
    if (status === 10) SELECTIONS.bond10Svt.add(id);
    if (status === 15) SELECTIONS.bond15Svt.add(id);
    const profile = getServantProfile(id);
    if (!profile.imported && (status === 10 || status === 15)) {
        SERVANT_PROFILES.set(id, {...profile, bondRank: status, bondRankMax: status, targetRank: status});
    }
    saveState();
    updateOwnershipSummary();
}

function getServantBondStatus(id) {
    if (SELECTIONS.bond15Svt.has(id)) return 15;
    if (SELECTIONS.bond10Svt.has(id)) return 10;
    return 0;
}

function serializeServantProfiles() {
    return Array.from(SERVANT_PROFILES.entries())
        .filter(([id]) => Number.isInteger(Number(id)))
        .map(([id, profile]) => [Number(id), {
            ...createDefaultServantProfile(Number(id)),
            ...profile,
            id: Number(id),
            bondThresholds: {...(profile?.bondThresholds || {})}
        }]);
}

function exportOwnership() {
    const payload = {
        format: 'fgo-calc-local-box-v9',
        exportedAt: new Date().toISOString(),
        servants: Array.from(SELECTIONS.ownedSvt).sort((a, b) => a - b),
        bond10Servants: Array.from(SELECTIONS.bond10Svt).sort((a, b) => a - b),
        bond15Servants: Array.from(SELECTIONS.bond15Svt).sort((a, b) => a - b),
        craftEssences: Array.from(SELECTIONS.ownedCe).sort((a, b) => a - b),
        servantProfiles: serializeServantProfiles(),
        boxImportMeta: {...BOX_IMPORT_META}
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], {type: 'application/json'});
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'fgo-box-v8.json';
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
}

function importOwnershipFile(input) {
    const file = input.files && input.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
        try {
            const payload = JSON.parse(String(reader.result || ''));
            if (!Array.isArray(payload.servants) || !Array.isArray(payload.craftEssences)) {
                throw new Error('Box JSON 缺少 servants / craftEssences 数组');
            }
            SELECTIONS.ownedSvt = new Set(payload.servants.filter(Number.isInteger));
            SELECTIONS.bond10Svt = new Set((payload.bond10Servants || []).filter(Number.isInteger));
            SELECTIONS.bond15Svt = new Set((payload.bond15Servants || []).filter(Number.isInteger));
            for (const id of SELECTIONS.bond10Svt) SELECTIONS.ownedSvt.add(id);
            for (const id of SELECTIONS.bond15Svt) {
                SELECTIONS.ownedSvt.add(id);
                SELECTIONS.bond10Svt.delete(id);
            }
            SELECTIONS.ownedCe = new Set(payload.craftEssences.filter(Number.isInteger));
            if (Array.isArray(payload.servantProfiles)) {
                SERVANT_PROFILES = new Map();
                for (const entry of payload.servantProfiles) {
                    if (!Array.isArray(entry) || entry.length < 2) continue;
                    const id = Number(entry[0]);
                    if (!Number.isInteger(id)) continue;
                    const raw = entry[1] || {};
                    SERVANT_PROFILES.set(id, {
                        ...createDefaultServantProfile(id),
                        ...raw,
                        id,
                        bondThresholds: {...(raw.bondThresholds || {})},
                        fixed: !!raw.fixed
                    });
                }
            }
            if (payload.boxImportMeta && typeof payload.boxImportMeta === 'object') {
                BOX_IMPORT_META = {...BOX_IMPORT_META, ...payload.boxImportMeta};
            }
            saveState();
            updateOwnershipSummary();
            showFlash('Box 导入成功', 'success');
        } catch (error) {
            showFlash('Box 导入失败: ' + error.message, 'error', 6000);
        } finally {
            input.value = '';
        }
    };
    reader.readAsText(file);
}

function parseCsvRows(text) {
    const rows = [];
    let row = [];
    let field = '';
    let quoted = false;
    const source = String(text || '').replace(/^\uFEFF/, '');
    for (let i = 0; i < source.length; i++) {
        const ch = source[i];
        if (quoted) {
            if (ch === '"') {
                if (source[i + 1] === '"') {
                    field += '"';
                    i++;
                } else {
                    quoted = false;
                }
            } else {
                field += ch;
            }
            continue;
        }
        if (ch === '"') {
            quoted = true;
        } else if (ch === ',') {
            row.push(field);
            field = '';
        } else if (ch === '\n') {
            row.push(field.replace(/\r$/, ''));
            rows.push(row);
            row = [];
            field = '';
        } else {
            field += ch;
        }
    }
    if (field.length > 0 || row.length > 0) {
        row.push(field.replace(/\r$/, ''));
        rows.push(row);
    }
    return rows.filter(r => r.some(cell => String(cell).trim() !== ''));
}

function parseCsvInt(value, fallback = 0) {
    const normalized = String(value ?? '').trim().replace(/,/g, '');
    if (!normalized) return fallback;
    const n = Number.parseInt(normalized, 10);
    return Number.isFinite(n) ? n : fallback;
}

function importChaldeaBondCsv(input) {
    const file = input.files && input.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
        try {
            const rows = parseCsvRows(reader.result);
            if (rows.length < 2) throw new Error('CSV 没有可导入的数据');
            const headers = rows[0].map(v => String(v).trim());
            const indexOf = name => headers.findIndex(h => h.toLowerCase() === name.toLowerCase());
            const required = ['svtId', 'Rank', 'RankMax', 'Total', 'Next'];
            const missing = required.filter(name => indexOf(name) < 0);
            if (missing.length) throw new Error(`不是 Chaldea 羁绊详情 CSV，缺少列：${missing.join(', ')}`);

            const knownIds = new Set(ALL_DATA.servants.map(s => s.id));
            const nextOwned = new Set();
            const nextBond10 = new Set();
            const nextBond15 = new Set();
            let matched = 0;
            let skipped = 0;
            let active15To16 = 0;

            for (const row of rows.slice(1)) {
                const id = parseCsvInt(row[indexOf('svtId')], -1);
                if (!knownIds.has(id)) {
                    if (id > 0) skipped++;
                    continue;
                }
                const bondRank = parseCsvInt(row[indexOf('Rank')], 0);
                const bondRankMax = parseCsvInt(row[indexOf('RankMax')], 0);
                const bondTotal = parseCsvInt(row[indexOf('Total')], 0);
                const bondNext = parseCsvInt(row[indexOf('Next')], 0);
                const previous = {...getServantProfile(id)};
                const thresholds = {};
                for (let lv = 5; lv <= 16; lv++) {
                    const idx = indexOf(`Total(Lv${lv})`);
                    if (idx >= 0) {
                        const total = parseCsvInt(row[idx], 0);
                        if (total > 0) thresholds[lv] = total;
                    }
                }
                const autoTarget = bondRankMax > 10 ? 15 : 10;
                SERVANT_PROFILES.set(id, {
                    ...createDefaultServantProfile(id),
                    ...previous,
                    id,
                    bondRank,
                    bondRankMax,
                    bondTotal,
                    bondNext,
                    bondThresholds: thresholds,
                    targetRank: previous.imported ? previous.targetRank : autoTarget,
                    imported: true
                });
                nextOwned.add(id);
                if (bondRank >= 15 && bondRankMax <= 15) nextBond15.add(id);
                else if (bondRank >= 10 && bondRankMax <= 10) nextBond10.add(id);
                if (bondRank >= 15 && bondRankMax > 15) active15To16++;
                matched++;
            }
            if (matched === 0) throw new Error('没有匹配到当前数据库中的从者 svtId');

            // Chaldea 的 Bond CSV 是当前账号持有从者的完整集合，因此作为英灵 Box 的权威快照覆盖。
            SELECTIONS.ownedSvt = nextOwned;
            SELECTIONS.bond10Svt = nextBond10;
            SELECTIONS.bond15Svt = nextBond15;
            OWNERSHIP_CONFIG.useOwnedServants = true;
            BOX_IMPORT_META = {
                source: 'Chaldea Bond CSV',
                importedAt: new Date().toISOString(),
                matched,
                skipped
            };
            saveState();
            updateOwnershipSummary();
            let message = `Chaldea 羁绊 CSV 导入成功：${matched}骑`;
            if (skipped) message += `，${skipped}条当前数据库未识别`;
            if (active15To16) message += `；另有${active15To16}骑为绊15→更高上限：自身继续培养，同时作为梦火+25%来源`;
            showFlash(message, 'success', 7000);
            if (currentModal.type === 'svt' && currentModal.list === 'owned') populateModalGrid(document.getElementById('modal-search').value);
        } catch (error) {
            showFlash('Chaldea CSV 导入失败: ' + error.message, 'error', 7000);
        } finally {
            input.value = '';
        }
    };
    reader.readAsText(file);
}


function formatSnapshotTime(epochSeconds) {
    const seconds = Number(epochSeconds || 0);
    if (!Number.isFinite(seconds) || seconds <= 0) return '';
    const date = new Date(seconds * 1000);
    return Number.isNaN(date.getTime()) ? '' : date.toISOString();
}

function calculateImportedBondNext(previousProfile, bondRank, bondTotal) {
    const thresholds = previousProfile?.bondThresholds || {};
    const nextRank = Number(bondRank || 0) + 1;
    const nextTotal = Number(thresholds[nextRank] ?? thresholds[String(nextRank)] ?? 0);
    if (Number.isFinite(nextTotal) && nextTotal > Number(bondTotal || 0)) {
        return Math.max(0, Math.round(nextTotal - Number(bondTotal || 0)));
    }
    return 0;
}

function importFgoResponseFile(input) {
    const file = input.files && input.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
        try {
            if (!window.FgoResponseImport) throw new Error('FGO Response 解析模块未加载');
            const payload = window.FgoResponseImport.unwrapResponseText(String(reader.result || ''));
            const snapshot = window.FgoResponseImport.extractSnapshot(payload, {
                servantIds: ALL_DATA.servants.map(item => item.id),
                craftEssenceIds: ALL_DATA.craftEssences.map(item => item.id)
            });

            const snapshotIso = formatSnapshotTime(snapshot.serverTime);
            const snapshotDate = snapshotIso ? new Date(snapshotIso) : null;
            const now = Date.now();
            const ageDays = snapshotDate ? Math.max(0, Math.floor((now - snapshotDate.getTime()) / 86400000)) : null;
            const staleText = ageDays !== null && ageDays >= 7 ? `\n⚠ 这份抓包距现在约 ${ageDays} 天，导入会把 Box/羁绊回退到当时状态。` : '';
            const ceText = snapshot.ceInventoryAvailable
                ? `\n匹配到满破羁绊礼装：${snapshot.mlbCraftEssences.length}张${snapshot.nonMlbCraftEssences.length ? `（另有${snapshot.nonMlbCraftEssences.length}张未满破，不会计入20%库存）` : ''}`
                : '\n未发现可用的礼装实例数据，本次不会覆盖礼装库存。';
            const confirmText = `准备导入 FGO 登录快照：\n抓包时间：${snapshotDate ? snapshotDate.toLocaleString() : '未知'}\n已持有从者：${snapshot.servants.length}骑${ceText}${staleText}\n\n将覆盖“持有从者 + 当前羁绊进度”，但保留司机/老板、手动优先级、固定出场和已有羁绊阈值。原始登录响应不会保存到本工具。`;
            if (!window.confirm(confirmText)) return;

            const nextOwned = new Set();
            const nextBond10 = new Set();
            const nextBond15 = new Set();
            let active15To16 = 0;

            for (const item of snapshot.servants) {
                const id = Number(item.id);
                const previous = {...getServantProfile(id)};
                const bondRank = Number(item.bondRank || 0);
                const bondRankMax = Number(item.bondRankMax || 10);
                const bondTotal = Number(item.bondTotal || 0);
                const autoTarget = bondRankMax > 10 ? 15 : 10;
                SERVANT_PROFILES.set(id, {
                    ...createDefaultServantProfile(id),
                    ...previous,
                    id,
                    bondRank,
                    bondRankMax,
                    bondTotal,
                    bondNext: calculateImportedBondNext(previous, bondRank, bondTotal),
                    bondThresholds: {...(previous.bondThresholds || {})},
                    targetRank: previous.imported ? previous.targetRank : autoTarget,
                    imported: true
                });
                nextOwned.add(id);
                if (bondRank >= 15 && bondRankMax <= 15) nextBond15.add(id);
                else if (bondRank >= 10 && bondRankMax <= 10) nextBond10.add(id);
                if (bondRank >= 15 && bondRankMax > 15) active15To16++;
            }

            SELECTIONS.ownedSvt = nextOwned;
            SELECTIONS.bond10Svt = nextBond10;
            SELECTIONS.bond15Svt = nextBond15;
            OWNERSHIP_CONFIG.useOwnedServants = true;

            if (snapshot.ceInventoryAvailable) {
                SELECTIONS.ownedCe = new Set(snapshot.mlbCraftEssences);
                OWNERSHIP_CONFIG.useOwnedCes = true;
            }

            BOX_IMPORT_META = {
                source: 'FGO / Stream Response',
                importedAt: new Date().toISOString(),
                snapshotAt: snapshotIso,
                matched: snapshot.servants.length,
                skipped: snapshot.unknownOwnedServants || 0,
                ownedCes: snapshot.mlbCraftEssences.length
            };
            saveState();
            updateOwnershipSummary();

            let message = `FGO 登录快照导入成功：${snapshot.servants.length}骑`;
            if (snapshot.ceInventoryAvailable) message += `，满破羁绊礼装${snapshot.mlbCraftEssences.length}张`;
            if (snapshot.unknownOwnedServants) message += `；另有${snapshot.unknownOwnedServants}条从者ID未被当前主数据识别`;
            if (active15To16) message += `；${active15To16}骑为绊15→更高上限：自身继续培养，同时作为梦火+25%来源`;
            if (snapshot.grandRecords?.length) message += `；检测到${snapshot.grandRecords.length}条Grand登记记录`;
            showFlash(message, 'success', 8000);
            if (currentModal.type === 'svt' && currentModal.list === 'owned') populateModalGrid(document.getElementById('modal-search').value);
        } catch (error) {
            showFlash('FGO / Stream Response 导入失败: ' + error.message, 'error', 8000);
        } finally {
            input.value = '';
        }
    };
    reader.onerror = () => {
        showFlash('FGO / Stream Response 文件读取失败', 'error', 6000);
        input.value = '';
    };
    reader.readAsText(file);
}

function isBond15ProviderId(id) {
    if (SELECTIONS.bond15Svt.has(id)) return true;
    const profile = SERVANT_PROFILES.get(id);
    return Number(profile?.bondRank || 0) >= 15;
}

function getEffectiveExcludedServants() {
    const excluded = new Set(SELECTIONS.excludeSvt);
    if (!OWNERSHIP_CONFIG.useOwnedServants) return excluded;
    for (const servant of getActiveServants()) {
        if (!SELECTIONS.ownedSvt.has(servant.id)) {
            excluded.add(servant.id);
            continue;
        }
        if (OWNERSHIP_CONFIG.excludeBond10 && SELECTIONS.bond10Svt.has(servant.id)) excluded.add(servant.id);
        if (OWNERSHIP_CONFIG.excludeBond15 && isBond15ProviderId(servant.id)) excluded.add(servant.id);
    }
    return excluded;
}

function getEffectiveExcludedCes() {
    const excluded = new Set(SELECTIONS.excludeCe);
    if (!OWNERSHIP_CONFIG.useOwnedCes) return excluded;
    for (const ce of getVisibleCraftEssences()) {
        if (!SELECTIONS.ownedCe.has(ce.id)) excluded.add(ce.id);
    }
    return excluded;
}

function validateOwnershipSelections() {
    if (OWNERSHIP_CONFIG.useOwnedServants) {
        const missing = Array.from(SELECTIONS.includeSvt.keys()).filter(id => !SELECTIONS.ownedSvt.has(id));
        if (missing.length > 0) {
            const names = missing.map(id => getActiveServants().find(item => item.id === id)?.name || String(id));
            throw new Error(`必选从者不在“我的 Box”中：${names.join('、')}`);
        }
        const blockedBond10 = Array.from(SELECTIONS.includeSvt.keys()).filter(id => OWNERSHIP_CONFIG.excludeBond10 && SELECTIONS.bond10Svt.has(id));
        const blockedBond15 = Array.from(SELECTIONS.includeSvt.keys()).filter(id => OWNERSHIP_CONFIG.excludeBond15 && isBond15ProviderId(id));
        if (blockedBond10.length > 0 || blockedBond15.length > 0) {
            const ids = [...blockedBond10, ...blockedBond15];
            const names = ids.map(id => getActiveServants().find(item => item.id === id)?.name || String(id));
            throw new Error(`必选从者被 Box 标记为已满羁绊并设置为排除：${names.join('、')}`);
        }
        const fixedIds = getFixedServantIds();
        const manualExcludedFixed = fixedIds.filter(id => SELECTIONS.excludeSvt.has(id));
        if (manualExcludedFixed.length > 0) {
            const names = manualExcludedFixed.map(id => getActiveServants().find(item => item.id === id)?.name || String(id));
            throw new Error(`固定出场从者同时被手动排除：${names.join('、')}`);
        }
        const blockedFixed10 = fixedIds.filter(id => OWNERSHIP_CONFIG.excludeBond10 && SELECTIONS.bond10Svt.has(id));
        const blockedFixed15 = fixedIds.filter(id => OWNERSHIP_CONFIG.excludeBond15 && isBond15ProviderId(id));
        if (blockedFixed10.length || blockedFixed15.length) {
            const ids = [...blockedFixed10, ...blockedFixed15];
            const names = ids.map(id => getActiveServants().find(item => item.id === id)?.name || String(id));
            throw new Error(`固定出场从者被“满羁绊排除”挡住：${names.join('、')}。请关闭对应排除开关或取消固定。`);
        }
        const badClassFixed = fixedIds.filter(id => !servantMatchesAllowedTraits(getActiveServants().find(s => s.id === id)));
        if (badClassFixed.length > 0) {
            const names = badClassFixed.map(id => getActiveServants().find(item => item.id === id)?.name || String(id));
            throw new Error(`固定出场从者不属于当前求解职阶：${names.join('、')}`);
        }
        const uniqueForced = new Set([...Array.from(SELECTIONS.includeSvt.keys()), ...fixedIds]);
        const svtLimit = parseInt(document.getElementById('svt-limit').value || '0', 10);
        if (uniqueForced.size > svtLimit) {
            throw new Error(`必选 + 固定出场共 ${uniqueForced.size} 骑，超过从者数量 ${svtLimit}`);
        }
        const effectiveOwnedCount = getActiveServants().filter(servant => {
            if (!SELECTIONS.ownedSvt.has(servant.id)) return false;
            if (OWNERSHIP_CONFIG.excludeBond10 && SELECTIONS.bond10Svt.has(servant.id)) return false;
            if (OWNERSHIP_CONFIG.excludeBond15 && isBond15ProviderId(servant.id)) return false;
            return true;
        }).length;
        if (effectiveOwnedCount === 0) {
            throw new Error('已启用“仅从我的英灵 Box 求解”，但当前服务器没有可继续获得羁绊的已登记从者');
        }
    }
    if (OWNERSHIP_CONFIG.useOwnedCes) {
        const missing = Array.from(SELECTIONS.includeCe).filter(id => !SELECTIONS.ownedCe.has(id));
        if (missing.length > 0) {
            const names = missing.map(id => ALL_DATA.craftEssences.find(item => item.id === id)?.name || String(id));
            throw new Error(`必选礼装不在“我的礼装”中：${names.join('、')}`);
        }
        const activeOwnedCeCount = getVisibleCraftEssences().filter(ce => SELECTIONS.ownedCe.has(ce.id)).length;
        if (activeOwnedCeCount === 0) {
            throw new Error('已启用“仅使用我拥有的自备羁绊礼装”，但当前服务器没有已登记礼装');
        }
    }
}

function getFixedServantIds() {
    if (!OWNERSHIP_CONFIG.useOwnedServants) return [];
    const activeIds = new Set(getActiveServants().map(s => s.id));
    return Array.from(SELECTIONS.ownedSvt).filter(id => activeIds.has(id) && getServantProfile(id).fixed);
}

function servantMatchesAllowedTraits(servant) {
    if (!servant || SELECTIONS.allowTraits.size === 0) return true;
    return Object.values(servant.diff || {}).some(detail => (detail.traits || []).some(t => SELECTIONS.allowTraits.has(t)));
}

function chooseFixedServantDiff(servant) {
    if (!servant) return 'default';
    const entries = Object.entries(servant.diff || {});
    if (SELECTIONS.allowTraits.size === 0) return servant.diff?.default ? 'default' : (entries[0]?.[0] || 'default');
    const matches = ([, detail]) => (detail.traits || []).some(t => SELECTIONS.allowTraits.has(t));
    if (servant.diff?.default && matches(['default', servant.diff.default])) return 'default';
    return entries.find(matches)?.[0] || (servant.diff?.default ? 'default' : (entries[0]?.[0] || 'default'));
}

function buildOptimizationProfiles() {
    const activeIds = new Set(getActiveServants().map(s => s.id));
    const ids = OWNERSHIP_CONFIG.useOwnedServants
        ? Array.from(SELECTIONS.ownedSvt).filter(id => activeIds.has(id))
        : Array.from(SERVANT_PROFILES.keys()).filter(id => activeIds.has(id));
    return ids.map(id => {
        const p = getServantProfile(id);
        return {
            id,
            bondRank: Number(p.bondRank || 0),
            bondRankMax: Number(p.bondRankMax || 0),
            bondTotal: Number(p.bondTotal || 0),
            bondNext: Number(p.bondNext || 0),
            targetRank: Number(p.targetRank || 10),
            targetTotal: getProfileTargetTotal(p),
            role: p.role || 'normal',
            priority: p.priority || 'auto'
        };
    });
}

function updateOptimizationModeNote() {
    const mode = document.getElementById('optimization-mode')?.value || 'max';
    const note = document.getElementById('optimization-mode-note');
    if (!note) return;
    const texts = {
        max: '只最大化本场真实羁绊总量；司机/老板/当前羁绊进度不会影响排名。',
        balanced: '长期均衡：低羁绊、平时只蹭后排的“老板”会升权；常用司机会降权。固定出场仍是硬约束。',
        finish: '优先收尾：更偏向距离当前培养目标较近、但尚未完成的从者；司机/老板和手动优先级仍会叠加。'
    };
    note.textContent = texts[mode] || texts.max;
}

function handleOptimizationModeChange() {
    updateOptimizationModeNote();
    saveState();
}

function saveState() {
    const state = {
        config: {
            costLimit: document.getElementById('cost-limit').value,
            svtLimit: document.getElementById('svt-limit').value,
            ceLimit: document.getElementById('ce-limit').value,
            supportLimit: document.getElementById('support-limit').value,
            grandMode: document.getElementById('grand-mode')?.checked || false,
            baseBond: document.getElementById('base-bond').value,
            server: document.getElementById('server-select').value,
            enableEventBonus: document.getElementById('enable-event-bonus').checked,
            useOwnedServants: OWNERSHIP_CONFIG.useOwnedServants,
            useOwnedCes: OWNERSHIP_CONFIG.useOwnedCes,
            excludeBond10: OWNERSHIP_CONFIG.excludeBond10,
            excludeBond15: OWNERSHIP_CONFIG.excludeBond15,
            bond15GuidanceMode: 'optimize25',
            optimizationMode: document.getElementById('optimization-mode')?.value || 'max',
        },
        selections: {
            includeSvt: Array.from(SELECTIONS.includeSvt.entries()),
            excludeSvt: Array.from(SELECTIONS.excludeSvt),
            includeCe: Array.from(SELECTIONS.includeCe),
            excludeCe: Array.from(SELECTIONS.excludeCe),
            supportLockCe: Array.from(SELECTIONS.supportLockCe),
            excludeSupportCe: Array.from(SELECTIONS.excludeSupportCe),
            allowTraits: Array.from(SELECTIONS.allowTraits),
            selectedEvents: Array.from(SELECTIONS.selectedEvents),
            ownedSvt: Array.from(SELECTIONS.ownedSvt),
            ownedCe: Array.from(SELECTIONS.ownedCe),
            bond10Svt: Array.from(SELECTIONS.bond10Svt),
            bond15Svt: Array.from(SELECTIONS.bond15Svt),
        },
        servantProfiles: serializeServantProfiles(),
        boxImportMeta: {...BOX_IMPORT_META},
        ui: {
            collapsedSections: { ...UI_STATE.collapsedSections },
            excludeSvtClassFilters: Array.from(EXCLUDE_SVT_CLASS_FILTER)
        }
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

function loadState() {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (!saved) return;
    try {
        const state = JSON.parse(saved);
        if (state.config) {
            if (state.config.costLimit !== undefined) document.getElementById('cost-limit').value = state.config.costLimit;
            if (state.config.svtLimit !== undefined) document.getElementById('svt-limit').value = state.config.svtLimit;
            if (state.config.ceLimit !== undefined) document.getElementById('ce-limit').value = state.config.ceLimit;
            if (state.config.supportLimit !== undefined) document.getElementById('support-limit').value = state.config.supportLimit;
            if (state.config.grandMode !== undefined && document.getElementById('grand-mode')) document.getElementById('grand-mode').checked = !!state.config.grandMode;
            if (state.config.baseBond !== undefined) document.getElementById('base-bond').value = state.config.baseBond;
            if (state.config.server !== undefined) document.getElementById('server-select').value = state.config.server;
            if (state.config.enableEventBonus !== undefined) document.getElementById('enable-event-bonus').checked = state.config.enableEventBonus;
            if (state.config.useOwnedServants !== undefined) OWNERSHIP_CONFIG.useOwnedServants = !!state.config.useOwnedServants;
            if (state.config.useOwnedCes !== undefined) OWNERSHIP_CONFIG.useOwnedCes = !!state.config.useOwnedCes;
            if (state.config.excludeBond10 !== undefined) OWNERSHIP_CONFIG.excludeBond10 = !!state.config.excludeBond10;
            if (state.config.optimizationMode !== undefined && document.getElementById('optimization-mode')) {
                document.getElementById('optimization-mode').value = ['max', 'balanced', 'finish'].includes(state.config.optimizationMode) ? state.config.optimizationMode : 'max';
            }
            // v3把绊15默认当作“已满直接排除”。v4起绊15是+25%梦火辅助，旧状态自动迁移为允许参与。
            if (state.config.bond15GuidanceMode === 'optimize25' && state.config.excludeBond15 !== undefined) {
                OWNERSHIP_CONFIG.excludeBond15 = !!state.config.excludeBond15;
            } else {
                OWNERSHIP_CONFIG.excludeBond15 = false;
            }
        }
        if (state.selections) {
            if (state.selections.includeSvt) SELECTIONS.includeSvt = new Map(state.selections.includeSvt);
            if (state.selections.excludeSvt) SELECTIONS.excludeSvt = new Set(state.selections.excludeSvt);
            if (state.selections.includeCe) SELECTIONS.includeCe = new Set(state.selections.includeCe);
            if (state.selections.excludeCe) SELECTIONS.excludeCe = new Set(state.selections.excludeCe);
            if (state.selections.supportLockCe) SELECTIONS.supportLockCe = new Set(state.selections.supportLockCe);
            if (state.selections.excludeSupportCe) SELECTIONS.excludeSupportCe = new Set(state.selections.excludeSupportCe);
            if (state.selections.allowTraits) SELECTIONS.allowTraits = new Set(state.selections.allowTraits);
            if (state.selections.selectedEvents) SELECTIONS.selectedEvents = new Set(state.selections.selectedEvents);
            if (state.selections.ownedSvt) SELECTIONS.ownedSvt = new Set(state.selections.ownedSvt);
            if (state.selections.ownedCe) SELECTIONS.ownedCe = new Set(state.selections.ownedCe);
            if (state.selections.bond10Svt) SELECTIONS.bond10Svt = new Set(state.selections.bond10Svt);
            if (state.selections.bond15Svt) SELECTIONS.bond15Svt = new Set(state.selections.bond15Svt);
            for (const id of SELECTIONS.bond10Svt) SELECTIONS.ownedSvt.add(id);
            for (const id of SELECTIONS.bond15Svt) {
                SELECTIONS.ownedSvt.add(id);
                SELECTIONS.bond10Svt.delete(id);
            }

            renderMainClassFilters();
            renderSelectionList('svt', 'include');
            renderSelectionList('svt', 'exclude');
            renderSelectionList('ce', 'include');
            renderSelectionList('ce', 'exclude');
            renderSelectionList('ce', 'supportLock');
            renderSelectionList('ce', 'excludeSupport');
            renderEventSelection();
        }
        if (Array.isArray(state.servantProfiles)) {
            SERVANT_PROFILES = new Map();
            for (const entry of state.servantProfiles) {
                if (!Array.isArray(entry) || entry.length < 2) continue;
                const id = Number(entry[0]);
                if (!Number.isInteger(id)) continue;
                const raw = entry[1] || {};
                SERVANT_PROFILES.set(id, {
                    ...createDefaultServantProfile(id),
                    ...raw,
                    id,
                    bondThresholds: {...(raw.bondThresholds || {})},
                    fixed: !!raw.fixed
                });
            }
        }
        if (state.boxImportMeta && typeof state.boxImportMeta === 'object') {
            BOX_IMPORT_META = {...BOX_IMPORT_META, ...state.boxImportMeta};
        }
        updateOptimizationModeNote();
        if (state.ui && state.ui.collapsedSections && typeof state.ui.collapsedSections === 'object') {
            Object.keys(UI_STATE.collapsedSections).forEach(key => {
                if (typeof state.ui.collapsedSections[key] === 'boolean') {
                    UI_STATE.collapsedSections[key] = state.ui.collapsedSections[key];
                }
            });
        }
        if (state.ui && Array.isArray(state.ui.excludeSvtClassFilters)) {
            EXCLUDE_SVT_CLASS_FILTER = new Set(
                state.ui.excludeSvtClassFilters.filter(t => Number.isInteger(t))
            );
        }
        renderExcludeSvtClassFilters();
        try { renderSelectionList('svt', 'exclude'); } catch(e){}
        Object.keys(UI_STATE.collapsedSections).forEach(applySectionCollapse);
        updateOwnershipSummary();
    } catch (e) {
        console.error('Failed to load state:', e);
    }
}

function toggleGrandMode(enabled, persist = true) {
    const ceLimit = document.getElementById('ce-limit');
    const supportLimit = document.getElementById('support-limit');
    const note = document.getElementById('grand-mode-note');

    if (enabled) {
        // 冠位模式：5个普通自备礼装位 + 自家Grand额外1个0 Cost报酬礼装位。
        if (ceLimit) {
            ceLimit.max = '5';
            if (parseInt(ceLimit.value || '0', 10) > 5) ceLimit.value = '5';
        }
        // 助战Grand同时提供普通礼装 + 报酬アップ追加枠，共2个全队加成礼装。
        if (supportLimit) {
            supportLimit.value = '2';
            supportLimit.readOnly = true;
            supportLimit.title = '冠位戴冠战模式固定为2张（助战Grand普通枠 + 报酬アップ追加枠）';
        }
        if (note) note.textContent = '已开启：普通自备最多5张 + 自家Grand额外报酬礼装1张（0 Cost）+ 助战Grand 2张，共8个羁绊加成位。Cost请直接填游戏实际上限。';
    } else {
        if (supportLimit) {
            supportLimit.readOnly = false;
            supportLimit.title = '';
            if (parseInt(supportLimit.value || '0', 10) > 1) supportLimit.value = '1';
        }
        if (note) note.textContent = '关闭时按普通编成计算。开启后：普通自备5张 + 自家Grand额外报酬礼装1张（0 Cost）+ 助战Grand 2张；必须选择对应职阶。';
    }

    refreshSupportLockLimitText();
    enforceSupportLockLimit(true);
    if (persist) saveState();
}

// --- 游戏数据同步（Atlas Academy + Chaldea Data） ---
function dataSyncEscapeHtml(value) {
    return String(value ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
}

function formatSyncTime(ts) {
    const value = Number(ts || 0);
    if (!value) return '从未同步';
    return new Date(value * 1000).toLocaleString('zh-CN', {hour12: false});
}

function summarizeAtlasInfo(info) {
    if (!info) return '已连接';
    if (typeof info === 'string' || typeof info === 'number') return String(info);
    const candidates = ['version', 'appVer', 'appVersion', 'dataVer', 'dataVersion', 'timestamp', 'hash', 'commit'];
    const parts = [];
    for (const key of candidates) {
        if (info[key] !== undefined && info[key] !== null && info[key] !== '') {
            parts.push(`${key}=${info[key]}`);
        }
    }
    if (parts.length) return parts.slice(0, 4).join(' · ');
    try {
        const text = JSON.stringify(info);
        return text.length > 180 ? `${text.slice(0, 177)}…` : text;
    } catch (_) {
        return '已连接';
    }
}

function renderDataSyncLocal() {
    const local = document.getElementById('data-sync-local');
    const ce = document.getElementById('data-sync-ce');
    if (!local) return;
    const meta = ALL_DATA?.dataSourceMeta || {};
    const localTime = Number(meta.syncedAt || ALL_DATA?.dataUpdatedAt || 0);
    const svtCount = Number(meta.servantCount || ALL_DATA?.servants?.length || 0);
    const ceCount = Number(meta.ceCount || ALL_DATA?.craftEssences?.length || 0);
    local.innerHTML = `<strong>本地数据：</strong>${formatSyncTime(localTime)} · 从者 ${svtCount} · 羁绊礼装 ${ceCount}`;
    const ceSync = meta.ceSync || {};
    if (ce) {
        if (Array.isArray(ceSync.newAutoDetectedIds) && ceSync.newAutoDetectedIds.length) {
            const details = Array.isArray(ceSync.newAutoDetected)
                ? ceSync.newAutoDetected.map(item => `${dataSyncEscapeHtml(item.name || item.id)} (#${item.id})`).join('、')
                : ceSync.newAutoDetectedIds.join(', ');
            ce.innerHTML = `<strong>自动发现：</strong>${details} <span class="data-sync-badge">machine parsed</span>`;
        } else if (ceSync.machineDetectedBondCeCount) {
            ce.innerHTML = `<strong>礼装解析器：</strong>已识别 ${ceSync.machineDetectedBondCeCount} 个机器数据中的羁绊礼装效果`;
        } else {
            ce.textContent = '礼装解析器：等待首次 Data Sync';
        }
    }
}

async function checkDataSync(silent = false) {
    const remote = document.getElementById('data-sync-remote');
    const btn = document.getElementById('btn-data-sync-check');
    if (btn) btn.disabled = true;
    if (remote) remote.innerHTML = '<strong>数据源：</strong>正在检查 Chaldea Data / Atlas Academy…';
    try {
        const response = await fetch('/api/data-sync/status', {cache: 'no-store'});
        const payload = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(payload.error || `HTTP ${response.status}`);
        const changed = !!payload.updateAvailable;
        if (remote) {
            const remoteCommit = payload.remoteChaldeaCommit ? String(payload.remoteChaldeaCommit).slice(0, 8) : '';
            const atlasText = payload.atlasAvailable === false
                ? 'Atlas 不可达（JP同步不受影响）'
                : `Atlas ${dataSyncEscapeHtml(summarizeAtlasInfo(payload.atlasJP))}`;
            let sourceText;
            if (remoteCommit) {
                sourceText = `Chaldea Data ${dataSyncEscapeHtml(remoteCommit)} · ${atlasText}`;
            } else if (payload.atlasAvailable !== false) {
                sourceText = `${atlasText} · Git 检查不可用（Atlas 兜底）`;
            } else {
                sourceText = '上游检查暂不可用 · 本地数据仍可正常使用';
            }
            const badge = payload.updateSignal === 'offline'
                ? '<span class="data-sync-badge">无法检查</span>'
                : (changed ? '<span class="data-sync-badge">发现更新</span>' : '<span class="data-sync-badge">当前源已同步</span>');
            remote.innerHTML = `<strong>数据源：</strong>${sourceText} ${badge}`;
        }
        const updateBtn = document.getElementById('btn-data-sync-update');
        if (updateBtn) updateBtn.classList.toggle('secondary', !changed);
        if (changed && !silent) showFlash('检测到数据源更新，可以直接一键同步。', 'success', 4500);
        return payload;
    } catch (error) {
        if (remote) remote.innerHTML = `<strong>数据源：</strong>检查失败 · ${dataSyncEscapeHtml(String(error.message || error))}`;
        if (!silent) showFlash(`数据源检查失败：${error.message || error}`, 'error', 6000);
        return null;
    } finally {
        if (btn) btn.disabled = false;
    }
}

async function runDataSyncUpdate() {
    const btn = document.getElementById('btn-data-sync-update');
    const checkBtn = document.getElementById('btn-data-sync-check');
    const remote = document.getElementById('data-sync-remote');
    const oldText = btn?.innerHTML;
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<span aria-busy="true">正在同步完整数据…</span>';
    }
    if (checkBtn) checkBtn.disabled = true;
    if (remote) remote.innerHTML = '<strong>数据源：</strong>正在同步 Chaldea Data、校验 Atlas 版本并重建索引，首次可能需要几分钟…';
    try {
        const response = await fetch('/api/data-sync/update', {method: 'POST', cache: 'no-store'});
        const payload = await response.json().catch(() => ({}));
        if (!response.ok) {
            const detail = payload.detail ? `\n${payload.detail}` : '';
            throw new Error((payload.error || `HTTP ${response.status}`) + detail);
        }
        const fresh = await fetch(`/api/data?ts=${Date.now()}`, {cache: 'no-store'});
        if (!fresh.ok) throw new Error(`数据已更新，但重新读取失败：HTTP ${fresh.status}`);
        ALL_DATA = await fresh.json();
        ALL_DATA.servants.sort((a, b) => a.id - b.id);
        ALL_DATA.cnServants = ALL_DATA.cnServants || [];
        ALL_DATA.cnServants.sort((a, b) => a.id - b.id);
        ALL_DATA.craftEssences.sort((a, b) => a.id - b.id);
        renderDataSyncLocal();
        const newIds = payload?.ceSync?.newAutoDetectedIds || [];
        const extra = newIds.length ? `；自动发现新羁绊礼装 ${newIds.length} 张` : '';
        const cnFallback = payload?.cnSync?.fallback
            ? '；Atlas CN 不可达，国服差异沿用上次缓存（不影响日服数据）'
            : '';
        showFlash(`游戏数据同步完成：从者 ${payload.servantCount ?? ALL_DATA.servants.length}，羁绊礼装 ${payload.ceCount ?? ALL_DATA.craftEssences.length}${extra}${cnFallback}。`, 'success', 8500);
        renderMainClassFilters();
        renderEventSelection();
        pruneInvalidSelectionsByServer();
        updateOwnershipSummary();
        await checkDataSync(true);
    } catch (error) {
        console.error('Data Sync failed:', error);
        const msg = String(error.message || error);
        if (remote) remote.innerHTML = '<strong>数据源：</strong>同步失败';
        showFlash(`数据同步失败：${msg.length > 800 ? msg.slice(0, 800) + '…' : msg}`, 'error', 9000);
    } finally {
        if (btn) {
            btn.disabled = false;
            if (oldText) btn.innerHTML = oldText;
        }
        if (checkBtn) checkBtn.disabled = false;
        lucide.createIcons();
    }
}

// --- 数据加载与初始化 ---
async function initApp() {
    lucide.createIcons();
    initCollapseToggles();
    renderExcludeSvtClassFilters();

    // 为所有基础配置输入添加自动保存
    ['cost-limit', 'svt-limit', 'ce-limit', 'support-limit', 'base-bond', 'server-select', 'enable-event-bonus', 'grand-mode', 'optimization-mode'].forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            el.addEventListener('change', () => {
                if (id === 'server-select') {
                    SELECTIONS.selectedEvents.clear();
                    renderEventSelection(true);
                    if (pruneInvalidSelectionsByServer()) {
                        saveState();
                    }
                    updateOwnershipSummary();
                } else if (id === 'enable-event-bonus') {
                    renderEventSelection(el.checked);
                } else if (id === 'support-limit') {
                    refreshSupportLockLimitText();
                    enforceSupportLockLimit(true);
                } else if (id === 'optimization-mode') {
                    updateOptimizationModeNote();
                }
                // Update bonus display
                try { renderSelectionList('svt', 'include'); } catch(e){}
                try { renderSelectionList('svt', 'exclude'); } catch(e){}
                saveState();
            });
            if (el.type === 'number' || el.type === 'text') {
                el.addEventListener('input', saveState);
                if (id === 'support-limit') {
                    el.addEventListener('input', refreshSupportLockLimitText);
                }
            }
        }
    });
    try {
        const dataResponse = await fetch('/api/data');
        if (!dataResponse.ok) throw new Error(`Server error: ${dataResponse.status}`);
        ALL_DATA = await dataResponse.json();
        if (ALL_DATA.dataUpdatedAt > 0) {
            const updatedAt = new Date(ALL_DATA.dataUpdatedAt * 1000);
            const label = document.getElementById('data-updated-at');
            label.textContent = `数据源更新于 ${updatedAt.toLocaleString('zh-CN', {hour12: false})}`;
            label.hidden = false;
        }
        ALL_DATA.servants.sort((a, b) => a.id - b.id);
        ALL_DATA.cnServants = ALL_DATA.cnServants || [];
        ALL_DATA.cnServants.sort((a, b) => a.id - b.id);
        ALL_DATA.craftEssences.sort((a, b) => a.id - b.id);

       // 加载持久化状态
       loadState();
       toggleGrandMode(document.getElementById('grand-mode')?.checked || false, false);
       updateOptimizationModeNote();
       refreshSupportLockLimitText();
       const lockChanged = pruneInvalidSelectionsByServer() || enforceSupportLockLimit();
       if (lockChanged) {
           saveState();
       }

       renderMainClassFilters();
       renderEventSelection();
       updateOwnershipSummary();
       renderDataSyncLocal();
       setTimeout(() => checkDataSync(true), 250);
    } catch (error) {
        console.error('Failed to load data:', error);
        showFlash('数据加载失败，请检查后端服务是否正常。', 'error', 6000);
    }
}

function hideAppLoading() {
    const loading = document.getElementById('app-loading');
    if (!loading) return;
    loading.classList.add('is-leaving');
    setTimeout(() => loading.remove(), 200);
}

document.addEventListener('DOMContentLoaded', () => {
    initApp().catch(error => {
        console.error('Failed to initialize app:', error);
        showFlash('页面初始化失败，请刷新后重试。', 'error', 6000);
    }).finally(hideAppLoading);
});

// --- 职阶筛选逻辑 ---
function toggleEventSelection() {
    const enabled = document.getElementById('enable-event-bonus').checked;
    const details = document.getElementById('event-selection-details');
    details.style.display = enabled ? 'block' : 'none';
    if (enabled) {
        renderEventSelection(true);
    }
}

function renderEventSelection(autoSelect = false) {
    const enabled = document.getElementById('enable-event-bonus').checked;
    const server = document.getElementById('server-select').value;
    const details = document.getElementById('event-selection-details');
    const container = document.getElementById('event-checkboxes-container');

    details.style.display = enabled ? 'block' : 'none';
    if (!enabled) return;

    // Extract unique events for current server
    const eventsMap = new Map();
    getActiveServants().forEach(svt => {
        const bonuses = svt.event_bonuses ? svt.event_bonuses[server] : [];
        const partyBonuses = svt.event_party_bonuses ? svt.event_party_bonuses[server] : [];
        const extraBonuses = svt.event_extra_bonuses ? svt.event_extra_bonuses[server] : [];
        [...bonuses, ...partyBonuses, ...extraBonuses].forEach(b => {
            eventsMap.set(b.id, b.name);
        });
    });

    container.innerHTML = '';
    if (eventsMap.size === 0) {
        container.innerHTML = '<p style="font-size: 0.8rem; color: var(--pico-muted-color); margin: 0;">当前服务器无进行中的活动数据。</p>';
        return;
    }

    // Check if we need to auto-select all
    const hasHistory = localStorage.getItem(STORAGE_KEY) !== null;
    if ((autoSelect || !hasHistory) && SELECTIONS.selectedEvents.size === 0) {
        eventsMap.forEach((_, id) => SELECTIONS.selectedEvents.add(id));
    }

    // Controls
    const controls = document.createElement('div');
    controls.style.gridColumn = '1 / -1';
    controls.style.marginBottom = '0.5rem';
    controls.style.display = 'flex';
    controls.style.gap = '8px';

    const btnAll = document.createElement('button');
    btnAll.className = 'outline secondary';
    btnAll.style.padding = '2px 8px';
    btnAll.style.fontSize = '0.75rem';
    btnAll.textContent = '全选';
    btnAll.onclick = (e) => {
        e.preventDefault();
        eventsMap.forEach((_, id) => SELECTIONS.selectedEvents.add(id));
        renderEventSelection();
        try { renderSelectionList('svt', 'include'); } catch(e){}
        try { renderSelectionList('svt', 'exclude'); } catch(e){}
        saveState();
    };

    const btnNone = document.createElement('button');
    btnNone.className = 'outline secondary';
    btnNone.style.padding = '2px 8px';
    btnNone.style.fontSize = '0.75rem';
    btnNone.textContent = '全不选';
    btnNone.onclick = (e) => {
        e.preventDefault();
        SELECTIONS.selectedEvents.clear();
        renderEventSelection();
        try { renderSelectionList('svt', 'include'); } catch(e){}
        try { renderSelectionList('svt', 'exclude'); } catch(e){}
        saveState();
    };

    controls.appendChild(btnAll);
    controls.appendChild(btnNone);
    container.appendChild(controls);

    const sortedEvents = Array.from(eventsMap.entries()).sort((a, b) => b[0] - a[0]);

    sortedEvents.forEach(([id, name]) => {
        const label = document.createElement('label');
        label.style.display = 'flex';
        label.style.alignItems = 'center';
        label.style.gap = '8px';
        // label.style.marginBottom = '0';
        label.style.fontSize = '0.75rem';

        const input = document.createElement('input');
        input.type = 'checkbox';
        input.checked = SELECTIONS.selectedEvents.has(id);

        input.onchange = (e) => {
            if (e.target.checked) SELECTIONS.selectedEvents.add(id);
            else SELECTIONS.selectedEvents.delete(id);
            saveState();
            // Update bonus display if needed
            try { renderSelectionList('svt', 'include'); } catch(e){}
            try { renderSelectionList('svt', 'exclude'); } catch(e){}
        };

        label.appendChild(input);
        const nameSpan = document.createElement('span');
        nameSpan.textContent = name;
        nameSpan.style.whiteSpace = 'nowrap';
        nameSpan.style.overflow = 'hidden';
        nameSpan.style.textOverflow = 'ellipsis';
        label.appendChild(nameSpan);
        container.appendChild(label);
    });
}

function renderMainClassFilters() {
    const container = document.getElementById('main-class-filters');
    if (!container) return;
    container.innerHTML = '';

    // Bronze All (Clear)
    const bronzeAll = document.createElement('img');
    bronzeAll.src = '/static/fgo-icon/铜卡All.png';
    bronzeAll.className = 'class-icon-btn';
    bronzeAll.title = '全部取消';
    bronzeAll.onclick = () => {
        SELECTIONS.allowTraits.clear();
        renderMainClassFilters();
        saveState();
    };
    container.appendChild(bronzeAll);

    // Gold All (Select All)
    const goldAll = document.createElement('img');
    goldAll.src = '/static/fgo-icon/金卡All.png';
    goldAll.className = 'class-icon-btn';
    goldAll.title = '全选';
    goldAll.onclick = () => {
        CLASS_TRAITS.forEach(c => {
            if (Array.isArray(c.trait)) c.trait.forEach(t => SELECTIONS.allowTraits.add(t));
            else SELECTIONS.allowTraits.add(c.trait);
        });
        renderMainClassFilters();
        saveState();
    };
    container.appendChild(goldAll);

    // Individual classes
    CLASS_TRAITS.forEach(({name, trait}) => {
        const isSelected = Array.isArray(trait) ? trait.every(t => SELECTIONS.allowTraits.has(t)) : SELECTIONS.allowTraits.has(trait);
        const img = document.createElement('img');
        img.className = 'class-icon-btn';
        img.src = `/static/fgo-icon/${isSelected ? '金卡' : '铜卡'}${name}.png`;
        img.title = name;
        img.onclick = () => {
            if (isSelected) {
                if (Array.isArray(trait)) trait.forEach(t => SELECTIONS.allowTraits.delete(t));
                else SELECTIONS.allowTraits.delete(trait);
            } else {
                if (Array.isArray(trait)) trait.forEach(t => SELECTIONS.allowTraits.add(t));
                else SELECTIONS.allowTraits.add(trait);
            }
            renderMainClassFilters();
            saveState();
        };
        container.appendChild(img);
    });
}

function openSvtDiffModal(svt) {
    const grid = document.getElementById('svt-diff-modal-grid');
    grid.innerHTML = '';
    document.getElementById('svt-diff-modal-title').textContent = `为 ${svt.name} 选择形态`;

    const baseEntry = svt.diff.default
        ? ['default', svt.diff.default]
        : Object.entries(svt.diff)[0];
    const baseTraits = new Set(baseEntry?.[1]?.traits || []);

    for (const [diffKey, detail] of Object.entries(svt.diff)) {
        const name = detail.name === '默认' ? svt.name : `${svt.name} (${detail.name})`;
        const div = createAvatar(svt.id, name, detail.img, () => {
            addSvtWithDiff(svt.id, diffKey);
        });
        div.appendChild(createSvtTraitDiff(detail.traits || [], baseTraits, diffKey === baseEntry?.[0]));
        grid.appendChild(div);
    }
    refreshIcons();
    document.getElementById('svt-diff-modal').showModal();
}

function createSvtTraitDiff(traits, baseTraits, isBase) {
    const container = document.createElement('div');
    container.className = 'svt-trait-diff';
    if (isBase) {
        const text = document.createElement('span');
        text.className = 'svt-trait-diff-empty';
        text.textContent = '基准形态';
        container.appendChild(text);
        return container;
    }

    const traitSet = new Set(traits);
    const added = traits.filter(id => !baseTraits.has(id));
    const removed = [...baseTraits].filter(id => !traitSet.has(id));
    if (added.length === 0 && removed.length === 0) {
        const text = document.createElement('span');
        text.className = 'svt-trait-diff-empty';
        text.textContent = '特性无变化';
        container.appendChild(text);
        return container;
    }

    appendSvtTraitDiffRow(container, '新增', added, 'is-added');
    appendSvtTraitDiffRow(container, '缺少', removed, 'is-removed');
    return container;
}

function appendSvtTraitDiffRow(container, labelText, traitIds, className) {
    if (traitIds.length === 0) return;
    const row = document.createElement('div');
    row.className = 'svt-trait-diff-row';
    const label = document.createElement('span');
    label.className = 'svt-trait-diff-label';
    label.textContent = `${labelText}:`;
    row.appendChild(label);

    traitIds.map(id => ALL_DATA.traits[id] || `特性 ${id}`)
        .sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'))
        .forEach(name => {
            const tag = document.createElement('span');
            tag.className = `svt-trait-diff-tag ${className}`;
            tag.textContent = name;
            row.appendChild(tag);
        });
    container.appendChild(row);
}

function closeSvtDiffModal() {
    document.getElementById('svt-diff-modal').close();
}


function showSvtTraits(svt, diffKey) {
    const detail = svt.diff[diffKey];
    const title = document.getElementById('svt-trait-display-title');
    const list = document.getElementById('svt-trait-display-list');

    title.textContent = `${svt.name} (${detail.name}) 的特性`;
    list.innerHTML = '';

    if (detail.traits && detail.traits.length > 0) {
        const traitNames = detail.traits
            .map(id => ALL_DATA.traits[id])
            .filter(name => name && name.trim())
            .sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'));

        traitNames.forEach(name => {
            const p = document.createElement('p');
            p.textContent = name;
            p.style.margin = '0.5rem 0';
            list.appendChild(p);
        });
    } else {
        list.innerHTML = '<p>无特殊特性。</p>';
    }

    document.getElementById('svt-trait-display-modal').showModal();
}

function closeSvtTraitDisplayModal() {
    document.getElementById('svt-trait-display-modal').close();
}

// --- 列表管理 ---
function addSvtWithDiff(id, diffKey) {
    SELECTIONS.includeSvt.set(id, diffKey);
    renderSelectionList('svt', 'include');
    saveState();
    closeSvtDiffModal();
    populateModalGrid(document.getElementById('modal-search').value);
}

function removeItem(type, list, id) {
    const key = getSelectionKey(type, list);
    if (type === 'svt' && list === 'include') {
        SELECTIONS.includeSvt.delete(id);
    } else {
        SELECTIONS[key].delete(id);
    }
    renderSelectionList(type, list);
    saveState();
}

function renderSelectionList(type, list) {
    if (list === 'owned') {
        updateOwnershipSummary();
        return;
    }
    const key = getSelectionKey(type, list);
    const container = document.getElementById(getSelectionContainerId(type, list));
    if (!container) return;
    container.innerHTML = '';
    const items = type === 'svt' ? getActiveServants() : ALL_DATA.craftEssences;

    const selection = (type === 'svt' && list === 'include') ?
        Array.from(SELECTIONS.includeSvt.keys()) :
        SELECTIONS[key];

    selection.forEach(id => {
        const item = items.find(i => i.id === id);
        if (item) {
            if (type === 'svt' && list === 'exclude' && EXCLUDE_SVT_CLASS_FILTER.size > 0) {
                const detail = item.diff.default || Object.values(item.diff)[0];
                if (!detail || !detail.traits || !detail.traits.some(t => EXCLUDE_SVT_CLASS_FILTER.has(t))) {
                    return;
                }
            }
            let img, name;
            if (type === 'svt' && list === 'include') {
                const diffKey = SELECTIONS.includeSvt.get(id);
                const detail = item.diff[diffKey];
                img = detail.img;
                name = detail.name === '默认' ? item.name : `${item.name} (${detail.name})`;
            } else {
                img = type === 'svt' ? item.diff.default.img : item.img;
                name = item.name;
            }
            const div = createAvatar(id, name, img, null, () => removeItem(type, list, id), type === 'ce');
            if (type === 'svt') {
                renderEventBonuses(div, item);
            }
            container.appendChild(div);
        }
    });
    refreshIcons();
}

// --- 计算逻辑 ---
async function calculate() {
    const btn = document.getElementById('calculate-btn');
    btn.setAttribute('aria-busy', 'true');
    btn.textContent = '计算中...';

    try {
        validateOwnershipSelections();
        const effectiveExcludedServants = getEffectiveExcludedServants();
        const effectiveExcludedCes = getEffectiveExcludedCes();
        const params = new URLSearchParams();
        params.append('costlimit', document.getElementById('cost-limit').value);
        params.append('svtlimit', document.getElementById('svt-limit').value);
        params.append('celimit', document.getElementById('ce-limit').value);
        params.append('supportlimit', document.getElementById('support-limit').value);
        const grandMode = document.getElementById('grand-mode')?.checked || false;
        if (grandMode && SELECTIONS.allowTraits.size === 0) {
            throw new Error('冠位戴冠战模式必须选择对应职阶');
        }
        params.append('grandmode', grandMode);
        params.append('basebond', document.getElementById('base-bond').value);
        params.append('server', document.getElementById('server-select').value);
        params.append('enable_event_bonus', document.getElementById('enable-event-bonus').checked);
        SELECTIONS.selectedEvents.forEach(id => params.append('selected_events', id));

        SELECTIONS.allowTraits.forEach(id => params.append('allowtraits', id));
        const forcedIds = new Set();
        SELECTIONS.includeSvt.forEach((diff, id) => {
            params.append('includesvt', id);
            params.append('includesvtdiff', diff);
            forcedIds.add(id);
        });
        for (const id of getFixedServantIds()) {
            if (forcedIds.has(id)) continue;
            const servant = getActiveServants().find(s => s.id === id);
            params.append('includesvt', id);
            params.append('includesvtdiff', chooseFixedServantDiff(servant));
            forcedIds.add(id);
        }
        effectiveExcludedServants.forEach(id => params.append('excludesvt', id));
        if (OWNERSHIP_CONFIG.useOwnedServants) {
            const activeIds = new Set(getActiveServants().map(s => s.id));
            SELECTIONS.bond10Svt.forEach(id => {
                if (activeIds.has(id) && SELECTIONS.ownedSvt.has(id)) params.append('bond10svt', id);
            });
            if (!OWNERSHIP_CONFIG.excludeBond15) {
                SELECTIONS.bond15Svt.forEach(id => {
                    if (activeIds.has(id) && SELECTIONS.ownedSvt.has(id)) params.append('bond15svt', id);
                });
            }
        }
        params.append('optimizemode', document.getElementById('optimization-mode')?.value || 'max');
        params.append('bondprofiles', JSON.stringify(buildOptimizationProfiles()));
        SELECTIONS.includeCe.forEach(id => params.append('includece', id));
        effectiveExcludedCes.forEach(id => params.append('excludece', id));
        if (SELECTIONS.supportLockCe.size > getSupportLimitValue()) {
            throw new Error('锁定助战礼装数量不能超过助战礼装数量');
        }
        SELECTIONS.supportLockCe.forEach(id => params.append('includesupportce', id));
        SELECTIONS.excludeSupportCe.forEach(id => params.append('excludesupportce', id));

        const requestStarted = performance.now();
        const response = await fetch('/api/calculate', {
            method: 'POST',
            body: params
        });
        const results = await response.json();
        const requestDuration = performance.now() - requestStarted;
        if (!response.ok) {
            throw new Error(results.error || `Server error: ${response.statusText}`);
        }
        const backendDuration = Number(results.duration) || 0;
        const networkDuration = Math.max(0, requestDuration - backendDuration);
        renderResults(results.teams, backendDuration, networkDuration);
    } catch (error) {
        console.error('Calculation failed:', error);
        alert(`计算失败: ${error.message}`);
    } finally {
        btn.removeAttribute('aria-busy');
        btn.textContent = '开始计算';
    }
}

function renderEventBonuses(div, svt) {
    const enableEventBonus = document.getElementById('enable-event-bonus').checked;
    const server = document.getElementById('server-select').value;
    if (enableEventBonus) {
        let bonuses = svt.event_bonuses ? svt.event_bonuses[server] : [];
        let partyBonuses = svt.event_party_bonuses ? svt.event_party_bonuses[server] : [];
        let extraBonuses = svt.event_extra_bonuses ? svt.event_extra_bonuses[server] : [];

        // Filter based on selected events
        bonuses = bonuses.filter(b => SELECTIONS.selectedEvents.has(b.id));
        partyBonuses = partyBonuses.filter(b => SELECTIONS.selectedEvents.has(b.id));
        extraBonuses = extraBonuses.filter(b => SELECTIONS.selectedEvents.has(b.id));

        if ((bonuses && bonuses.length > 0) || (partyBonuses && partyBonuses.length > 0) || (extraBonuses && extraBonuses.length > 0)) {
            const bonusContainer = document.createElement('div');
            bonusContainer.className = 'event-bonus-container';

            bonuses.forEach(b => {
                const item = document.createElement('div');
                item.className = 'event-bonus-item';
                item.innerHTML = `<span class="event-bonus-val">+${b.bonus}%</span><span class="event-bonus-name" title="${b.name}">${b.name}</span>`;
                bonusContainer.appendChild(item);
            });

            partyBonuses.forEach(b => {
                const item = document.createElement('div');
                item.className = 'event-bonus-item';
                item.style.border = '1px solid var(--pico-primary)';
                item.innerHTML = `<span class="event-bonus-val">全队 +${b.bonus}%</span><span class="event-bonus-name" title="${b.name}">${b.name}</span>`;
                bonusContainer.appendChild(item);
            });

            extraBonuses.forEach(b => {
                const item = document.createElement('div');
                item.className = 'event-bonus-item';
                item.style.border = '1px solid var(--pico-secondary)';
                // Temporary logic change: convert independent multiplier back to additive percent
                // In UI it shows as +20% instead of x1.2
                const percentBonus = Math.round((b.bonus / 100 - 1) * 100);
                item.innerHTML = `<span class="event-bonus-val" style="background:var(--pico-secondary)">+${percentBonus}%</span><span class="event-bonus-name" title="${b.name}">${b.name}</span>`;
                bonusContainer.appendChild(item);
            });
            div.appendChild(bonusContainer);
        }
    }
}

// --- 结果渲染 ---
function renderResults(teams, backendDuration, networkDuration) {
    const container = document.getElementById('results-container');
    const list = document.getElementById('results-list');
    list.innerHTML = '';

    if (backendDuration !== undefined) {
        const p = document.createElement('p');
        p.style.textAlign = 'right';
        p.style.fontSize = '0.8rem';
        p.style.opacity = '0.7';
        const networkText = networkDuration === undefined ? '' : ` | 网络延迟: ${Math.round(networkDuration)} ms`;
        p.innerHTML = `<i data-lucide="timer" style="width: 14px; vertical-align: middle;"></i> 后端计算: ${Math.round(backendDuration)} ms${networkText}`;
        list.appendChild(p);
        refreshIcons();
    }

    if (!teams || teams.length === 0) {
        list.innerHTML += '<p>没有找到符合条件的最优组合。</p>';
        container.hidden = false;
        return;
    }

    teams.forEach((team, index) => {
        const teamDiv = document.createElement('div');
        teamDiv.className = 'team-result';

        const header = document.createElement('hgroup');
        header.style.marginBottom = '0.75rem';
        const guidanceCount = Number(team.Bond15GuidanceCount || 0);
        const guidanceText = guidanceCount > 0 ? ` <small>| 梦火の導き: ${guidanceCount}骑 / 全队 +${Number(team.Bond15GuidancePercent || guidanceCount * 25)}%</small>` : '';
        const optimizationMode = team.OptimizationMode || document.getElementById('optimization-mode')?.value || 'max';
        const modeLabels = {max: '单场最大', balanced: '长期均衡', finish: '优先收尾'};
        const scoreText = optimizationMode === 'max' ? '' : ` <small>| 培养评分: ${Math.round(Number(team.OptimizationScore || 0))}</small>`;
        header.innerHTML = `<h5><i data-lucide="award" style="width: 20px; vertical-align: middle; margin-right: 8px;"></i> 方案 ${index + 1} <small>· ${modeLabels[optimizationMode] || modeLabels.max}</small></h5><p><mark>真实总羁绊: ${team.TotalBond}</mark> <small>| 总Cost: ${team.TotalCost}</small>${scoreText}${guidanceText}</p>`;
        teamDiv.appendChild(header);

        const svtTitle = document.createElement('div');
        svtTitle.className = 'team-section-title';
        svtTitle.textContent = '推荐从者阵容';
        teamDiv.appendChild(svtTitle);

        const svtGrid = document.createElement('div');
        svtGrid.className = 'team-grid';
        teamDiv.appendChild(svtGrid);

        const svtLimit = parseInt(document.getElementById('svt-limit').value, 10);
        for (let i = 0; i < svtLimit; i++) {
            const wrapper = document.createElement('div');
            wrapper.className = 'result-item-wrapper';

            if (i < team.Servants.length) {
                const svtId = team.Servants[i];
                const svt = getActiveServants().find(s => s.id === svtId);
                if (!svt) continue;
                const diffKey = team.DiffChoice[i];
                const detail = svt.diff[diffKey];
                var name;
                if (detail.name === "默认") {
                    name = svt.name;
                } else {
                    name = `${svt.name} (${detail.name})`;
                }
                const div = createAvatar(svt.id, name, detail.img, () => showSvtTraits(svt, diffKey));

                renderEventBonuses(div, svt);

                const bondInfo = Array.isArray(team.ServantBondBonuses) ? team.ServantBondBonuses[i] : null;
                if (bondInfo) {
                    const contributionSpan = document.createElement('div');
                    contributionSpan.className = 'contribution servant-bond-bonus';
                    if (bondInfo.bond15GuidanceSource) {
                        contributionSpan.textContent = '绊15辅助 · 梦火の導き 全队+25% · 自身已满0';
                    } else if (bondInfo.bondCapped) {
                        contributionSpan.textContent = '当前培养上限已满 · 自身羁绊0';
                    } else {
                        const percent = Number(bondInfo.bonusPercent || 0);
                        const percentText = Number.isInteger(percent) ? String(percent) : percent.toFixed(1).replace(/\.0$/, '');
                        const direct = Number(bondInfo.directBonus || 0);
                        const directText = direct > 0 ? ` +${direct}固定` : direct < 0 ? ` ${direct}固定` : '';
                        const guidance = Number(bondInfo.guidanceReceivedPercent || 0);
                        const guidanceText = guidance > 0 ? `（梦火+${Number.isInteger(guidance) ? guidance : guidance.toFixed(1)}%）` : '';
                        contributionSpan.textContent = `羁绊 +${percentText}%${directText}${guidanceText} · 实得 ${bondInfo.totalBond}`;
                    }
                    div.appendChild(contributionSpan);
                }

                if (SELECTIONS.ownedSvt.has(svt.id) || SERVANT_PROFILES.has(svt.id)) {
                    const profile = getServantProfile(svt.id);
                    const profileLine = document.createElement('div');
                    profileLine.className = 'servant-profile-line';
                    const roleLabel = {normal: '普通', driver: '司机', passenger: '老板'}[profile.role] || '普通';
                    const priorityLabel = {auto: '自动', high: '高优先', low: '低优先'}[profile.priority] || '自动';
                    const rankText = profile.bondRank > 0 ? `绊${profile.bondRank}${profile.bondRankMax > 0 ? '/' + profile.bondRankMax : ''}` : '羁绊未导入';
                    const remaining = getProfileRemaining(profile);
                    const remainingText = remaining === null ? '' : ` · 距目标 ${formatNumber(remaining)}`;
                    const fixedText = profile.fixed ? ' · 固定出场' : '';
                    const score = Number(bondInfo?.optimizationScore || 0);
                    const weight = Number(bondInfo?.preferenceWeight || 0);
                    const scorePart = (team.OptimizationMode && team.OptimizationMode !== 'max' && score > 0) ? ` · 权重×${weight.toFixed(2)} / 评分${Math.round(score)}` : '';
                    profileLine.textContent = `${roleLabel} · ${priorityLabel} · ${rankText}${remainingText}${fixedText}${scorePart}`;
                    div.appendChild(profileLine);
                }

                wrapper.appendChild(div);

                const actions = document.createElement('div');
                actions.className = 'result-actions';
                actions.innerHTML = `
                    <button class="outline" title="加入必选" onclick="quickAdd(event, 'svt', 'include', ${svt.id}, '${diffKey}')">
                        <i data-lucide="plus"></i>
                    </button>
                    <button class="outline secondary" title="加入排除" onclick="quickAdd(event, 'svt', 'exclude', ${svt.id})">
                        <i data-lucide="minus"></i>
                    </button>
                `;
                wrapper.appendChild(actions);
            } else {
                wrapper.appendChild(createPlaceholder());
            }
            svtGrid.appendChild(wrapper);
        }

        const ceTitle = document.createElement('div');
        ceTitle.className = 'team-section-title';
        ceTitle.textContent = '推荐礼装搭配';
        teamDiv.appendChild(ceTitle);

        const ceGrid = document.createElement('div');
        ceGrid.className = 'team-grid';
        teamDiv.appendChild(ceGrid);

        const ceLimit = parseInt(document.getElementById('ce-limit').value, 10);
         for (let i = 0; i < ceLimit; i++) {
            const wrapper = document.createElement('div');
            wrapper.className = 'result-item-wrapper';

            if (i < team.CraftEssences.length) {
                const ceInfo = team.CraftEssences[i];
                const ce = ALL_DATA.craftEssences.find(c => c.id === ceInfo.id);
                if (!ce) continue;
                const div = createAvatar(ce.id, ce.name, ce.img, null, null, true);

                const contributionSpan = document.createElement('div');
                contributionSpan.className = 'contribution';
                contributionSpan.textContent = `羁绊 +${ceInfo.contribution}`;
                div.appendChild(contributionSpan);

                wrapper.appendChild(div);

                const actions = document.createElement('div');
                actions.className = 'result-actions';
                actions.innerHTML = `
                    <button class="outline" title="加入必选" onclick="quickAdd(event, 'ce', 'include', ${ce.id})">
                        <i data-lucide="plus"></i>
                    </button>
                    <button class="outline secondary" title="加入排除" onclick="quickAdd(event, 'ce', 'exclude', ${ce.id})">
                        <i data-lucide="minus"></i>
                    </button>
                `;
                wrapper.appendChild(actions);
            } else {
                wrapper.appendChild(createPlaceholder());
            }
            ceGrid.appendChild(wrapper);
        }

        if (team.GrandCraftEssence) {
            const grandTitle = document.createElement('div');
            grandTitle.className = 'team-section-title';
            grandTitle.textContent = '自家Grand额外报酬礼装（0 Cost）';
            teamDiv.appendChild(grandTitle);

            const grandGrid = document.createElement('div');
            grandGrid.className = 'team-grid';
            teamDiv.appendChild(grandGrid);

            const ceInfo = team.GrandCraftEssence;
            const ce = ALL_DATA.craftEssences.find(c => c.id === ceInfo.id);
            if (ce) {
                const wrapper = document.createElement('div');
                wrapper.className = 'result-item-wrapper';
                const div = createAvatar(ce.id, ce.name, ce.img, null, null, true);
                const contributionSpan = document.createElement('div');
                contributionSpan.className = 'contribution';
                contributionSpan.textContent = `羁绊 +${ceInfo.contribution} · 0 Cost`;
                div.appendChild(contributionSpan);
                wrapper.appendChild(div);
                grandGrid.appendChild(wrapper);
            }
        }

        if (team.SupportCraftEssences && team.SupportCraftEssences.length > 0) {
            const supportTitle = document.createElement('div');
            supportTitle.className = 'team-section-title';
            supportTitle.textContent = (document.getElementById('grand-mode')?.checked ? '建议助战Grand礼装（2张）' : '建议助战礼装');
            teamDiv.appendChild(supportTitle);

            const supportGrid = document.createElement('div');
            supportGrid.className = 'team-grid';
            teamDiv.appendChild(supportGrid);

            team.SupportCraftEssences.forEach(ceInfo => {
                const wrapper = document.createElement('div');
                wrapper.className = 'result-item-wrapper';

                const ce = ALL_DATA.craftEssences.find(c => c.id === ceInfo.id);
                if (!ce) return;

                 const div = createAvatar(ce.id, ce.name, ce.img, null, null, true);

                const contributionSpan = document.createElement('div');
                contributionSpan.className = 'contribution';
                contributionSpan.textContent = `羁绊 +${ceInfo.contribution}`;
                div.appendChild(contributionSpan);

                wrapper.appendChild(div);

                const actions = document.createElement('div');
                actions.className = 'result-actions';
                actions.innerHTML = `
                    <button class="outline" title="加入必选" onclick="quickAdd(event, 'ce', 'supportLock', ${ce.id})">
                        <i data-lucide="plus"></i>
                    </button>
                    <button class="outline secondary" title="加入排除" onclick="quickAdd(event, 'ce', 'excludeSupport', ${ce.id})">
                        <i data-lucide="minus"></i>
                    </button>
                `;
                wrapper.appendChild(actions);

                supportGrid.appendChild(wrapper);
            });
        }

        list.appendChild(teamDiv);
    });
    container.hidden = false;
    refreshIcons();
}

function quickAdd(event, type, list, id, extra) {
    event.stopPropagation();
    const key = getSelectionKey(type, list);
    if (key === 'includeSvt') {
        SELECTIONS[key].set(id, extra || 'default');
    } else if (key === 'supportLockCe') {
        const limit = getSupportLimitValue();
        if (!SELECTIONS[key].has(id) && SELECTIONS[key].size >= limit) {
            alert(`锁定助战礼装数量不能超过 ${limit} 个`);
            return;
        }
        SELECTIONS[key].add(id);
    } else {
        SELECTIONS[key].add(id);
    }
    renderSelectionList(type, list);
    saveState();
}

// --- 辅助函数 ---
function openDonateModal() {
    document.getElementById('donate-modal').showModal();
}

function closeDonateModal() {
    document.getElementById('donate-modal').close();
}

function refreshIcons() {
    lucide.createIcons();
}

function createAvatar(id, name, imgSrc, clickHandler, removeHandler, isCe = false) {
    const div = document.createElement('div');
    div.className = 'avatar-container';
    div.dataset.id = id;
    if (clickHandler) div.onclick = clickHandler;

    const img = document.createElement('img');
    img.src = imgSrc;
    img.alt = name;
    img.title = name;
    if (isCe) img.className = 'ce-img';
    div.appendChild(img);

    const nameSpan = document.createElement('span');
    nameSpan.className = 'name';
    nameSpan.textContent = name;
    div.appendChild(nameSpan);

    if (removeHandler) {
        const removeBtn = document.createElement('button');
        removeBtn.className = 'remove-btn';
        removeBtn.innerHTML = '<i data-lucide="x" style="width: 16px; height: 16px; stroke-width: 3;"></i>';
        removeBtn.onclick = (e) => {
            e.stopPropagation();
            removeHandler();
        };
        div.appendChild(removeBtn);
    }
    return div;
}

function createPlaceholder() {
    const div = document.createElement('div');
    div.className = 'placeholder';
    return div;
}
