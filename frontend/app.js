const API_BASE = 'http://localhost:8081/api/v1';

window.addEventListener('DOMContentLoaded', () => {
    BedCrossbow3D.init();
    PowerPanel.init();
    initTabs();
    initCompare();
    initEraCompare();
    initBarrage();
    initAimGame();
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
                    loadCompareData();
                } else if (target === 'tab-era') {
                    loadEraCompareData();
                } else if (target === 'tab-barrage') {
                    setTimeout(() => initBarrageCanvas(), 50);
                } else if (target === 'tab-aim') {
                    setTimeout(() => initAimCanvas(), 50);
                    loadTargetList();
                }
            }
        });
    });
}

function loadCompareData() {
    const grid = document.getElementById('compare-grid');
    if (!grid) return;
    grid.innerHTML = '<div style="color:#9ca3af;grid-column:1/-1;text-align:center;padding:40px;">加载中...</div>';
    fetch(API_BASE + '/compare/crossbows', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ launch_angle: 45, arrow_head_type: 'bodkin' })
    }).then(r => r.json()).then(data => {
        if (!data.crossbows) return;
        renderCompareCards(grid, data.crossbows, false);
    }).catch(err => {
        grid.innerHTML = '<div style="color:#ef4444;grid-column:1/-1;text-align:center;padding:40px;">加载失败，使用本地降级数据</div>';
        renderCompareCards(grid, getLocalCrossbowData(), false);
    });
}

