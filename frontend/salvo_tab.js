const API_BASE = window.API_BASE || 'http://localhost:8081/api/v1';

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

function initBarrage() { }

window.SalvoTab = {
    init: function() {
        initBarrage();
    },
    initBarrageCanvas: initBarrageCanvas,
    calculateBarrage: calculateBarrage
};
