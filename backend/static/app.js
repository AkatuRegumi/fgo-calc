let CURRENT_USER = null;
let ALL_DATA = { servants: [], craftEssences: [], traits: {} };
let SELECTIONS = {
    includeSvt: new Map(),
    excludeSvt: new Set(),
    includeCe: new Set(),
    excludeCe: new Set(),
    supportLockCe: new Set(),
    excludeSupportCe: new Set(),
    allowTraits: new Set(),
    selectedEvents: new Set()
};
let HISTORY_ITEMS = [];

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

function updateAuthUI() {
    const welcome = document.getElementById('user-welcome');
    const usernameSpan = document.getElementById('current-username');
    const btnLogin = document.getElementById('btn-login-modal');
    const btnLogout = document.getElementById('btn-logout');
    const btnHistory = document.getElementById('btn-history');
    
    if (CURRENT_USER) {
        usernameSpan.textContent = CURRENT_USER;
        welcome.style.display = 'inline';
        btnLogout.style.display = 'inline-block';
        if (btnHistory) btnHistory.style.display = 'inline-block';
        btnLogin.style.display = 'none';
    } else {
        welcome.style.display = 'none';
        btnLogout.style.display = 'none';
        if (btnHistory) btnHistory.style.display = 'none';
        btnLogin.style.display = 'inline-block';
    }
}

async function handleLogin() {
    const user = document.getElementById('login-username').value.trim();
    const pass = document.getElementById('login-password').value;
    if (!user) return;
    
    const btn = document.getElementById('btn-submit-login');
    btn.setAttribute('aria-busy', 'true');
    btn.textContent = '请稍候...';
    
    try {
        let res = await fetch('/api/login', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({username: user, password: pass})
        });
        
        let data = await res.json();
        if (!res.ok) {
            if (data.error === "user not found") {
                res = await fetch('/api/register', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({username: user, password: pass})
                });
                data = await res.json();
                if (!res.ok) throw new Error(data.error);
                
                showFlash('注册成功并已登录', 'success');
                data.state = localStorage.getItem(STORAGE_KEY) || "";
            } else {
                throw new Error(data.error);
            }
        } else {
            showFlash('登录成功', 'success');
        }
        
        CURRENT_USER = data.username || user;
        document.getElementById('login-modal').close();
        updateAuthUI();
        
        if (data.state) {
            localStorage.setItem(STORAGE_KEY, data.state);
            loadState();
            renderEventSelection();
            renderMainClassFilters();
            renderExcludeSvtClassFilters();
        } else {
            syncState();
        }
    } catch (err) {
        showFlash('登录失败: ' + err.message, 'error', 6000);
    } finally {
        btn.removeAttribute('aria-busy');
        btn.textContent = '登录 / 注册';
    }
}

async function logout() {
    try {
        await fetch('/api/logout', { method: 'POST' });
    } catch(e) {}
    CURRENT_USER = null;
    updateAuthUI();
    showFlash('已退出登录，当前为访客状态', 'info');
}

async function syncState() {
    if (!CURRENT_USER) return;
    const state = localStorage.getItem(STORAGE_KEY);
    if (!state) return;
    
    try {
        await fetch('/api/state', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({state: state})
        });
    } catch(e) {
        console.error("Failed to sync state to server:", e);
    }
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

    if (changed) {
        renderSelectionList('ce', 'include');
        renderSelectionList('ce', 'exclude');
        renderSelectionList('ce', 'supportLock');
        renderSelectionList('ce', 'excludeSupport');
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

function saveState() {
    const state = {
        config: {
            costLimit: document.getElementById('cost-limit').value,
            svtLimit: document.getElementById('svt-limit').value,
            ceLimit: document.getElementById('ce-limit').value,
            supportLimit: document.getElementById('support-limit').value,
            baseBond: document.getElementById('base-bond').value,
            server: document.getElementById('server-select').value,
            enableEventBonus: document.getElementById('enable-event-bonus').checked,
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
        },
        ui: {
            collapsedSections: { ...UI_STATE.collapsedSections },
            excludeSvtClassFilters: Array.from(EXCLUDE_SVT_CLASS_FILTER)
        }
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    if (CURRENT_USER) {
        clearTimeout(window.syncTimeout);
        window.syncTimeout = setTimeout(() => {
            syncState();
        }, 1000);
    }
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
            if (state.config.baseBond !== undefined) document.getElementById('base-bond').value = state.config.baseBond;
            if (state.config.server !== undefined) document.getElementById('server-select').value = state.config.server;
            if (state.config.enableEventBonus !== undefined) document.getElementById('enable-event-bonus').checked = state.config.enableEventBonus;
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
            
            renderMainClassFilters();
            renderSelectionList('svt', 'include');
            renderSelectionList('svt', 'exclude');
            renderSelectionList('ce', 'include');
            renderSelectionList('ce', 'exclude');
            renderSelectionList('ce', 'supportLock');
            renderSelectionList('ce', 'excludeSupport');
            renderEventSelection();
        }
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
    } catch (e) {
        console.error('Failed to load state:', e);
    }
}

