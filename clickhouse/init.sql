CREATE DATABASE IF NOT EXISTS ballistics;

USE ballistics;

CREATE TABLE IF NOT EXISTS sensor_data (
    id UUID DEFAULT generateUUIDv4(),
    device_id String,
    timestamp DateTime64(9) DEFAULT now64(),
    bowstring_tension Float64,
    arm_deformation Float64,
    arrow_initial_velocity Float64,
    arrow_spin_rate Float64 DEFAULT 25.0,
    penetration_depth Float64,
    temperature Float64 DEFAULT 0,
    humidity Float64 DEFAULT 0
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 1 YEAR;

CREATE TABLE IF NOT EXISTS simulation_results (
    id UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(9) DEFAULT now64(),
    device_id String,
    initial_velocity Float64,
    launch_angle Float64,
    flight_time Float64,
    max_height Float64,
    range Float64,
    impact_velocity Float64,
    kinetic_energy Float64,
    impact_spin_rate Float64 DEFAULT 25.0,
    impact_gyro_stab Float64 DEFAULT 1.0,
    trajectory String,
    armor_type String,
    penetration_depth Float64,
    penetration_success Bool
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 1 YEAR;

CREATE TABLE IF NOT EXISTS alerts (
    id UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(9) DEFAULT now64(),
    device_id String,
    alert_type String,
    alert_level String,
    message String,
    sensor_value Float64,
    threshold Float64,
    acknowledged Bool DEFAULT false
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 1 YEAR;

CREATE TABLE IF NOT EXISTS armor_performance (
    id UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(9) DEFAULT now64(),
    armor_type String,
    armor_thickness Float64,
    impact_velocity Float64,
    arrow_mass Float64,
    arrow_diameter Float64 DEFAULT 0.012,
    arrow_length Float64 DEFAULT 1.0,
    spin_rate Float64 DEFAULT 25.0,
    gyro_stability Float64 DEFAULT 1.0,
    yaw_angle Float64 DEFAULT 0.0,
    effective_area Float64 DEFAULT 0,
    arrow_head_type String,
    penetration_depth Float64,
    residual_velocity Float64,
    energy_absorbed Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (armor_type, timestamp)
TTL timestamp + INTERVAL 1 YEAR;

CREATE TABLE IF NOT EXISTS bow_release_energy (
    id UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(9) DEFAULT now64(),
    device_id String,
    initial_potential_energy Float64,
    arrow_ke Float64,
    arm_ke Float64,
    string_ke Float64,
    hysteresis_loss Float64,
    viscous_loss Float64,
    internal_loss Float64,
    nonlinear_loss Float64,
    total_dissipated Float64,
    efficiency Float64,
    release_time Float64,
    exit_velocity Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 1 YEAR;

CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, toStartOfHour(timestamp))
AS SELECT
    device_id,
    toStartOfHour(timestamp) as timestamp,
    count() as count,
    sum(bowstring_tension) as sum_tension,
    sum(arm_deformation) as sum_deformation,
    sum(arrow_initial_velocity) as sum_velocity,
    sum(arrow_spin_rate) as sum_spin,
    sum(penetration_depth) as sum_penetration,
    max(bowstring_tension) as max_tension,
    max(arm_deformation) as max_deformation,
    max(arrow_spin_rate) as max_spin
FROM sensor_data
GROUP BY device_id, toStartOfHour(timestamp);

-- =========================================================
-- 历史日聚合 (降采样)：从小时级 MV 自动聚合，
-- 供大屏/报表/考古使用（不需要 1h 粒度时，性能快 24x）
-- =========================================================
CREATE TABLE IF NOT EXISTS sensor_data_daily_ds (
    device_id String,
    timestamp Date,
    shots SimpleAggregateFunction(sum, UInt64),
    avg_tension SimpleAggregateFunction(avg, Float64),
    max_tension SimpleAggregateFunction(max, Float64),
    avg_deformation SimpleAggregateFunction(avg, Float64),
    max_deformation SimpleAggregateFunction(max, Float64),
    avg_velocity SimpleAggregateFunction(avg, Float64),
    max_velocity SimpleAggregateFunction(max, Float64),
    avg_spin SimpleAggregateFunction(avg, Float64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 3 YEAR;

CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_daily_ds_mv
TO sensor_data_daily_ds
AS SELECT
    device_id,
    toDate(timestamp)                                  AS timestamp,
    sumSimpleState(count)                              AS shots,
    avgSimpleState(sum_tension / count)                AS avg_tension,
    maxSimpleState(max_tension)                        AS max_tension,
    avgSimpleState(sum_deformation / count)            AS avg_deformation,
    maxSimpleState(max_deformation)                    AS max_deformation,
    avgSimpleState(sum_velocity / count)               AS avg_velocity,
    maxSimpleState(sum_velocity / count)               AS max_velocity,
    avgSimpleState(sum_spin / count)                   AS avg_spin
FROM sensor_data_stats_mv
GROUP BY device_id, toDate(timestamp);

-- =========================================================
-- simulation_results 历史日聚合 (射程/KE 趋势分析)
-- =========================================================
CREATE TABLE IF NOT EXISTS sim_daily_ds (
    device_id String,
    timestamp Date,
    shots SimpleAggregateFunction(sum, UInt64),
    avg_range SimpleAggregateFunction(avg, Float64),
    p95_range SimpleAggregateFunction(max, Float64),
    avg_impact_v SimpleAggregateFunction(avg, Float64),
    avg_ke SimpleAggregateFunction(avg, Float64),
    below_min_range SimpleAggregateFunction(sum, UInt64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 3 YEAR;

CREATE MATERIALIZED VIEW IF NOT EXISTS sim_daily_ds_mv
TO sim_daily_ds
AS SELECT
    device_id,
    toDate(timestamp)                       AS timestamp,
    sumSimpleState(1)                       AS shots,
    avgSimpleState(range)                   AS avg_range,
    maxSimpleState(range)                   AS p95_range,
    avgSimpleState(impact_velocity)         AS avg_impact_v,
    avgSimpleState(kinetic_energy)          AS avg_ke,
    sumSimpleState(if(range < 300, 1, 0))   AS below_min_range
FROM simulation_results
GROUP BY device_id, toDate(timestamp);

-- =========================================================
-- armor_performance 按铠甲类型聚合（穿甲概率统计）
-- =========================================================
CREATE TABLE IF NOT EXISTS armor_perf_daily_ds (
    armor_type String,
    arrow_head_type String,
    timestamp Date,
    tests SimpleAggregateFunction(sum, UInt64),
    avg_penetration_mm SimpleAggregateFunction(avg, Float64),
    max_penetration_mm SimpleAggregateFunction(max, Float64),
    success_rate SimpleAggregateFunction(avg, Float64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (armor_type, arrow_head_type, timestamp)
TTL timestamp + INTERVAL 3 YEAR;

CREATE MATERIALIZED VIEW IF NOT EXISTS armor_perf_daily_ds_mv
TO armor_perf_daily_ds
AS SELECT
    armor_type,
    arrow_head_type,
    toDate(timestamp)                                                                  AS timestamp,
    sumSimpleState(1)                                                                  AS tests,
    avgSimpleState(penetration_depth * 1000)                                           AS avg_penetration_mm,
    maxSimpleState(penetration_depth * 1000)                                           AS max_penetration_mm,
    avgSimpleState(if(penetration_depth > armor_thickness AND penetration_depth > 0.0005, 1.0, 0.0)) AS success_rate
FROM armor_performance
GROUP BY armor_type, arrow_head_type, toDate(timestamp);

