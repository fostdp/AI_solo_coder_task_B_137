const API_BASE = window.API_BASE || 'http://localhost:8081/api/v1';
const renderCompareCards = window.renderCompareCards;

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

function initEraCompare() { }

window.EraTab = {
    init: function() {
        initEraCompare();
    },
    loadEraCompareData: loadEraCompareData
};
