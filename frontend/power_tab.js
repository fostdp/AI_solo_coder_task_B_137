const API_BASE = window.API_BASE || 'http://localhost:8081/api/v1';
const renderCompareCards = window.renderCompareCards;

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

function getLocalCrossbowData() {
    return [
        { CrossbowName: '臂张弩', Era: '春秋战国 - 汉代', Description: '单人手臂拉力上弦，轻便灵活', InitialVelocity: 65, Range: 180, KineticEnergy: 74, ArrowMass: 0.035, ShotsPerMinute: 7.5, CrewSize: 1, DrawForce: 350, PowerIndex: 5.2, Penetrations: { leather: 12.5, lamellar: 4.2, plate: 0.8 }, PenetrationSuccess: { leather: true, lamellar: false, plate: false } },
        { CrossbowName: '蹶张弩', Era: '战国 - 唐宋', Description: '双脚蹬踏+腰部发力上弦', InitialVelocity: 95, Range: 350, KineticEnergy: 383, ArrowMass: 0.085, ShotsPerMinute: 3.0, CrewSize: 1, DrawForce: 900, PowerIndex: 28.5, Penetrations: { leather: 35.2, lamellar: 14.8, plate: 3.2 }, PenetrationSuccess: { leather: true, lamellar: true, plate: false } },
        { CrossbowName: '单弓床弩', Era: '汉代 - 宋代', Description: '安装在床架上的单弓弩', InitialVelocity: 110, Range: 550, KineticEnergy: 907, ArrowMass: 0.15, ShotsPerMinute: 1.3, CrewSize: 3, DrawForce: 2500, PowerIndex: 55.3, Penetrations: { leather: 65.0, lamellar: 30.5, plate: 7.8 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { CrossbowName: '三弓床弩', Era: '宋代', Description: '宋代重型床弩，三弓复合结构', InitialVelocity: 135, Range: 800, KineticEnergy: 1822, ArrowMass: 0.2, ShotsPerMinute: 0.67, CrewSize: 7, DrawForce: 5500, PowerIndex: 87.2, Penetrations: { leather: 120.5, lamellar: 58.2, plate: 16.5 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
        { CrossbowName: '七弓床弩', Era: '宋代', Description: '超重型床弩，传说中的八牛弩', InitialVelocity: 165, Range: 1500, KineticEnergy: 6806, ArrowMass: 0.5, ShotsPerMinute: 0.2, CrewSize: 20, DrawForce: 12000, PowerIndex: 100, Penetrations: { leather: 280.0, lamellar: 145.0, plate: 48.0 }, PenetrationSuccess: { leather: true, lamellar: true, plate: true } },
    ];
}

function initCompare() { }

window.PowerTab = {
    init: function() {
        initCompare();
    },
    loadCompareData: loadCompareData
};
