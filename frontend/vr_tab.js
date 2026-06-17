const API_BASE = window.API_BASE || 'http://localhost:8081/api/v1';

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

    document.getElementById('btn-aim-shoot').addEventListener('click', shootArrow);

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

function shootArrow() {
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
        renderShootResult(data);
    }).catch(() => {
        const elev = parseFloat(document.getElementById('aim-elev').value);
        const azi = parseFloat(document.getElementById('aim-azi').value);
        const localData = simulateLocalShoot(currentTarget, elev, azi, crossbow);
        renderShootResult(localData);
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

function renderShootResult(data) {
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

function initAimGame() { }

window.VRCrossbow = {
    init: function() {
        initAimGame();
    },
    initAimCanvas: initAimCanvas,
    loadTargetList: loadTargetList,
    shootArrow: shootArrow,
    renderShootResult: renderShootResult
};
