const API_BASE = 'http://localhost:8081/api/v1';

const PowerPanel = (function() {
    async function fetchSensorData() {
        try {
            const res = await fetch(`${API_BASE}/sensor/chuangnu-001?limit=1`);
            if (!res.ok) throw new Error('API error');
            const data = await res.json();
            if (data.data && data.data.length > 0) {
                updateSensorDisplay(data.data[0]);
                document.getElementById('sensor-status').classList.remove('offline');
            }
        } catch (e) {
            document.getElementById('sensor-status').classList.add('offline');
        }
    }

    function updateSensorDisplay(data) {
        const tensionEl = document.getElementById('val-tension');
        const defEl = document.getElementById('val-deformation');
        const velEl = document.getElementById('val-velocity');
        const penEl = document.getElementById('val-penetration');

        tensionEl.textContent = data.bowstring_tension?.toFixed(0) || '0';
        defEl.textContent = data.arm_deformation?.toFixed(2) || '0';
        velEl.textContent = data.arrow_initial_velocity?.toFixed(1) || '0';
        penEl.textContent = (data.penetration_depth ? data.penetration_depth * 1000 : 0).toFixed(2);

        if (data.arm_deformation > 15) {
            defEl.classList.add('danger');
            defEl.classList.remove('warning');
        } else if (data.arm_deformation > 12) {
            defEl.classList.add('warning');
            defEl.classList.remove('danger');
        } else {
            defEl.classList.remove('warning', 'danger');
        }
    }

    async function fetchAlerts() {
        try {
            const res = await fetch(`${API_BASE}/alerts/unacknowledged?limit=20`);
            if (!res.ok) return;
            const data = await res.json();
            renderAlerts(data.data || []);
        } catch (e) {}
    }

    function renderAlerts(alerts) {
        const container = document.getElementById('alerts-container');
        if (!alerts.length) {
            container.innerHTML = '<div style="color: #6b7280; font-size: 12px; text-align: center; padding: 20px;">暂无告警</div>';
            return;
        }
        container.innerHTML = alerts.map(a => `
            <div class="alert-item ${a.alert_level}">
                <span class="alert-icon">${a.alert_level === 'critical' ? '🚨' : '⚠️'}</span>
                <span>${a.message}</span>
            </div>
        `).join('');
    }

    async function runSimulation() {
        const v0 = parseFloat(document.getElementById('param-velocity').value) || 120;
        const angle = parseFloat(document.getElementById('param-angle').value) || 45;
        const armor = document.getElementById('param-armor').value;
        const arrow = document.getElementById('param-arrow').value;
        const spin = parseFloat(document.getElementById('param-spin')?.value) || 25;

        try {
            const res = await fetch(`${API_BASE}/simulate?armor=${armor}&arrow=${arrow}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    initial_velocity: v0,
                    launch_angle: angle,
                    azimuth_angle: 0,
                    arrow_mass: 0.2,
                    arrow_diameter: 0.012,
                    arrow_length: 1.0,
                    drag_coefficient: 0.4,
                    air_density: 1.225,
                    spin_rate: spin
                })
            });
            if (!res.ok) throw new Error('sim failed');
            const result = await res.json();
            updateArmorCompare(v0, 0.012, 1.0, spin, arrow);
            return result;
        } catch (e) {
            updateArmorCompareLocal(v0, arrow);
            return null;
        }
    }

    async function updateArmorCompare(v0, diameter, length, spin, arrowType) {
        try {
            const res = await fetch(`${API_BASE}/penetrate/compare`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    impact_velocity: v0,
                    arrow_mass: 0.2,
                    arrow_diameter: diameter,
                    arrow_length: length,
                    spin_rate: spin,
                    arrow_head_type: arrowType
                })
            });
            if (!res.ok) throw new Error();
            const data = await res.json();
            for (const [name, r] of Object.entries(data.results || {})) {
                const key = name === '皮甲' ? 'leather' : name === '锁子甲' ? 'chainmail' : 'plate';
                const el = document.getElementById('armor-' + key);
                if (el) {
                    el.textContent = (r.penetration_depth * 1000).toFixed(1) + 'mm';
                    el.className = 'armor-result ' + (r.success ? 'success' : 'fail');
                }
            }
        } catch (e) {
            updateArmorCompareLocal(v0, arrowType);
        }
    }

    function updateArmorCompareLocal(v0, arrowType) {
        const ke = 0.5 * 0.2 * v0 * v0;
        const penetrations = {
            leather: Math.min(ke / 50, 15),
            chainmail: Math.min(ke / 150, 8),
            plate: Math.min(ke / 300, 4)
        };
        for (const [key, val] of Object.entries(penetrations)) {
            const el = document.getElementById('armor-' + key);
            const thickness = key === 'leather' ? 8 : key === 'chainmail' ? 6 : 2.5;
            el.textContent = val.toFixed(1) + 'mm';
            el.className = 'armor-result ' + (val >= thickness ? 'success' : 'fail');
        }
    }

    async function checkHealth() {
        try {
            const res = await fetch(`${API_BASE}/health`);
            if (res.ok) {
                document.getElementById('db-status').classList.remove('offline');
                return;
            }
        } catch (e) {}
        document.getElementById('db-status').classList.add('offline');
        document.getElementById('mqtt-status').classList.add('offline');
    }

    function drawTrajectory2D(points) {
        const canvas = document.getElementById('trajectory-canvas');
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        const rect = canvas.parentElement.getBoundingClientRect();
        canvas.width = rect.width;
        canvas.height = rect.height - 30;

        ctx.fillStyle = '#0a0a1a';
        ctx.fillRect(0, 0, canvas.width, canvas.height);

        const padding = 40;
        const maxX = Math.max(...points.map(p => p.x), 10);
        const maxY = Math.max(...points.map(p => p.y), 5);
        const scaleX = (canvas.width - padding * 2) / maxX;
        const scaleY = (canvas.height - padding * 2) / maxY;
        const scale = Math.min(scaleX, scaleY);

        ctx.strokeStyle = '#333';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 5; i++) {
            const x = padding + (maxX * i / 5) * scale;
            ctx.beginPath(); ctx.moveTo(x, padding); ctx.lineTo(x, canvas.height - padding); ctx.stroke();
            const y = canvas.height - padding - (maxY * i / 5) * scale;
            ctx.beginPath(); ctx.moveTo(padding, y); ctx.lineTo(canvas.width - padding, y); ctx.stroke();
        }

        ctx.fillStyle = '#9ca3af';
        ctx.font = '11px sans-serif';
        for (let i = 0; i <= 5; i++) {
            ctx.fillText((maxX * i / 5).toFixed(0) + 'm', padding + (maxX * i / 5) * scale - 10, canvas.height - 15);
            ctx.fillText((maxY * i / 5).toFixed(0) + 'm', 5, canvas.height - padding - (maxY * i / 5) * scale + 4);
        }

        const grad = ctx.createLinearGradient(0, 0, canvas.width, 0);
        grad.addColorStop(0, '#ffd700');
        grad.addColorStop(1, '#ff6b35');
        ctx.strokeStyle = grad;
        ctx.lineWidth = 2.5;
        ctx.beginPath();
        points.forEach((p, i) => {
            const px = padding + p.x * scale;
            const py = canvas.height - padding - p.y * scale;
            if (i === 0) ctx.moveTo(px, py);
            else ctx.lineTo(px, py);
        });
        ctx.stroke();

        const last = points[points.length - 1];
        ctx.fillStyle = '#ff6b35';
        ctx.beginPath();
        ctx.arc(padding + last.x * scale, canvas.height - padding - last.y * scale, 5, 0, Math.PI * 2);
        ctx.fill();

        ctx.fillStyle = '#ffd700';
        ctx.beginPath();
        ctx.arc(padding, canvas.height - padding - points[0].y * scale, 4, 0, Math.PI * 2);
        ctx.fill();

        const maxH = Math.max(...points.map(p => p.y));
        const maxHIdx = points.findIndex(p => p.y === maxH);
        const maxHPt = points[maxHIdx];
        ctx.fillStyle = '#9ca3af';
        ctx.font = '12px sans-serif';
        ctx.fillText(`最高点: ${maxH.toFixed(1)}m`, padding + maxHPt.x * scale, canvas.height - padding - maxHPt.y * scale - 10);
        ctx.fillText(`射程: ${last.x.toFixed(1)}m`, padding + last.x * scale - 40, canvas.height - 10);
    }

    function bindEvents() {
        document.getElementById('btn-shoot').addEventListener('click', () => {
            const v0 = parseFloat(document.getElementById('param-velocity').value) || 120;
            const angle = parseFloat(document.getElementById('param-angle').value) || 45;
            runSimulation();
            BedCrossbow3D.animateShot(v0, angle);
        });

        document.getElementById('btn-reset').addEventListener('click', () => BedCrossbow3D.resetView());
        document.getElementById('btn-animate').addEventListener('click', () => BedCrossbow3D.animateDrawString());

        const updateCompare = () => {
            const v0 = parseFloat(document.getElementById('param-velocity').value) || 120;
            const arrow = document.getElementById('param-arrow').value;
            updateArmorCompareLocal(v0, arrow);
        };
        document.getElementById('param-velocity').addEventListener('change', updateCompare);
        document.getElementById('param-arrow').addEventListener('change', updateCompare);
        if (document.getElementById('param-spin')) {
            document.getElementById('param-spin').addEventListener('change', updateCompare);
        }
    }

    function init() {
        bindEvents();
        checkHealth();
        fetchSensorData();
        fetchAlerts();

        setTimeout(() => {
            BedCrossbow3D.updateGPUStringDraw(0);
            const v0 = 120;
            const result = BedCrossbow3D.simulateTrajectoryLocally(v0, 45);
            BedCrossbow3D.showTrajectory3D(result.points);
            updateArmorCompareLocal(v0, 'bodkin');
        }, 500);

        setInterval(() => {
            checkHealth();
            fetchSensorData();
            fetchAlerts();
        }, 5000);
    }

    return {
        init: init,
        fetchSensorData: fetchSensorData,
        fetchAlerts: fetchAlerts,
        runSimulation: runSimulation,
        updateArmorCompare: updateArmorCompare,
        updateArmorCompareLocal: updateArmorCompareLocal,
        checkHealth: checkHealth,
        drawTrajectory2D: drawTrajectory2D
    };
})();
