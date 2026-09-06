let currentModal = {type: '', list: ''};

function openModal(type, list) {
    currentModal = {type, list};
    if (list === 'owned') {
        document.getElementById('modal-title').textContent = type === 'svt' ? '管理我的英灵 Box / 羁绊培养档案' : '管理我拥有的羁绊礼装';
    } else if (type === 'ce' && list === 'supportLock') {
        document.getElementById('modal-title').textContent = '选择锁定助战礼装';
    } else if (type === 'ce' && list === 'excludeSupport') {
        document.getElementById('modal-title').textContent = '选择排除助战礼装';
    } else {
        document.getElementById('modal-title').textContent = `选择${list === 'include' ? '必选' : '排除'}${type === 'svt' ? '从者' : '礼装'}`;
    }
    document.getElementById('modal-search').value = '';

    const batchActions = document.getElementById('modal-batch-actions');
    if (batchActions) batchActions.style.display = list === 'owned' ? 'flex' : 'none';
    document.querySelectorAll('.bond-batch-action').forEach(button => {
        button.style.display = (type === 'svt' && list === 'owned') ? 'inline-flex' : 'none';
    });

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

function getFilteredModalItems(filter = '') {
    const items = currentModal.type === 'svt' ? getActiveServants() : ALL_DATA.craftEssences;
    return items.filter(item => {
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
    });
}

function populateModalGrid(filter = '') {
    const grid = document.getElementById('modal-grid');
    grid.innerHTML = '';

    getFilteredModalItems(filter).forEach(item => {
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

        if (currentModal.type === 'svt' && currentModal.list === 'owned') {
            const status = getServantBondStatus(item.id);
            if (status === 10 || status === 15) {
                const badge = document.createElement('span');
                badge.className = `bond-status-badge bond-${status}`;
                badge.textContent = `绊${status}`;
                avatar.appendChild(badge);
            }

            const controls = document.createElement('div');
            controls.className = 'bond-status-controls';
            [
                {value: 0, label: '未满'},
                {value: 10, label: '绊10'},
                {value: 15, label: '绊15'}
            ].forEach(option => {
                const button = document.createElement('button');
                button.type = 'button';
                button.className = 'bond-status-btn';
                button.textContent = option.label;
                button.classList.toggle('is-active', selected && status === option.value);
                button.onclick = event => {
                    event.stopPropagation();
                    setServantBondStatus(item.id, option.value);
                    populateModalGrid(document.getElementById('modal-search').value);
                };
                controls.appendChild(button);
            });
            avatar.appendChild(controls);

            const profile = getServantProfile(item.id);
            if (profile.imported || profile.bondRank > 0) {
                const progress = document.createElement('div');
                progress.className = 'bond-profile-progress';
                const rankText = `Lv.${profile.bondRank || 0}${profile.bondRankMax ? '/' + profile.bondRankMax : ''}`;
                const nextText = profile.bondNext > 0 ? ` · Next ${formatNumber(profile.bondNext)}` : '';
                const remaining = getProfileRemaining(profile);
                const remainingText = remaining === null ? '' : ` · 距目标 ${formatNumber(remaining)}`;
                progress.textContent = `${rankText} · Total ${formatNumber(profile.bondTotal)}${nextText}${remainingText}`;
                avatar.appendChild(progress);
            }

            const profileControls = document.createElement('div');
            profileControls.className = 'servant-profile-controls';
            profileControls.onclick = event => event.stopPropagation();

            const roleSelect = document.createElement('select');
            roleSelect.className = 'profile-mini-select';
            roleSelect.title = '角色定位：司机会降低长期培养权重，老板会提高';
            [
                ['normal', '普通'],
                ['driver', '司机'],
                ['passenger', '老板']
            ].forEach(([value, label]) => {
                const option = document.createElement('option');
                option.value = value;
                option.textContent = label;
                roleSelect.appendChild(option);
            });
            roleSelect.value = profile.role || 'normal';
            roleSelect.onchange = event => {
                event.stopPropagation();
                SELECTIONS.ownedSvt.add(item.id);
                updateServantProfile(item.id, {role: roleSelect.value});
                populateModalGrid(document.getElementById('modal-search').value);
            };

            const prioritySelect = document.createElement('select');
            prioritySelect.className = 'profile-mini-select';
            prioritySelect.title = '手动培养优先级';
            [
                ['auto', '优先:自动'],
                ['high', '优先:高'],
                ['low', '优先:低']
            ].forEach(([value, label]) => {
                const option = document.createElement('option');
                option.value = value;
                option.textContent = label;
                prioritySelect.appendChild(option);
            });
            prioritySelect.value = profile.priority || 'auto';
            prioritySelect.onchange = event => {
                event.stopPropagation();
                SELECTIONS.ownedSvt.add(item.id);
                updateServantProfile(item.id, {priority: prioritySelect.value});
                populateModalGrid(document.getElementById('modal-search').value);
            };

            const targetSelect = document.createElement('select');
            targetSelect.className = 'profile-mini-select';
            targetSelect.title = '长期培养目标';
            [[10, '目标:绊10'], [15, '目标:绊15']].forEach(([value, label]) => {
                const option = document.createElement('option');
                option.value = String(value);
                option.textContent = label;
                targetSelect.appendChild(option);
            });
            targetSelect.value = String(profile.targetRank || 10);
            targetSelect.onchange = event => {
                event.stopPropagation();
                SELECTIONS.ownedSvt.add(item.id);
                updateServantProfile(item.id, {targetRank: Number(targetSelect.value)});
                populateModalGrid(document.getElementById('modal-search').value);
            };

            const fixedLabel = document.createElement('label');
            fixedLabel.className = 'profile-fixed-label';
            fixedLabel.title = '固定出场是硬约束；适合当前周回必须使用的司机/打手';
            const fixedInput = document.createElement('input');
            fixedInput.type = 'checkbox';
            fixedInput.checked = !!profile.fixed;
            fixedInput.onchange = event => {
                event.stopPropagation();
                SELECTIONS.ownedSvt.add(item.id);
                updateServantProfile(item.id, {fixed: fixedInput.checked});
                populateModalGrid(document.getElementById('modal-search').value);
            };
            fixedLabel.append(fixedInput, '固定');

            profileControls.append(roleSelect, prioritySelect, targetSelect, fixedLabel);
            avatar.appendChild(profileControls);
        }
        grid.appendChild(avatar);
    });
    refreshIcons();
}

function togglePickerItem(id) {
    const {type, list} = currentModal;
    const selected = isPickerItemSelected(id);
    if (selected) {
        if (type === 'svt' && list === 'include') {
            SELECTIONS.includeSvt.delete(id);
        } else {
            SELECTIONS[getSelectionKey(type, list)].delete(id);
            if (type === 'svt' && list === 'owned') {
                SELECTIONS.bond10Svt.delete(id);
                SELECTIONS.bond15Svt.delete(id);
                updateOwnershipSummary();
            }
        }
        renderSelectionList(type, list);
        saveState();
        populateModalGrid(document.getElementById('modal-search').value);
        return;
    }

    if (type === 'svt' && list === 'include') {
        const servant = getActiveServants().find(item => item.id === id);
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

function setOwnedForCurrentFilter(selected) {
    if (currentModal.list !== 'owned') return;
    const key = getSelectionKey(currentModal.type, currentModal.list);
    const set = SELECTIONS[key];
    const filter = document.getElementById('modal-search').value;
    for (const item of getFilteredModalItems(filter)) {
        if (selected) {
            set.add(item.id);
        } else {
            set.delete(item.id);
            if (currentModal.type === 'svt') {
                SELECTIONS.bond10Svt.delete(item.id);
                SELECTIONS.bond15Svt.delete(item.id);
            }
        }
    }
    saveState();
    updateOwnershipSummary();
    populateModalGrid(filter);
}


function setBondStatusForCurrentFilter(status) {
    if (currentModal.type !== 'svt' || currentModal.list !== 'owned') return;
    const filter = document.getElementById('modal-search').value;
    for (const item of getFilteredModalItems(filter)) {
        SELECTIONS.ownedSvt.add(item.id);
        SELECTIONS.bond10Svt.delete(item.id);
        SELECTIONS.bond15Svt.delete(item.id);
        if (status === 10) SELECTIONS.bond10Svt.add(item.id);
        if (status === 15) SELECTIONS.bond15Svt.add(item.id);
        const profile = getServantProfile(item.id);
        if (!profile.imported && (status === 10 || status === 15)) {
            SERVANT_PROFILES.set(item.id, {...profile, bondRank: status, bondRankMax: status, targetRank: status});
        }
    }
    saveState();
    updateOwnershipSummary();
    populateModalGrid(filter);
}

document.getElementById('modal-search').addEventListener('input', event => {
    populateModalGrid(event.target.value);
});