// --- 数据加载与初始化 ---
async function initApp() {
    lucide.createIcons();
    initCollapseToggles();
    renderExcludeSvtClassFilters();
    
    // 为所有基础配置输入添加自动保存
    ['cost-limit', 'svt-limit', 'ce-limit', 'support-limit', 'base-bond', 'server-select', 'enable-event-bonus'].forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            el.addEventListener('change', () => {
                if (id === 'server-select') {
                    SELECTIONS.selectedEvents.clear();
                    renderEventSelection(true);
                    if (pruneInvalidSelectionsByServer()) {
                        saveState();
                    }
                } else if (id === 'enable-event-bonus') {
                    renderEventSelection(el.checked);
                } else if (id === 'support-limit') {
                    refreshSupportLockLimitText();
                    enforceSupportLockLimit(true);
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

    const [meResult, dataResult] = await Promise.allSettled([
        fetch('/api/me'),
        fetch('/api/data')
    ]);

    if (meResult.status === 'fulfilled' && meResult.value.ok) {
        try {
            const data = await meResult.value.json();
            CURRENT_USER = data.username;
        } catch (error) {
            console.error('Failed to parse user data:', error);
        }
    }
    updateAuthUI();

    try {
        if (dataResult.status === 'rejected') throw dataResult.reason;
        if (!dataResult.value.ok) throw new Error(`Server error: ${dataResult.value.status}`);
        ALL_DATA = await dataResult.value.json();
        if (ALL_DATA.dataUpdatedAt > 0) {
            const updatedAt = new Date(ALL_DATA.dataUpdatedAt * 1000);
            const label = document.getElementById('data-updated-at');
            label.textContent = `数据源更新于 ${updatedAt.toLocaleString('zh-CN', {hour12: false})}`;
            label.hidden = false;
        }
        ALL_DATA.servants.sort((a, b) => a.id - b.id);
        ALL_DATA.craftEssences.sort((a, b) => a.id - b.id);
       
       // 加载持久化状态
       loadState();
       refreshSupportLockLimitText();
       const lockChanged = pruneInvalidSelectionsByServer() || enforceSupportLockLimit();
       if (lockChanged) {
           saveState();
       }
       
       renderMainClassFilters();
       renderEventSelection();
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
    ALL_DATA.servants.forEach(svt => {
        const bonuses = svt.event_bonuses ? svt.event_bonuses[server] : [];
        const extraBonuses = svt.event_extra_bonuses ? svt.event_extra_bonuses[server] : [];
        [...bonuses, ...extraBonuses].forEach(b => {
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
    const key = getSelectionKey(type, list);
    const container = document.getElementById(getSelectionContainerId(type, list));
    if (!container) return;
    container.innerHTML = '';
    const items = type === 'svt' ? ALL_DATA.servants : ALL_DATA.craftEssences;
    
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
        const params = new URLSearchParams();
        params.append('costlimit', document.getElementById('cost-limit').value);
        params.append('svtlimit', document.getElementById('svt-limit').value);
        params.append('celimit', document.getElementById('ce-limit').value);
        params.append('supportlimit', document.getElementById('support-limit').value);
        params.append('basebond', document.getElementById('base-bond').value);
        params.append('server', document.getElementById('server-select').value);
        params.append('enable_event_bonus', document.getElementById('enable-event-bonus').checked);
        SELECTIONS.selectedEvents.forEach(id => params.append('selected_events', id));

        SELECTIONS.allowTraits.forEach(id => params.append('allowtraits', id));
        SELECTIONS.includeSvt.forEach((diff, id) => {
            params.append('includesvt', id);
            params.append('includesvtdiff', diff);
        });
        SELECTIONS.excludeSvt.forEach(id => params.append('excludesvt', id));
        SELECTIONS.includeCe.forEach(id => params.append('includece', id));
        SELECTIONS.excludeCe.forEach(id => params.append('excludece', id));
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
        if (CURRENT_USER) {
            postHistory(results.teams);
        }
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
        let extraBonuses = svt.event_extra_bonuses ? svt.event_extra_bonuses[server] : [];
        
        // Filter based on selected events
        bonuses = bonuses.filter(b => SELECTIONS.selectedEvents.has(b.id));
        extraBonuses = extraBonuses.filter(b => SELECTIONS.selectedEvents.has(b.id));

        if ((bonuses && bonuses.length > 0) || (extraBonuses && extraBonuses.length > 0)) {
            const bonusContainer = document.createElement('div');
            bonusContainer.className = 'event-bonus-container';
            
            bonuses.forEach(b => {
                const item = document.createElement('div');
                item.className = 'event-bonus-item';
                item.innerHTML = `<span class="event-bonus-val">+${b.bonus}%</span><span class="event-bonus-name" title="${b.name}">${b.name}</span>`;
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
        header.innerHTML = `<h5><i data-lucide="award" style="width: 20px; vertical-align: middle; margin-right: 8px;"></i> 方案 ${index + 1}</h5><p><mark>总羁绊: ${team.TotalBond}</mark> <small>| 总Cost: ${team.TotalCost}</small></p>`;
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
                const svt = ALL_DATA.servants.find(s => s.id === svtId);
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

        if (team.SupportCraftEssences && team.SupportCraftEssences.length > 0) {
            const supportTitle = document.createElement('div');
            supportTitle.className = 'team-section-title';
            supportTitle.textContent = '建议助战礼装';
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

async function postHistory(teams) {
    const state = localStorage.getItem(STORAGE_KEY);
    if (!state) return;
    try {
        await fetch('/api/history', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                state: state,
                result: JSON.stringify(teams)
            })
        });
    } catch (e) {
        console.error("Failed to post history", e);
    }
}

async function openHistoryModal() {
    const container = document.getElementById('history-list-container');
    container.innerHTML = '<p style="text-align: center; color: var(--pico-muted-color);">加载中...</p>';
    const modal = document.getElementById('history-modal');
    if (!modal.open) modal.showModal();

    try {
        const res = await fetch(`/api/history`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);

        container.innerHTML = '';
        if (!data.history || data.history.length === 0) {
            HISTORY_ITEMS = [];
            container.innerHTML = '<p style="text-align: center; color: var(--pico-muted-color);">暂无历史记录。</p>';
            return;
        }

        HISTORY_ITEMS = data.history;
        renderHistoryList();
    } catch(err) {
        container.innerHTML = `<p style="text-align: center; color: var(--pico-form-element-invalid-border-color);">加载失败: ${err.message}</p>`;
    }
}

function renderHistoryList() {
    const container = document.getElementById('history-list-container');
    container.innerHTML = '';
    const pinned = HISTORY_ITEMS.filter(item => item.pinned);
    const recent = HISTORY_ITEMS.filter(item => !item.pinned);
    if (pinned.length > 0) renderHistorySection(container, `已固定 (${pinned.length}/15)`, pinned, 'pin');
    if (recent.length > 0) renderHistorySection(container, `最近记录 (${recent.length}/10)`, recent, 'history');
    if (HISTORY_ITEMS.length === 0) {
        container.innerHTML = '<p style="text-align: center; color: var(--pico-muted-color);">暂无历史记录。</p>';
    }
    refreshIcons();
}

function renderHistorySection(container, title, items, icon) {
    const heading = document.createElement('div');
    heading.className = 'history-section-title';
    heading.innerHTML = `<i data-lucide="${icon}" style="width: 14px;"></i><span></span>`;
    heading.querySelector('span').textContent = title;
    container.appendChild(heading);

    items.forEach(item => {
        const entry = document.createElement('div');
        entry.className = 'history-entry';

        const main = document.createElement('div');
        main.className = 'history-entry-main';
        const open = document.createElement('button');
        open.className = 'outline secondary history-entry-open';
        const name = document.createElement('span');
        name.className = 'history-entry-name';
        name.textContent = item.name || new Date(item.timestamp).toLocaleString();
        const time = document.createElement('span');
        time.className = 'history-entry-time';
        time.textContent = item.name ? new Date(item.timestamp).toLocaleString() : '点击查看方案';
        open.append(name, time);
        open.onclick = () => restoreHistory(item);
        main.appendChild(open);

        const actions = document.createElement('div');
        actions.className = 'history-entry-actions';
        actions.append(
            createHistoryAction('pencil', '重命名', () => beginRenameHistory(item, main)),
            createHistoryAction(item.pinned ? 'pin-off' : 'pin', item.pinned ? '取消固定' : '固定', () => toggleHistoryPinned(item))
        );
        entry.append(main, actions);
        container.appendChild(entry);
    });
}

function createHistoryAction(icon, title, handler) {
    const button = document.createElement('button');
    button.className = 'outline secondary';
    button.type = 'button';
    button.title = title;
    button.setAttribute('aria-label', title);
    button.innerHTML = `<i data-lucide="${icon}"></i>`;
    button.onclick = handler;
    return button;
}

async function toggleHistoryPinned(item) {
    try {
        const res = await fetch(`/api/history/${item.id}/pin`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({pinned: !item.pinned})
        });
        const data = await res.json();
        if (!res.ok) {
            if (data.error === 'pinned history limit reached') throw new Error('最多只能固定 15 条记录');
            throw new Error(data.error);
        }
        item.pinned = !item.pinned;
        if (!item.pinned) {
            const recent = HISTORY_ITEMS.filter(entry => !entry.pinned)
                .sort((a, b) => b.timestamp - a.timestamp || b.id - a.id);
            const retained = new Set(recent.slice(0, 10).map(entry => entry.id));
            HISTORY_ITEMS = HISTORY_ITEMS.filter(entry => entry.pinned || retained.has(entry.id));
        }
        renderHistoryList();
    } catch (err) {
        showFlash(`操作失败: ${err.message}`, 'error', 6000);
    }
}

function beginRenameHistory(item, main) {
    if (main.querySelector('input')) return;
    const open = main.querySelector('.history-entry-open');
    const input = document.createElement('input');
    input.className = 'history-entry-name-input';
    input.type = 'text';
    input.maxLength = 100;
    input.value = item.name || '';
    input.placeholder = new Date(item.timestamp).toLocaleString();
    open.hidden = true;
    main.prepend(input);
    input.focus();
    input.select();

    let finished = false;
    const finish = async save => {
        if (finished) return;
        finished = true;
        if (save) await renameHistory(item, input.value);
        input.remove();
        open.hidden = false;
    };
    input.onkeydown = event => {
        if (event.key === 'Enter') {
            event.preventDefault();
            input.blur();
        } else if (event.key === 'Escape') {
            event.preventDefault();
            finish(false);
        }
    };
    input.onblur = () => finish(true);
}

async function renameHistory(item, name) {
    const trimmedName = name.trim();
    if ([...trimmedName].length > 100) {
        showFlash('记录名称不能超过 100 个字符', 'error');
        return;
    }
    try {
        const res = await fetch(`/api/history/${item.id}/name`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({name: trimmedName})
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        item.name = trimmedName;
        renderHistoryList();
    } catch (err) {
        showFlash(`重命名失败: ${err.message}`, 'error', 6000);
    }
}

function restoreHistory(item) {
    if (item.state) {
        localStorage.setItem(STORAGE_KEY, item.state);
        loadState();
        renderEventSelection();
        renderMainClassFilters();
        renderExcludeSvtClassFilters();
        saveState();
    }
    if (item.result) {
        try {
            const teams = JSON.parse(item.result);
            renderResults(teams);
        } catch(e) {}
    }
    document.getElementById('history-modal').close();
    window.scrollTo({ top: document.getElementById('results-container').offsetTop - 50, behavior: 'smooth' });
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
