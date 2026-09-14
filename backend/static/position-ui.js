(() => {
    if (typeof renderResults !== 'function') return;

    const FORCE_SUPPORT_FRONT_KEY = 'fgo-calc-force-support-front-v1';
    const originalRenderResults = renderResults;
    const nativeFetch = window.fetch.bind(window);

    function multiplierText(percent) {
        const value = 1 + Number(percent || 0) / 100;
        return `×${value.toFixed(2)}`;
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
        `;
        document.head.appendChild(style);
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
                助战必须前排（打手 / 换人服场景）
            </label>
            <small>关闭时自动比较助战前排/后排的总羁绊；开启后固定助战占用一个前排位，自家前排2骑按×1.24、后排3骑按×1.04计算。</small>
        `;
        anchor.insertAdjacentElement('afterend', panel);

        const checkbox = panel.querySelector('#force-support-front');
        checkbox.checked = localStorage.getItem(FORCE_SUPPORT_FRONT_KEY) === 'true';
        checkbox.addEventListener('change', () => {
            localStorage.setItem(FORCE_SUPPORT_FRONT_KEY, checkbox.checked ? 'true' : 'false');
        });
    }

    function currentSupportPositionMode() {
        return document.getElementById('force-support-front')?.checked ? 'front' : 'auto';
    }

    window.fetch = function(input, init = {}) {
        const url = typeof input === 'string' ? input : (input?.url || '');
        if (url.includes('/api/calculate') && init?.body) {
            const mode = currentSupportPositionMode();
            if (init.body instanceof FormData || init.body instanceof URLSearchParams) {
                init.body.set('supportpositionmode', mode);
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
                const label = bondInfo.position === 'front' ? '前排' : '后排';
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

    renderResults = function(teams, backendDuration, networkDuration) {
        originalRenderResults(teams, backendDuration, networkDuration);
        try {
            decoratePositionResults(teams);
        } catch (error) {
            console.warn('Position optimizer UI decoration failed:', error);
        }
    };
})();
