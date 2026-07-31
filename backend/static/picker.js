let currentModal = {type: '', list: ''};

function openModal(type, list) {
    currentModal = {type, list};
    if (type === 'ce' && list === 'supportLock') {
        document.getElementById('modal-title').textContent = '选择锁定助战礼装';
    } else if (type === 'ce' && list === 'excludeSupport') {
        document.getElementById('modal-title').textContent = '选择排除助战礼装';
    } else {
        document.getElementById('modal-title').textContent = `选择${list === 'include' ? '必选' : '排除'}${type === 'svt' ? '从者' : '礼装'}`;
    }
    document.getElementById('modal-search').value = '';

    const filterContainer = document.getElementById('modal-filter-container');
    if (type === 'svt') {
        filterContainer.style.display = 'block';
        renderStarFilters();
        renderClassFilters();
    } else {
        filterContainer.style.display = 'none';
    }

    populateModalGrid();
    document.getElementById('add-modal').showModal();
}

function closeModal() {
    document.getElementById('add-modal').close();
}

function renderStarFilters() {
    const container = document.getElementById('star-filters');
    container.innerHTML = '';
    [0, 1, 2, 3, 4, 5].forEach(star => {
        const label = document.createElement('label');
        label.className = 'star-filter-label';
        const input = document.createElement('input');
        input.type = 'checkbox';
        input.checked = FILTER_STATE.stars.has(STAR_TRAITS[star]);
        input.onchange = event => {
            if (event.target.checked) FILTER_STATE.stars.add(STAR_TRAITS[star]);
            else FILTER_STATE.stars.delete(STAR_TRAITS[star]);
            populateModalGrid(document.getElementById('modal-search').value);
        };
        label.append(input, `${star}星`);
        container.appendChild(label);
    });
}

function renderClassFilters() {
    const container = document.getElementById('class-filters');
    container.innerHTML = '';

    const bronzeAll = document.createElement('img');
    bronzeAll.src = '/static/fgo-icon/铜卡All.png';
    bronzeAll.className = 'class-icon-btn';
    bronzeAll.title = '全部取消';
    bronzeAll.onclick = () => {
        FILTER_STATE.classes.clear();
        renderClassFilters();
        populateModalGrid(document.getElementById('modal-search').value);
    };
    container.appendChild(bronzeAll);

    const goldAll = document.createElement('img');
    goldAll.src = '/static/fgo-icon/金卡All.png';
    goldAll.className = 'class-icon-btn';
    goldAll.title = '全选';
    goldAll.onclick = () => {
        CLASS_TRAITS.forEach(({trait}) => {
            if (Array.isArray(trait)) trait.forEach(value => FILTER_STATE.classes.add(value));
            else FILTER_STATE.classes.add(trait);
        });
        renderClassFilters();
        populateModalGrid(document.getElementById('modal-search').value);
    };
    container.appendChild(goldAll);

    CLASS_TRAITS.forEach(({name, trait}) => {
        const selected = Array.isArray(trait)
            ? trait.every(value => FILTER_STATE.classes.has(value))
            : FILTER_STATE.classes.has(trait);
        const image = document.createElement('img');
        image.className = 'class-icon-btn';
        image.src = `/static/fgo-icon/${selected ? '金卡' : '铜卡'}${name}.png`;
        image.title = name;
        image.onclick = () => {
            const update = selected ? 'delete' : 'add';
            if (Array.isArray(trait)) trait.forEach(value => FILTER_STATE.classes[update](value));
            else FILTER_STATE.classes[update](trait);
            renderClassFilters();
            populateModalGrid(document.getElementById('modal-search').value);
        };
        container.appendChild(image);
    });
}

function isPickerItemSelected(id) {
    const {type, list} = currentModal;
    if (type === 'svt' && list === 'include') return SELECTIONS.includeSvt.has(id);
    return SELECTIONS[getSelectionKey(type, list)].has(id);
}

function populateModalGrid(filter = '') {
    const grid = document.getElementById('modal-grid');
    grid.innerHTML = '';
    const items = currentModal.type === 'svt' ? ALL_DATA.servants : ALL_DATA.craftEssences;

    items.filter(item => {
        if (!item.name?.toLowerCase().includes(filter.toLowerCase())) return false;
        if (currentModal.type === 'svt') {
            const detail = item.diff.default || Object.values(item.diff)[0];
            if (!detail) return false;
            if (FILTER_STATE.stars.size > 0 && !detail.traits.some(value => FILTER_STATE.stars.has(value))) return false;
            if (FILTER_STATE.classes.size > 0 && !detail.traits.some(value => FILTER_STATE.classes.has(value))) return false;
        } else {
            const server = document.getElementById('server-select').value;
            if (item.server === 'JP' && server !== 'JP') return false;
            if ((currentModal.list === 'supportLock' || currentModal.list === 'excludeSupport') && !isSupportCandidateCe(item)) return false;
        }
        return true;
    }).forEach(item => {
        const detail = currentModal.type === 'svt' ? item.diff.default || Object.values(item.diff)[0] : null;
        const image = currentModal.type === 'svt' ? detail?.img || '' : item.img;
        const selected = isPickerItemSelected(item.id);
        const avatar = createAvatar(item.id, item.name, image, () => togglePickerItem(item.id), null, currentModal.type === 'ce');
        avatar.classList.toggle('is-selected', selected);
        avatar.setAttribute('role', 'checkbox');
        avatar.setAttribute('aria-checked', String(selected));
        avatar.tabIndex = 0;
        avatar.onkeydown = event => {
            if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                togglePickerItem(item.id);
            }
        };
        const check = document.createElement('span');
        check.className = 'picker-check';
        check.innerHTML = '<i data-lucide="check"></i>';
        avatar.appendChild(check);
        if (currentModal.type === 'svt') renderEventBonuses(avatar, item);
        grid.appendChild(avatar);
    });
    refreshIcons();
}

function togglePickerItem(id) {
    const {type, list} = currentModal;
    const selected = isPickerItemSelected(id);
    if (selected) {
        if (type === 'svt' && list === 'include') SELECTIONS.includeSvt.delete(id);
        else SELECTIONS[getSelectionKey(type, list)].delete(id);
        renderSelectionList(type, list);
        saveState();
        populateModalGrid(document.getElementById('modal-search').value);
        return;
    }

    if (type === 'svt' && list === 'include') {
        const servant = ALL_DATA.servants.find(item => item.id === id);
        if (!servant) return;
        const diffs = Object.keys(servant.diff);
        if (diffs.length > 1) {
            openSvtDiffModal(servant);
            return;
        }
        addSvtWithDiff(id, diffs[0] || 'default');
        return;
    }

    if (type === 'ce' && list === 'supportLock') {
        const limit = getSupportLimitValue();
        if (SELECTIONS.supportLockCe.size >= limit) {
            showFlash(`锁定助战礼装数量不能超过 ${limit} 个`, 'error');
            return;
        }
    }
    SELECTIONS[getSelectionKey(type, list)].add(id);
    renderSelectionList(type, list);
    saveState();
    populateModalGrid(document.getElementById('modal-search').value);
}

document.getElementById('modal-search').addEventListener('input', event => {
    populateModalGrid(event.target.value);
});