function loadEraCompareData() {
    const grid = document.getElementById('era-compare-grid');
    if (!grid) return;
    grid.innerHTML = '<div style="color:#9ca3af;grid-column:1/-1;text-align:center;padding:40px;">加载中...</div>';
    fetch(API_BASE + '/compare/era', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ compare_range_m: 1000, arrow_head_type: 'bodkin' })
    }).then(r => r.json()).then(data => {
        if (!data.weapons) return;
        renderCompareCards(grid, data.weapons, true);
    }).catch(err => {
        grid.innerHTML = '<div style="color:#ef4444;grid-column:1/-1;text-align:center;padding:40px;">加载失败，使用本地降级数据</div>';
        renderCompareCards(grid, getLocalEraData(), true);
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

function getLocalCrossbowData() {
    return [
        { CrossbowName: '臂张弩', Era: '春秋战国 - 汉代', Description: '单人手臂拉力上弦，轻便灵活', InitialVelocity: 65, Range: 180, KineticEnergy: 74, ArrowMass: 0.035, ShotsPerMinute: 7.5, CrewSize: 1, DrawForce: 350, PowerIndex: 5.2, Penetrations: { leather: 12.5, lamellar: 4.2, plate: 0.8 }, PenetrationSuccess: { leather: true, lamellar: false, plate: false } },
        { CrossbowName: '蹶张弩', Era: '战国 - 唐宋', Description: '双脚蹬踏+腰部发力上弦', InitialVelocity: 95, Range: 350, KineticEnergy: 383, ArrowMass: 0.085, ShotsPerMinute: 3.0, CrewSize: 1, DrawForce: 900, PowerIndex: 28.5, Penetrations: { leather: 35.2, lamellar: 14.8, plate: 3.2 }, PenetrationSuccess: { leather: true, lamellar: true, plate: false } },
        { CrossbowName: '单弓床弩', Era: '汉代 - 宋代', Description: '安装在床架上的单弓弩', InitialVelocity: 110, Range: 550, KineticEnergy: 907, ArrowMass: 0.15, ShotsPerMinute: 1.3, CrewSize: 3, DrawForce: 2500, PowerIndex: 55.3, Penetrations: { leather: 65.0, lamellar: 30.5, plate: 7.8 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { CrossbowName: '三弓床弩', Era: '宋代', Description: '宋代重型床弩，三弓复合结构', InitialVelocity: 135, Range: 800, KineticEnergy: 1822, ArrowMass: 0.2, ShotsPerMinute: 0.67, CrewSize: 7, DrawForce: 5500, PowerIndex: 87.2, Penetrations: { leather: 120.5, lamellar: 58.2, plate: 16.5 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { CrossbowName: '七弓床弩', Era: '宋代', Description: '超重型床弩，传说中的八牛弩', InitialVelocity: 165, Range: 1500, KineticEnergy: 6806, ArrowMass: 0.5, ShotsPerMinute: 0.2, CrewSize: 20, DrawForce: 12000, PowerIndex: 100, Penetrations: { leather: 280.0, lamellar: 145.0, plate: 48.0 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
    ];
}

function getLocalEraData() {
    return [
        { WeaponName: '臂张弩', Era: '春秋战国 - 汉代', is_modern: false, Description: '单人手臂拉力上弦', MuzzleVelocity: 65, MaxRange: 180, ProjectileMass: 0.035, KineticEnergy: 74, ImpactKE: 18, ShotsPerMinute: 7.5, CrewSize: 1, PowerRatio: 0.02, Penetrations: { leather: 3.2, lamellar: 0.8 }, PenetrationSuccess: { leather: true, lamellar: false } },
        { WeaponName: '三弓床弩', Era: '宋代', is_modern: false, Description: '宋代重型床弩，三弓复合结构', MuzzleVelocity: 135, MaxRange: 800, ProjectileMass: 0.2, KineticEnergy: 1822, ImpactKE: 850, ShotsPerMinute: 0.67, CrewSize: 7, PowerRatio: 1.0, Penetrations: { leather: 45.0, lamellar: 18.5, plate: 4.2 }, PenetrationSuccess: { leather: true, lamellar: true, plate: false } },
        { WeaponName: '七弓床弩', Era: '宋代', is_modern: false, Description: '超重型床弩', MuzzleVelocity: 165, MaxRange: 1500, ProjectileMass: 0.5, KineticEnergy: 6806, ImpactKE: 3200, ShotsPerMinute: 0.2, CrewSize: 20, PowerRatio: 3.76, Penetrations: { leather: 120.0, lamellar: 55.0, plate: 15.0 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { WeaponName: '巴雷特 M82A1', Era: '现代 (1982-今)', is_modern: true, Description: '美军 .50 BMG 反器材步枪', MuzzleVelocity: 853, MaxRange: 1800, ProjectileMass: 0.042, KineticEnergy: 15275, ImpactKE: 12800, ShotsPerMinute: 20, CrewSize: 1, PowerRatio: 15.1, Penetrations: { leather: 800.0, lamellar: 450.0, plate: 120.0 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { WeaponName: '精密国际 AW50', Era: '现代 (2000-今)', is_modern: true, Description: '英军 .50 BMG 反器材步枪', MuzzleVelocity: 870, MaxRange: 2000, ProjectileMass: 0.042, KineticEnergy: 15895, ImpactKE: 13500, ShotsPerMinute: 17, CrewSize: 1, PowerRatio: 15.9, Penetrations: { leather: 830.0, lamellar: 470.0, plate: 128.0 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { WeaponName: 'NTW-20 20mm', Era: '现代 (1998-今)', is_modern: true, Description: '南非 20mm 超大口径反器材步枪', MuzzleVelocity: 720, MaxRange: 1600, ProjectileMass: 0.125, KineticEnergy: 32400, ImpactKE: 27500, ShotsPerMinute: 12, CrewSize: 2, PowerRatio: 32.4, Penetrations: { leather: 1500.0, lamellar: 850.0, plate: 280.0 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
    ];
}

function initCompare() { }

function initEraCompare() { }

let barrageCanvas, barrageCtx;
function initBarrageCanvas() {
    barrageCanvas = document.getElementById('barrage-canvas');
    if (!barrageCanvas) return;
    const container = barrageCanvas.parentElement;
    barrageCanvas.width = container.clientWidth;
    barrageCanvas.height = container.clientHeight;
    barrageCtx = barrageCanvas.getContext('2d');
    calculateBarrage();
    document.getElementById('btn-barrage-calc').addEventListener('click', calculateBarrage);
}

function calculateBarrage() {
    const count = parseInt(document.getElementById('barrage-count').value);
    const shots = parseInt(document.getElementById('barrage-shots').value);
    const spread = parseFloat(document.getElementById('barrage-spread').value);
    const radius = parseFloat(document.getElementById('barrage-radius').value);

    fetch(API_BASE + '/barrage/optimize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            crossbows: generateDefaultCrossbows(count),
            target: { x: 0, y: 500, radius: radius },
            max_shots_per_crossbow: shots,
            spread_angle: spread
        })
    }).then(r => r.json()).then(data => {
        renderBarrage(data);
        updateBarrageStats(data);
    }).catch(err => {
        const mock = generateLocalBarrage(count, shots, spread, radius);
        renderBarrage(mock);
        updateBarrageStats(mock);
    });
}

function generateDefaultCrossbows(count) {
    const crossbows = [];
    const types = ['bed_crossbow_triple', 'bed_crossbow_single'];
    const names = ['三弓床弩', '单弓床弩'];
    for (let i = 0; i < count; i++) {
        const angle = (i / count) * Math.PI * 0.6 - Math.PI * 0.3;
        const r = 30 + (i % 2) * 15;
        const typeIdx = i < Math.ceil(count / 2) ? 0 : 1;
        crossbows.push({
            id: 'cb-' + (i + 1),
            type: types[typeIdx],
            name: names[typeIdx] + '#' + (i + 1),
            x: Math.sin(angle) * r,
            y: -50 - (i % 3) * 10,
            heading: 0,
            elevation: 35
        });
    }
    return crossbows;
}

function generateLocalBarrage(count, shots, spread, radius) {
    const crossbows = generateDefaultCrossbows(count);
    const shotList = [];
    let minT = 1e9, maxT = 0;
    let totalKE = 0;
    let hits = 0;
    const targetX = 0, targetY = 500;

    crossbows.forEach(cb => {
        for (let i = 0; i < shots; i++) {
            const angleSpread = (Math.random() - 0.5) * spread * (Math.PI / 180);
            const distSpread = 1 + (Math.random() - 0.5) * 0.1;
            const baseAngle = Math.atan2(targetY - cb.y, targetX - cb.x);
            const shotAngle = baseAngle + angleSpread;
            const dist = (Math.sqrt((targetX - cb.x) ** 2 + (targetY - cb.y) ** 2)) * distSpread;
            const ix = cb.x + Math.cos(shotAngle) * dist;
            const iy = cb.y + Math.sin(shotAngle) * dist;
            const t = dist / 120 + Math.random() * 0.5;
            if (t < minT) minT = t;
            if (t > maxT) maxT = t;
            const ke = (cb.type === 'bed_crossbow_triple' ? 1800 : 900) * (0.9 + Math.random() * 0.2);
            totalKE += ke;
            const d = Math.sqrt((ix - targetX) ** 2 + (iy - targetY) ** 2);
            if (d <= radius) hits++;
            shotList.push({
                crossbow_id: cb.id,
                crossbow_name: cb.name,
                impact_x: ix,
                impact_y: iy,
                arrival_time: t,
                initial_velocity: cb.type === 'bed_crossbow_triple' ? 135 : 110
            });
        }
    });

    const allX = shotList.map(s => s.impact_x);
    const allY = shotList.map(s => s.impact_y);
    const minX = Math.min(...allX, ...crossbows.map(c => c.x)) - 20;
    const maxX = Math.max(...allX, ...crossbows.map(c => c.x)) + 20;
    const minY = Math.min(...allY, ...crossbows.map(c => c.y)) - 20;
    const maxY = Math.max(...allY, ...crossbows.map(c => c.y)) + 20;

    const cellSize = 5;
    const nx = Math.ceil((maxX - minX) / cellSize);
    const ny = Math.ceil((maxY - minY) / cellSize);
    const grid = Array.from({ length: nx }, () => Array(ny).fill(0));
    let covered = 0;
    shotList.forEach(s => {
        const ix = Math.floor((s.impact_x - minX) / cellSize);
        const iy = Math.floor((s.impact_y - minY) / cellSize);
        if (ix >= 0 && ix < nx && iy >= 0 && iy < ny) {
            if (grid[ix][iy] === 0) covered++;
            grid[ix][iy]++;
        }
    });

    return {
        shots: shotList,
        coverage: { min_x: minX, max_x: maxX, min_y: minY, max_y: maxY, cell_size: cellSize, grid },
        target_hit_rate: shotList.length > 0 ? hits / shotList.length : 0,
        area_covered_m2: covered * cellSize * cellSize,
        shots_in_target: hits,
        total_shots: shotList.length,
        time_window_seconds: maxT - minT,
        ke_concentrated_joules: totalKE,
        _crossbows: crossbows,
        _target: { x: targetX, y: targetY, radius }
    };
}

function renderBarrage(data) {
    if (!barrageCtx) return;
    const w = barrageCanvas.width;
    const h = barrageCanvas.height;
    barrageCtx.clearRect(0, 0, w, h);
    barrageCtx.fillStyle = 'rgba(15, 52, 96, 0.3)';
    barrageCtx.fillRect(0, 0, w, h);

    const shots = data.shots || [];
    const crossbows = data._crossbows || [];
    const target = data._target || { x: 0, y: 500, radius: 20 };
    const cov = data.coverage || {};

    const padding = 60;
    let minX = cov.min_x ?? -100, maxX = cov.max_x ?? 100;
    let minY = cov.min_y ?? -100, maxY = cov.max_y ?? 600;
    const scaleX = (w - padding * 2) / (maxX - minX);
    const scaleY = (h - padding * 2) / (maxY - minY);
    const scale = Math.min(scaleX, scaleY);
    const offX = padding + (w - padding * 2 - (maxX - minX) * scale) / 2;
    const offY = padding + (h - padding * 2 - (maxY - minY) * scale) / 2;

    function toScreen(x, y) {
        return [offX + (x - minX) * scale, h - (offY + (y - minY) * scale)];
    }

    barrageCtx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
    barrageCtx.lineWidth = 1;
    const gridStep = 50;
    for (let gx = Math.ceil(minX / gridStep) * gridStep; gx <= maxX; gx += gridStep) {
        const [sx] = toScreen(gx, 0);
        barrageCtx.beginPath();
        barrageCtx.moveTo(sx, 0);
        barrageCtx.lineTo(sx, h);
        barrageCtx.stroke();
    }
    for (let gy = Math.ceil(minY / gridStep) * gridStep; gy <= maxY; gy += gridStep) {
        const [, sy] = toScreen(0, gy);
        barrageCtx.beginPath();
        barrageCtx.moveTo(0, sy);
        barrageCtx.lineTo(w, sy);
        barrageCtx.stroke();
    }

    const [tx, ty] = toScreen(target.x, target.y);
    const rpx = target.radius * scale;
    barrageCtx.beginPath();
    barrageCtx.arc(tx, ty, rpx, 0, Math.PI * 2);
    barrageCtx.fillStyle = 'rgba(239, 68, 68, 0.15)';
    barrageCtx.fill();
    barrageCtx.strokeStyle = '#ef4444';
    barrageCtx.lineWidth = 2;
    barrageCtx.stroke();
    barrageCtx.fillStyle = '#ef4444';
    barrageCtx.font = '12px sans-serif';
    barrageCtx.fillText('目标区域', tx + rpx + 5, ty - rpx);

    crossbows.forEach(cb => {
        const [cx, cy] = toScreen(cb.x, cb.y);
        barrageCtx.beginPath();
        barrageCtx.arc(cx, cy, 8, 0, Math.PI * 2);
        barrageCtx.fillStyle = '#ffd700';
        barrageCtx.fill();
        barrageCtx.strokeStyle = '#b8860b';
        barrageCtx.lineWidth = 2;
        barrageCtx.stroke();
        barrageCtx.fillStyle = '#ffd700';
        barrageCtx.font = '10px sans-serif';
        barrageCtx.fillText(cb.name, cx + 12, cy + 4);
    });

    shots.forEach(s => {
        const [sx, sy] = toScreen(s.impact_x, s.impact_y);
        const dx = s.impact_x - target.x;
        const dy = s.impact_y - target.y;
        const dist = Math.sqrt(dx * dx + dy * dy);
        const inTarget = dist <= target.radius;
        barrageCtx.beginPath();
        barrageCtx.arc(sx, sy, 4, 0, Math.PI * 2);
        barrageCtx.fillStyle = inTarget ? 'rgba(34, 197, 94, 0.9)' : 'rgba(96, 165, 250, 0.7)';
        barrageCtx.fill();
    });

    barrageCtx.fillStyle = '#9ca3af';
    barrageCtx.font = '11px sans-serif';
    barrageCtx.fillText('X (m)', w - 50, h - 10);
    barrageCtx.save();
    barrageCtx.translate(15, 30);
    barrageCtx.rotate(-Math.PI / 2);
    barrageCtx.fillText('Y (m)', 0, 0);
    barrageCtx.restore();
}

function updateBarrageStats(data) {
    document.getElementById('stat-total-shots').textContent = data.total_shots || 0;
    document.getElementById('stat-hit-rate').textContent = ((data.target_hit_rate || 0) * 100).toFixed(1) + '%';
    document.getElementById('stat-hits').textContent = data.shots_in_target || 0;
    document.getElementById('stat-area').textContent = (data.area_covered_m2 || 0).toFixed(0) + ' m²';
    document.getElementById('stat-time').textContent = (data.time_window_seconds || 0).toFixed(2) + ' s';
    document.getElementById('stat-ke').textContent = (data.ke_concentrated_joules || 0).toFixed(0) + ' J';
}

let aimCanvas, aimCtx;
let currentTarget = null;
let aimScore = 0;
let aimTargets = [];

function initAimCanvas() {
    aimCanvas = document.getElementById('aim-canvas');
    if (!aimCanvas) return;
    const container = aimCanvas.parentElement;
    aimCanvas.width = container.clientWidth;
    aimCanvas.height = container.clientHeight;
    aimCtx = aimCanvas.getContext('2d');

    const elev = document.getElementById('aim-elev');
    const azi = document.getElementById('aim-azi');
    elev.addEventListener('input', () => {
        document.getElementById('aim-elev-val').textContent = elev.value + '°';
        drawAimScene();
    });
    azi.addEventListener('input', () => {
        document.getElementById('aim-azi-val').textContent = azi.value + '°';
        drawAimScene();
    });

    document.getElementById('btn-aim-shoot').addEventListener('click', doShoot);

    drawAimScene();
}

function loadTargetList() {
    fetch(API_BASE + '/aim/targets').then(r => r.json()).then(data => {
        aimTargets = data.targets || [];
        if (aimTargets.length === 0) aimTargets = getLocalTargets();
        renderTargetList();
    }).catch(() => {
        aimTargets = getLocalTargets();
        renderTargetList();
    });
}

function getLocalTargets() {
    return [
        { id: 'training', name: '训练靶 (50m)', distance: 50, height: 1.5, armor_type: 'leather', difficulty: 'easy', points: 10, icon: '🎯' },
        { id: 'soldier', name: '敌军步兵 (200m)', distance: 200, height: 1.7, armor_type: 'lamellar', difficulty: 'medium', points: 50, icon: '🛡️' },
        { id: 'rider', name: '敌方骑兵 (350m)', distance: 350, height: 2.0, armor_type: 'mail', difficulty: 'hard', points: 100, icon: '🐴' },
        { id: 'gate', name: '城门木盾 (500m)', distance: 500, height: 3.0, armor_type: 'leather', difficulty: 'hard', points: 150, icon: '🚪' },
        { id: 'tower', name: '瞭望塔守卫 (650m)', distance: 650, height: 8.0, armor_type: 'lamellar', difficulty: 'expert', points: 250, icon: '🏰' },
        { id: 'commander', name: '敌将 (800m)', distance: 800, height: 1.7, armor_type: 'plate', difficulty: 'legendary', points: 500, icon: '👑' },
    ];
}

function renderTargetList() {
    const list = document.getElementById('target-list');
    if (!list) return;
    list.innerHTML = '';
    aimTargets.forEach(t => {
        const item = document.createElement('div');
        item.className = 'target-item' + (currentTarget && currentTarget.id === t.id ? ' selected' : '');
        item.innerHTML = `
            <div><span class="icon">${t.icon}</span><span class="name">${t.name}</span><span class="difficulty-badge difficulty-${t.difficulty}">${t.difficulty}</span></div>
            <div class="info">距离 ${t.distance}m · ${t.height}m高 · 分值 ${t.points}</div>
        `;
        item.addEventListener('click', () => {
            currentTarget = t;
            renderTargetList();
            drawAimScene();
        });
        list.appendChild(item);
    });
    if (!currentTarget && aimTargets.length > 0) {
        currentTarget = aimTargets[0];
        renderTargetList();
    }
}

function drawAimScene() {
    if (!aimCtx) return;
    const w = aimCanvas.width;
    const h = aimCanvas.height;
    aimCtx.clearRect(0, 0, w, h);

    const grad = aimCtx.createLinearGradient(0, 0, 0, h);
    grad.addColorStop(0, '#1e3a5f');
    grad.addColorStop(0.5, '#2d5a87');
    grad.addColorStop(1, '#4a7c59');
    aimCtx.fillStyle = grad;
    aimCtx.fillRect(0, 0, w, h);

    aimCtx.fillStyle = 'rgba(139, 90, 43, 0.6)';
    aimCtx.beginPath();
    aimCtx.moveTo(0, h * 0.75);
    aimCtx.lineTo(w, h * 0.7);
    aimCtx.lineTo(w, h);
    aimCtx.lineTo(0, h);
    aimCtx.closePath();
    aimCtx.fill();

    aimCtx.fillStyle = 'rgba(100, 80, 60, 0.7)';
    aimCtx.beginPath();
    aimCtx.moveTo(0, h * 0.85);
    aimCtx.lineTo(w, h * 0.82);
    aimCtx.lineTo(w, h);
    aimCtx.lineTo(0, h);
    aimCtx.closePath();
    aimCtx.fill();

    for (let i = 0; i < 5; i++) {
        const mx = (i / 5) * w + w * 0.1;
        const my = h * 0.72 + Math.sin(i * 1.5) * 20;
        drawMountain(mx, my, 100 + i * 20, 80 + i * 10);
    }

    if (currentTarget) {
        const elev = parseFloat(document.getElementById('aim-elev').value);
        const azi = parseFloat(document.getElementById('aim-azi').value);
        const dist = currentTarget.distance;
        const cx = w / 2 + azi * 3;
        const baseY = h * 0.7;
        const angleRad = elev * Math.PI / 180;
        const maxHeight = (dist * Math.tan(angleRad)) / 20;
        const ty = baseY - maxHeight - currentTarget.height * 8;

        drawTarget(cx, ty, currentTarget);

        aimCtx.save();
        aimCtx.globalAlpha = 0.4;
        aimCtx.strokeStyle = '#ffd700';
        aimCtx.lineWidth = 2;
        aimCtx.setLineDash([5, 5]);
        aimCtx.beginPath();
        aimCtx.moveTo(w / 2, h * 0.9);
        aimCtx.quadraticCurveTo(w / 2 + azi * 1.5, h * 0.9 - maxHeight * 1.5, cx, ty);
        aimCtx.stroke();
        aimCtx.restore();
    }

    aimCtx.fillStyle = 'rgba(0, 0, 0, 0.5)';
    aimCtx.fillRect(10, 10, 220, 90);
    aimCtx.strokeStyle = '#ffd700';
    aimCtx.lineWidth = 2;
    aimCtx.strokeRect(10, 10, 220, 90);
    aimCtx.fillStyle = '#ffd700';
    aimCtx.font = 'bold 13px sans-serif';
    aimCtx.fillText('弩机状态', 20, 30);
    aimCtx.fillStyle = '#e4e4e7';
    aimCtx.font = '12px monospace';
    const cwSel = document.getElementById('aim-crossbow');
    const cwName = cwSel ? cwSel.options[cwSel.selectedIndex].text : '三弓床弩';
    aimCtx.fillText('弩机: ' + cwName, 20, 50);
    aimCtx.fillText('俯仰角: ' + document.getElementById('aim-elev').value + '°', 20, 68);
    aimCtx.fillText('方位角: ' + document.getElementById('aim-azi').value + '°', 20, 86);
    if (currentTarget) {
        aimCtx.fillText('目标距离: ' + currentTarget.distance + 'm', 130, 50);
    }
}

function drawMountain(x, y, w, h) {
    aimCtx.fillStyle = 'rgba(60, 80, 100, 0.5)';
    aimCtx.beginPath();
    aimCtx.moveTo(x - w / 2, y);
    aimCtx.lineTo(x, y - h);
    aimCtx.lineTo(x + w / 2, y);
    aimCtx.closePath();
    aimCtx.fill();
}

function drawTarget(x, y, t) {
    aimCtx.save();
    const size = Math.max(20, 60 - t.distance / 15);
    aimCtx.fillStyle = '#dc2626';
    aimCtx.fillRect(x - size / 2, y - size, size, size);
    aimCtx.strokeStyle = '#7f1d1d';
    aimCtx.lineWidth = 2;
    aimCtx.strokeRect(x - size / 2, y - size, size, size);
    aimCtx.fillStyle = '#fef3c7';
    aimCtx.font = size * 0.7 + 'px serif';
    aimCtx.textAlign = 'center';
    aimCtx.fillText(t.icon, x, y - size * 0.2);
    aimCtx.restore();
}

function doShoot() {
    if (!currentTarget) {
        alert('请先选择目标');
        return;
    }
    const btn = document.getElementById('btn-aim-shoot');
    btn.disabled = true;
    btn.textContent = '发射中...';

    const crossbow = document.getElementById('aim-crossbow').value;
    const arrow = document.getElementById('aim-arrow').value;
    const windSpeed = parseFloat(document.getElementById('aim-wind-speed').value);
    const windDir = parseFloat(document.getElementById('aim-wind-dir').value);

    const payload = {
        target: {
            distance: currentTarget.distance,
            height: currentTarget.height,
            armor_type: currentTarget.armor_type,
            name: currentTarget.name
        },
        crossbow_type: crossbow,
        arrow_type: arrow,
        wind_speed: windSpeed,
        wind_direction: windDir
    };

    fetch(API_BASE + '/aim/shoot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    }).then(r => r.json()).then(data => {
        processShootResult(data);
    }).catch(() => {
        const elev = parseFloat(document.getElementById('aim-elev').value);
        const azi = parseFloat(document.getElementById('aim-azi').value);
        const localData = simulateLocalShoot(currentTarget, elev, azi, crossbow);
        processShootResult(localData);
    }).finally(() => {
        btn.disabled = false;
        btn.textContent = '🏹 发射！';
    });
}

function simulateLocalShoot(target, elev, azi, crossbow) {
    const cwVels = { arm_stretched: 65, leg_stretched: 95, bed_crossbow_single: 110, bed_crossbow_triple: 135, bed_crossbow_seven: 165 };
    const v0 = cwVels[crossbow] || 135;
    const g = 9.81;
    const angleRad = elev * Math.PI / 180;
    const vx = v0 * Math.cos(angleRad);
    const vy = v0 * Math.sin(angleRad);
    const tFlight = target.distance / vx;
    const impactY = vy * tFlight - 0.5 * g * tFlight * tFlight;
    const impactV = Math.sqrt(vx * vx + Math.pow(vy - g * tFlight, 2));
    const cwMass = { arm_stretched: 0.035, leg_stretched: 0.085, bed_crossbow_single: 0.15, bed_crossbow_triple: 0.2, bed_crossbow_seven: 0.5 };
    const mass = cwMass[crossbow] || 0.2;
    const ke = 0.5 * mass * impactV * impactV;

    const diffRange = Math.abs(impactY - target.height);
    const aziError = Math.abs(azi);
    let hit = diffRange < 3 && aziError < 5;
    let penDepth = (ke / 1000) * (target.armor_type === 'leather' ? 50 : target.armor_type === 'lamellar' ? 20 : 8);
    let penSuccess = penDepth > (target.armor_type === 'plate' ? 15 : 5);

    let score = 0;
    let message = '未命中目标';
    if (hit && penSuccess) {
        score = target.points;
        message = '完美命中并穿透！';
        if (diffRange < 0.5) { score += Math.floor(target.points * 0.5); message = '直击靶心！完全穿透！'; }
    } else if (hit && !penSuccess) {
        score = Math.floor(target.points * 0.5);
        message = '命中目标，但未能穿透铠甲';
    } else if (diffRange < 10) {
        score = Math.floor(target.points * 0.2);
        message = '接近目标，但未命中';
    }

    return {
        success: true,
        hit,
        actual_range: target.distance,
        flight_time: tFlight,
        max_height: vy * vy / (2 * g),
        impact_velocity: impactV,
        kinetic_energy: ke,
        penetration_depth_mm: penDepth,
        penetration_success: penSuccess,
        message,
        score,
        trajectory: []
    };
}

function processShootResult(data) {
    if (!data) return;
    const resultDiv = document.getElementById('shoot-result');
    const cls = data.hit ? 'hit' : 'miss';
    resultDiv.innerHTML = `
        <div class="shoot-result ${cls}">
            <h3>${data.hit ? (data.penetration_success ? '🎯 命中穿透！' : '⚠️ 命中未穿透') : '❌ 未命中'}</h3>
            <div style="font-size:13px;margin:8px 0;">${data.message}</div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:6px;font-size:11px;">
                <div>射程: ${(data.actual_range || 0).toFixed(0)} m</div>
                <div>飞行: ${(data.flight_time || 0).toFixed(2)} s</div>
                <div>命中速度: ${(data.impact_velocity || 0).toFixed(1)} m/s</div>
                <div>动能: ${(data.kinetic_energy || 0).toFixed(0)} J</div>
                <div>穿深: ${(data.penetration_depth_mm || 0).toFixed(1)} mm</div>
                <div>穿透: ${data.penetration_success ? '✓' : '✗'}</div>
            </div>
            <div style="margin-top:10px;font-size:20px;font-weight:bold;color:#ffd700;">得分: +${data.score || 0}</div>
        </div>
    `;
    aimScore += data.score || 0;
    document.getElementById('score-value').textContent = aimScore;
}
