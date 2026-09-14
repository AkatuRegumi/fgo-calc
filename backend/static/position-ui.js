(() => {
    if (typeof renderResults !== 'function') return;

    const FORCE_SUPPORT_FRONT_KEY = 'fgo-calc-force-support-front-v1';
    const FRONT_LOCKED_SERVANTS_KEY = 'fgo-calc-front-locked-servants-v1';
    const originalRenderResults = renderResults;
    const originalRenderSelectionList = typeof renderSelectionList === 'function' ? renderSelectionList : null;
    const nativeFetch = window.fetch.bind(window);
    let selectionStateReady = false;

    function loadFrontLockedServants() {
        try {
            const raw = JSON.parse(localStorage.getItem(FRONT_LOCKED_SERVANTS_KEY) || '[]');
            if (!Array.isArray(raw)) return new Set();
            return new Set(raw.map(Number).filter(Number.isInteger));
        } catch (_) {
            return new Set();
        }
    }

    const frontLockedServants = loadFrontLockedServants();

    function saveFrontLockedServants() {
        localStorage.setItem(
            FRONT_LOCKED_SERVANTS_KEY,
            JSON.stringify(Array.from(frontLockedServants).sort((a, b) => a - b))
        );
    }

    function pruneFrontLockedServants() {
        if (!selectionStateReady) return false;
        if (typeof SELECTIONS === 'undefined' || !(SELECTIONS.includeSvt instanceof Map)) return false;
        const included = new Set(Array.from(SELECTIONS.includeSvt.keys()).map(Number));
        let changed = false;
        for (const id of Array.from(frontLockedServants)) {
            if (!included.has(id)) {
                frontLockedServants.delete(id);
                changed = true;
            }
        }
        if (changed) saveFrontLockedServants();
        return changed;
    }

    function multiplierText(percent) {
        const value = 1 + Number(percent || 0) / 100;
        return `×${value.toFixed(2)}`;
    }

    function showPositionMessage(message, type = 'info') {
        if (typeof showFlash === 'function') {
            showFlash(message, type, 5500);
        } else {
            alert(message);
        }
    }

    function ensureStyles() {
        if (document.getElementById('position-optimizer-style')) return;
        const style = document.createElement('style');
        style.id = 'position-optimizer-style';
        style.textContent = `
            .position-badge {
                position: absolute;
                top: 0.35rem;
                left: 0.35rem;
                z-index: 5;
                padding: 0.12rem 0.35rem;
                border-radius: 999px;
                background: color-mix(in srgb, var(--pico-primary) 88%, transparent);
                color: var(--pico-primary-inverse);
                font-size: 0.62rem;
                font-weight: 700;
                line-height: 1.25;
                pointer-events: none;
                box-shadow: 0 1px 4px rgba(0,0,0,0.18);
            }
            .position-summary {
                margin-left: 0.25rem;
            }
            .support-position-mode-panel {
                margin: 0.65rem 0 1rem;
                padding: 0.75rem 0.9rem;
                border: 1px solid var(--pico-muted-border-color);
                border-radius: var(--pico-border-radius);
            }
            .support-position-mode-panel label {
                margin-bottom: 0.2rem;
                font-weight: 600;
            }
            .support-position-mode-panel small {
                display: block;
                opacity: 0.78;
            }
            #include-svt-list .avatar-container {
                position: relative;
            }
            .front-lock-btn {
                position: absolute;
                top: 0.35rem;
                left: 0.35rem;
                z-index: 12;
                width: auto;
                min-width: 2.1rem;
                height: 1.45rem;
                margin: 0;
                padding: 0.05rem 0.28rem;
                border: 1px solid var(--pico-muted-border-color);
                border-radius: 999px;
                background: color-mix(in srgb, var(--pico-card-background-color) 92%, transparent);
                color: var(--pico-muted-color);
                font-size: 0.62rem;
                font-weight: 700;
                line-height: 1;
                box-shadow: 0 1px 4px rgba(0,0,0,0.15);
            }
            .front-lock-btn.is-locked {
                border-color: var(--pico-primary);
                background: var(--pico-primary);
                color: var(--pico-primary-inverse);
            }
        `;
        document.head.appendChild(style);
    }

    function currentSupportPositionMode() {
        return document.getElementById('force-support-front')?.checked ? 'front' : 'auto';
    }

    function maxSelfFrontLocks() {
        return currentSupportPositionMode() === 'front' ? 2 : 3;
    }

    function validateSupportFrontConflict(showMessage) {
        if (!selectionStateReady) return true;
        pruneFrontLockedServants();
        const checkbox = document.getElementById('force-support-front');
        if (!checkbox || !checkbox.checked || frontLockedServants.size <= 2) return true;
        checkbox.checked = false;
        localStorage.setItem(FORCE_SUPPORT_FRONT_KEY, 'false');
        if (showMessage) {
            showPositionMessage('当前已锁定3名自家前排，助战没有可用的前排位置。请先解除至少1名前排锁定。', 'error');
        }
        return false;
    }

    function decorateIncludeServantLocks() {
        if (!selectionStateReady) return;
        pruneFrontLockedServants();
        const cards = Array.from(document.querySelectorAll('#include-svt-list .avatar-container'));
        cards.forEach(card => {
            const id = Number(card.dataset.id);
            if (!Number.isInteger(id)) return;
            let button = card.querySelector('.front-lock-btn');
            if (!button) {
                button = document.createElement('button');
                button.type = 'button';
                button.className = 'front-lock-btn';
                button.addEventListener('click', event => {
                    event.preventDefault();
                    event.stopPropagation();
                    const locked = frontLockedServants.has(id);
                    if (locked) {
                        frontLockedServants.delete(id);
                        saveFrontLockedServants();
                        decorateIncludeServantLocks();
                        return;
                    }

                    pruneFrontLockedServants();
                    const limit = maxSelfFrontLocks();
                    if (frontLockedServants.size >= limit) {
                        const message = currentSupportPositionMode() === 'front'
                            ? '助战已固定前排，自家前排最多只能锁定2骑。请先解除一个前排锁定。'
                            : '自家前排最多只能锁定3骑。';
                        showPositionMessage(message, 'error');
                        return;
                    }

                    frontLockedServants.add(id);
                    saveFrontLockedServants();
                    decorateIncludeServantLocks();
                });
                card.appendChild(button);
            }

            const locked = frontLockedServants.has(id);
            button.classList.toggle('is-locked', locked);
            button.textContent = locked ? '🔒前' : '🔓前';
            button.title = locked
                ? '解除前排锁定'
                : '锁定前排：该必选从者必须占用自家前排位置';
            button.setAttribute('aria-label', button.title);
        });
    }

    function installSupportFrontControl() {
        if (document.getElementById('force-support-front')) return;
        const anchor = document.querySelector('.optimization-mode-panel');
        if (!anchor) return;

        const panel = document.createElement('div');
        panel.className = 'support-position-mode-panel';
        panel.innerHTML = `
            <label>
                <input type="checkbox" id="force-support-front">
                助战必须前排
            </label>
            <small>关闭时，求解器会比较助战前排与后排两种编成并选择总羁绊更高的方案；开启后，助战固定占用1个前排位置，自家前排最多2骑。助战前排时，自家前排站位乘区为×1.24、后排为×1.04。若已锁定3名自家前排，则无法开启。</small>
        `;
        anchor.insertAdjacentElement('afterend', panel);

        const checkbox = panel.querySelector('#force-support-front');
        checkbox.checked = localStorage.getItem(FORCE_SUPPORT_FRONT_KEY) === 'true';
        checkbox.addEventListener('change', () => {
            if (selectionStateReady && checkbox.checked && frontLockedServants.size > 2) {
                checkbox.checked = false;
                localStorage.setItem(FORCE_SUPPORT_FRONT_KEY, 'false');
                showPositionMessage('当前已锁定3名自家前排，助战没有可用的前排位置。请先解除至少1名前排锁定。', 'error');
                return;
            }
            localStorage.setItem(FORCE_SUPPORT_FRONT_KEY, checkbox.checked ? 'true' : 'false');
            decorateIncludeServantLocks();
        });
    }

    window.fetch = function(input, init = {}) {
        const url = typeof input === 'string' ? input : (input?.url || '');
        if (url.includes('/api/calculate') && init?.body) {
            selectionStateReady = true;
            pruneFrontLockedServants();
            if (!validateSupportFrontConflict(true)) {
                return Promise.reject(new Error('助战前排与自家前排锁定冲突'));
            }
            const mode = currentSupportPositionMode();
            if (init.body instanceof FormData || init.body instanceof URLSearchParams) {
                init.body.set('supportpositionmode', mode);
                init.body.delete('frontlockedsvt');
                Array.from(frontLockedServants)
                    .sort((a, b) => a - b)
                    .forEach(id => init.body.append('frontlockedsvt', String(id)));
            }
        }
        return nativeFetch(input, init);
    };

    function decoratePositionResults(teams) {
        ensureStyles();
        const teamDivs = Array.from(document.querySelectorAll('#results-list .team-result'));

        teamDivs.forEach((teamDiv, teamIndex) => {
            const team = teams?.[teamIndex];
            if (!team?.PositionOptimized) return;

            const forcedFront = currentSupportPositionMode() === 'front' && team.SupportPosition === 'front';
            const summary = teamDiv.querySelector('hgroup p');
            if (summary) {
                const supportText = team.SupportPosition === 'front'
                    ? `助战前排${forcedFront ? '（已锁定）' : ''} · 自家全员站位+4%`
                    : team.SupportPosition === 'back'
                        ? '助战后排'
                        : '无助战';
                const gain = Number(team.PositionBondGain || 0);
                const small = document.createElement('small');
                small.className = 'position-summary';
                small.textContent = ` | 站位：${supportText} | 站位增益 +${Math.round(gain)}`;
                summary.appendChild(small);
            }

            const servantGrid = teamDiv.querySelector('.team-grid');
            const servantWrappers = servantGrid ? Array.from(servantGrid.children) : [];
            servantWrappers.forEach((wrapper, index) => {
                const bondInfo = Array.isArray(team.ServantBondBonuses) ? team.ServantBondBonuses[index] : null;
                if (!bondInfo?.position) return;
                const avatar = wrapper.querySelector('.avatar-container');
                if (!avatar) return;

                const badge = document.createElement('span');
                badge.className = 'position-badge';
                const label = bondInfo.position === 'front'
                    ? (bondInfo.frontLocked ? '前排🔒' : '前排')
                    : '后排';
                badge.textContent = `${label} ${multiplierText(bondInfo.positionBonusPercent)}`;
                avatar.appendChild(badge);

                const contribution = avatar.querySelector('.servant-bond-bonus');
                if (contribution && !bondInfo.bondCapped) {
                    contribution.textContent = `${label}${multiplierText(bondInfo.positionBonusPercent)} · ${contribution.textContent}`;
                }
            });

            if (team.SupportPosition && team.SupportPosition !== 'none') {
                const supportTitle = Array.from(teamDiv.querySelectorAll('.team-section-title'))
                    .find(el => el.textContent.includes('建议助战'));
                if (supportTitle) {
                    supportTitle.textContent += team.SupportPosition === 'front'
                        ? ` · 助战前排${forcedFront ? '（已锁定）' : ''}（自家全员 +4% 站位乘区）`
                        : ' · 助战后排';
                }
            }
        });
    }

    ensureStyles();
    installSupportFrontControl();

    if (originalRenderSelectionList) {
        renderSelectionList = function(type, list) {
            const result = originalRenderSelectionList(type, list);
            if (type === 'svt' && list === 'include') {
                selectionStateReady = true;
                decorateIncludeServantLocks();
                validateSupportFrontConflict(false);
            }
            return result;
        };
    }

    renderResults = function(teams, backendDuration, networkDuration) {
        originalRenderResults(teams, backendDuration, networkDuration);
        try {
            decoratePositionResults(teams);
        } catch (error) {
            console.warn('Position optimizer UI decoration failed:', error);
        }
    };
})();
