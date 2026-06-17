const API_BASE = 'http://localhost:8081/api/v1';
window.API_BASE = API_BASE;

window.addEventListener('DOMContentLoaded', () => {
    BedCrossbow3D.init();
    PowerPanel.init();
    initTabs();
    window.PowerTab.init();
    window.EraTab.init();
    window.SalvoTab.init();
    window.VRCrossbow.init();
});

function initTabs() {
    const tabBtns = document.querySelectorAll('.tab-btn');
    const tabContents = document.querySelectorAll('.tab-content');
    tabBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            const target = btn.dataset.tab;
            tabBtns.forEach(b => b.classList.remove('active'));
            tabContents.forEach(c => c.classList.remove('active'));
            btn.classList.add('active');
            const content = document.getElementById(target);
            if (content) {
                content.classList.add('active');
                if (target === 'tab-compare') {
                    window.PowerTab.loadCompareData();
                } else if (target === 'tab-era') {
                    window.EraTab.loadEraCompareData();
                } else if (target === 'tab-barrage') {
                    setTimeout(() => window.SalvoTab.initBarrageCanvas(), 50);
                } else if (target === 'tab-aim') {
                    setTimeout(() => window.VRCrossbow.initAimCanvas(), 50);
                    window.VRCrossbow.loadTargetList();
                }
            }
        });
    });
}

function renderCompareCards(grid, items, isEra) {
    grid.innerHTML = '';
    if (!items || items.length === 0) return;
    const maxKE = Math.max(...items.map(i => i.kinetic_energy || i.KineticEnergy || 0));
    items.forEach(item => {
        const name = item.crossbow_name || item.weapon_name || item.CrossbowName || item.WeaponName || '未知';
        const era = item.era || item.Era || '';
        const desc = item.description || item.Description || '';
        const isModern = item.is_modern !== undefined ? item.is_modern : (isEra && item.WeaponType && !item.WeaponType.includes('stretched') && !item.WeaponType.includes('bed_'));
        const velocity = item.initial_velocity ?? item.muzzle_velocity ?? item.InitialVelocity ?? item.MuzzleVelocity ?? 0;
        const range = item.range ?? item.max_range ?? item.Range ?? item.MaxRange ?? 0;
        const ke = item.kinetic_energy || item.KineticEnergy || 0;
        const flightTime = item.flight_time || item.FlightTime || 0;
        const impactVel = item.impact_velocity || item.ImpactVelocity || 0;
        const shotsPerMin = item.shots_per_minute || item.ShotsPerMinute || 0;
        const powerIndex = item.power_index || item.PowerIndex || 0;
        const powerRatio = item.power_ratio_to_bedcrossbow || item.PowerRatio || 0;
        const drawForce = item.draw_force || item.DrawForce || 0;
        const crewSize = item.crew_size || item.CrewSize || 0;
        const projectileMass = item.arrow_mass || item.projectile_mass || item.ArrowMass || item.ProjectileMass || 0;

        let armorHtml = '';
        const pens = item.penetration_mm || item.Penetrations || {};
        const penSuccess = item.penetration_success || item.PenetrationSuccess || {};
        const armorKeys = Object.keys(pens);
        if (armorKeys.length > 0) {
            armorHtml = '<div class="armor-mini-grid">';
            armorKeys.slice(0, 6).forEach(k => {
                const ok = penSuccess[k];
                const val = pens[k];
                armorHtml += `<div class="armor-mini ${ok ? 'ok' : 'no'}">${k}: ${val.toFixed(1)}mm ${ok ? '✓' : '✗'}</div>`;
            });
            armorHtml += '</div>';
        }

        const powerPct = maxKE > 0 ? Math.min(100, (ke / maxKE) * 100) : 0;

        const card = document.createElement('div');
        card.className = 'compare-card';
        card.innerHTML = `
            <h3>${name}</h3>
            <span class="era-tag ${isModern ? 'modern' : ''}">${era}</span>
            <div class="desc">${desc}</div>
            <div class="compare-stats">
                <div class="stat-item">
                    <div class="stat-label">初速</div>
                    <div class="stat-value highlight">${velocity.toFixed(1)} m/s</div>
                </div>
                <div class="stat-item">
                    <div class="stat-label">射程</div>
                    <div class="stat-value">${range.toFixed(0)} m</div>
                </div>
                <div class="stat-item">
                    <div class="stat-label">动能</div>
                    <div class="stat-value highlight">${ke.toFixed(0)} J</div>
                </div>
                <div class="stat-item">
                    <div class="stat-label">弹重</div>
                    <div class="stat-value">${(projectileMass * 1000).toFixed(0)} g</div>
                </div>
                <div class="stat-item">
                    <div class="stat-label">射速</div>
                    <div class="stat-value">${shotsPerMin.toFixed(1)} 发/分</div>
                </div>
                <div class="stat-item">
                    <div class="stat-label">操作人员</div>
                    <div class="stat-value">${crewSize} 人</div>
                </div>
                ${drawForce > 0 ? `
                <div class="stat-item">
                    <div class="stat-label">拉力</div>
                    <div class="stat-value">${drawForce.toFixed(0)} N</div>
                </div>` : ''}
                <div class="stat-item">
                    <div class="stat-label">${isEra ? '相对威力' : '威力指数'}</div>
                    <div class="stat-value highlight">${isEra ? powerRatio.toFixed(1) + 'x' : powerIndex.toFixed(1)}</div>
                </div>
            </div>
            ${armorHtml}
            <div class="power-bar">
                <div class="power-bar-fill ${isModern ? 'modern' : ''}" style="width:${powerPct}%"></div>
            </div>
            <div class="power-label">
                <span>动能占比</span>
                <span>${powerPct.toFixed(0)}%</span>
            </div>
        `;
        grid.appendChild(card);
    });
}

window.renderCompareCards = renderCompareCards;
