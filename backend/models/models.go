package models

import "time"

type SensorData struct {
	DeviceID             string    `json:"device_id"`
	Timestamp            time.Time `json:"timestamp"`
	BowstringTension     float64   `json:"bowstring_tension"`
	ArmDeformation       float64   `json:"arm_deformation"`
	ArrowInitialVelocity float64   `json:"arrow_initial_velocity"`
	ArrowSpinRate        float64   `json:"arrow_spin_rate"`
	PenetrationDepth     float64   `json:"penetration_depth"`
	Temperature          float64   `json:"temperature"`
	Humidity             float64   `json:"humidity"`
}

type ValidatedSensorData struct {
	Data    *SensorData
	IsValid bool
	Errors  []string
}

type TrajectoryPoint struct {
	Time           float64 `json:"t"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Z              float64 `json:"z"`
	Vx             float64 `json:"vx"`
	Vy             float64 `json:"vy"`
	Vz             float64 `json:"vz"`
	Velocity       float64 `json:"v"`
	SpinRate       float64 `json:"spin_rate"`
	GyroStability  float64 `json:"gyro_stab"`
	AttitudeStable bool    `json:"stable"`
}

type SimulationParams struct {
	InitialVelocity float64 `json:"initial_velocity"`
	LaunchAngle     float64 `json:"launch_angle"`
	AzimuthAngle    float64 `json:"azimuth_angle"`
	ArrowMass       float64 `json:"arrow_mass"`
	ArrowDiameter   float64 `json:"arrow_diameter"`
	ArrowLength     float64 `json:"arrow_length"`
	DragCoefficient float64 `json:"drag_coefficient"`
	AirDensity      float64 `json:"air_density"`
	SpinRate        float64 `json:"spin_rate"`
}

type SimulationResult struct {
	DeviceID           string            `json:"device_id"`
	Timestamp          time.Time         `json:"timestamp"`
	InitialVelocity    float64           `json:"initial_velocity"`
	LaunchAngle        float64           `json:"launch_angle"`
	FlightTime         float64           `json:"flight_time"`
	MaxHeight          float64           `json:"max_height"`
	Range              float64           `json:"range"`
	ImpactVelocity     float64           `json:"impact_velocity"`
	KineticEnergy      float64           `json:"kinetic_energy"`
	ImpactSpinRate     float64           `json:"impact_spin"`
	ImpactGyroStab     float64           `json:"impact_gyro"`
	Trajectory         []TrajectoryPoint `json:"trajectory"`
	ArmorType          string            `json:"armor_type"`
	PenetrationDepth   float64           `json:"penetration_depth"`
	PenetrationSuccess bool              `json:"penetration_success"`
	RangeError         float64           `json:"range_error_m,omitempty"`
	HeightError        float64           `json:"height_error_m,omitempty"`
	LateralError       float64           `json:"lateral_error_m,omitempty"`
	DriftLateral       float64           `json:"drift_lateral_m,omitempty"`
}

type ArmorParams struct {
	Type          string  `json:"type"`
	Thickness     float64 `json:"thickness"`
	Density       float64 `json:"density"`
	YieldStrength float64 `json:"yield_strength"`
	Hardness      float64 `json:"hardness"`
	Name          string  `json:"name"`
}

type ArrowHeadParams struct {
	Type        string  `json:"type"`
	TipDiameter float64 `json:"tip_diameter"`
	TipArea     float64 `json:"tip_area"`
	TipMass     float64 `json:"tip_mass"`
	Hardness    float64 `json:"hardness"`
	Name        string  `json:"name"`
}

type PenetrationResult struct {
	ArmorType        string  `json:"armor_type"`
	ArmorThickness   float64 `json:"armor_thickness"`
	ImpactVelocity   float64 `json:"impact_velocity"`
	ArrowMass        float64 `json:"arrow_mass"`
	ArrowHeadType    string  `json:"arrow_head_type"`
	PenetrationDepth float64 `json:"penetration_depth"`
	ResidualVelocity float64 `json:"residual_velocity"`
	EnergyAbsorbed   float64 `json:"energy_absorbed"`
	Success          bool    `json:"success"`
	ImpactSpinRate   float64 `json:"impact_spin"`
	GyroStability    float64 `json:"gyro_stab"`
	YawAngle         float64 `json:"yaw_angle"`
	EffectiveArea    float64 `json:"effective_area"`
	StabilityFactor  float64 `json:"stab_factor"`
}

type Alert struct {
	DeviceID     string    `json:"device_id"`
	Timestamp    time.Time `json:"timestamp"`
	AlertType    string    `json:"alert_type"`
	AlertLevel   string    `json:"alert_level"`
	Message      string    `json:"message"`
	SensorValue  float64   `json:"sensor_value"`
	Threshold    float64   `json:"threshold"`
	Acknowledged bool      `json:"acknowledged"`
}

type ArmorPerformance struct {
	Timestamp        time.Time `json:"timestamp"`
	ArmorType        string    `json:"armor_type"`
	ArmorThickness   float64   `json:"armor_thickness"`
	ImpactVelocity   float64   `json:"impact_velocity"`
	ArrowMass        float64   `json:"arrow_mass"`
	ArrowDiameter    float64   `json:"arrow_diameter"`
	ArrowLength      float64   `json:"arrow_length"`
	SpinRate         float64   `json:"spin_rate"`
	GyroStability    float64   `json:"gyro_stability"`
	YawAngle         float64   `json:"yaw_angle"`
	EffectiveArea    float64   `json:"effective_area"`
	ArrowHeadType    string    `json:"arrow_head_type"`
	PenetrationDepth float64   `json:"penetration_depth"`
	ResidualVelocity float64   `json:"residual_velocity"`
	EnergyAbsorbed   float64   `json:"energy_absorbed"`
}

type BowReleaseEnergy struct {
	DeviceID             string    `json:"device_id"`
	Timestamp            time.Time `json:"timestamp"`
	InitialPotentialEnergy float64 `json:"initial_potential_energy"`
	ArrowKE              float64   `json:"arrow_ke"`
	ArmKE                float64   `json:"arm_ke"`
	StringKE             float64   `json:"string_ke"`
	HysteresisLoss       float64   `json:"hysteresis_loss"`
	ViscousLoss          float64   `json:"viscous_loss"`
	InternalLoss         float64   `json:"internal_loss"`
	NonlinearLoss        float64   `json:"nonlinear_loss"`
	TotalDissipated      float64   `json:"total_dissipated"`
	Efficiency           float64   `json:"efficiency"`
	ReleaseTime          float64   `json:"release_time"`
	ExitVelocity         float64   `json:"exit_velocity"`
}

type SimJob struct {
	Params   *SimulationParams
	DeviceID string
}

type PenJob struct {
	ImpactVelocity float64
	ArrowMass      float64
	ArrowDiameter  float64
	ArrowLength    float64
	SpinRate       float64
	ArmorType      string
	ArrowHeadType  string
	ArmorThickness float64
	DeviceID       string
}

type PipelineResult struct {
	DeviceID      string
	SensorData    *SensorData
	SimResult     *SimulationResult
	PenResult     *PenetrationResult
	ReleaseEnergy map[string]float64
}

type CrossbowComparisonItem struct {
	CrossbowType   string             `json:"crossbow_type"`
	CrossbowName   string             `json:"crossbow_name"`
	Description    string             `json:"description"`
	Era            string             `json:"era"`
	DrawForce      float64            `json:"draw_force"`
	DrawLength     float64            `json:"draw_length"`
	ArrowMass      float64            `json:"arrow_mass"`
	ArrowDiameter  float64            `json:"arrow_diameter"`
	ArrowLength    float64            `json:"arrow_length"`
	SpinRate       float64            `json:"spin_rate"`
	BowEfficiency  float64            `json:"bow_efficiency"`
	CrewSize       int                `json:"crew_size"`
	ReloadSeconds  float64            `json:"reload_seconds"`
	InitialVelocity float64           `json:"initial_velocity"`
	Range          float64            `json:"range"`
	FlightTime     float64            `json:"flight_time"`
	MaxHeight      float64            `json:"max_height"`
	ImpactVelocity float64            `json:"impact_velocity"`
	KineticEnergy  float64            `json:"kinetic_energy"`
	ImpactSpinRate float64            `json:"impact_spin"`
	ImpactGyroStab float64            `json:"impact_gyro"`
	Penetrations   map[string]float64 `json:"penetration_mm"`
	PenetrationSuccess map[string]bool `json:"penetration_success"`
	ShotsPerMinute float64            `json:"shots_per_minute"`
	KEPerMinute    float64            `json:"ke_per_minute"`
	PowerIndex     float64            `json:"power_index"`
}

type CrossbowComparisonResponse struct {
	Crossbows []CrossbowComparisonItem `json:"crossbows"`
	ArmorTypes []string                `json:"armor_types"`
}

type WeaponEraComparison struct {
	WeaponType      string  `json:"weapon_type"`
	WeaponName      string  `json:"weapon_name"`
	Description     string  `json:"description"`
	Era             string  `json:"era"`
	IsModern        bool    `json:"is_modern"`
	ProjectileMass  float64 `json:"projectile_mass"`
	ProjectileDia   float64 `json:"projectile_diameter"`
	ProjectileLen   float64 `json:"projectile_length"`
	MuzzleVelocity  float64 `json:"muzzle_velocity"`
	MaxRange        float64 `json:"max_range"`
	EffectiveRange  float64 `json:"effective_range"`
	SpinRate        float64 `json:"spin_rate"`
	KineticEnergy   float64 `json:"kinetic_energy"`
	ImpactVelocity  float64 `json:"impact_velocity_at_1000m"`
	ImpactKE        float64 `json:"impact_ke_at_1000m"`
	Penetrations    map[string]float64 `json:"penetration_mm"`
	PenetrationSuccess map[string]bool  `json:"penetration_success"`
	CrewSize        int     `json:"crew_size"`
	ReloadSeconds   float64 `json:"reload_seconds"`
	ShotsPerMinute  float64 `json:"shots_per_minute"`
	KEPerMinute     float64 `json:"ke_per_minute"`
	PowerRatio      float64 `json:"power_ratio_to_bedcrossbow"`
}

type EraComparisonResponse struct {
	Weapons []WeaponEraComparison `json:"weapons"`
	ArmorTypes []string           `json:"armor_types"`
}

type BarrageCrossbow struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Name         string  `json:"name"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Heading      float64 `json:"heading"`
	Elevation    float64 `json:"elevation"`
}

type BarrageTarget struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Radius       float64 `json:"radius"`
}

type BarrageOptimizationRequest struct {
	Crossbows []BarrageCrossbow `json:"crossbows"`
	Target    BarrageTarget     `json:"target"`
	MaxShotsPerCrossbow int    `json:"max_shots_per_crossbow"`
	SpreadAngle float64         `json:"spread_angle"`
	EnableCollisionAvoidance bool `json:"enable_collision_avoidance,omitempty"`
	FireDelayBaseMs float64      `json:"fire_delay_base_ms,omitempty"`
	SafetySeparationM float64     `json:"safety_separation_m,omitempty"`
}

type BarrageShot struct {
	CrossbowID    string  `json:"crossbow_id"`
	CrossbowName  string  `json:"crossbow_name"`
	Azimuth       float64 `json:"azimuth"`
	Elevation     float64 `json:"elevation"`
	Range         float64 `json:"range"`
	FlightTime    float64 `json:"flight_time"`
	ImpactX       float64 `json:"impact_x"`
	ImpactY       float64 `json:"impact_y"`
	ArrivalTime   float64 `json:"arrival_time"`
	InitialVelocity float64 `json:"initial_velocity"`
	FireDelayMs   float64 `json:"fire_delay_ms"`
	MinSeparationM float64 `json:"min_separation_m,omitempty"`
	CollisionRisk string  `json:"collision_risk,omitempty"`
}

type CoverageGrid struct {
	MinX    float64   `json:"min_x"`
	MaxX    float64   `json:"max_x"`
	MinY    float64   `json:"min_y"`
	MaxY    float64   `json:"max_y"`
	CellSize float64  `json:"cell_size"`
	Grid     [][]int `json:"grid"`
}

type TrajectorySample struct {
	TimeS       float64
	CrossbowID  string
	ShotIndex   int
	X, Y, Z     float64
}

type BarrageOptimizationResponse struct {
	Shots          []BarrageShot `json:"shots"`
	Coverage       CoverageGrid  `json:"coverage"`
	TargetHitRate  float64       `json:"target_hit_rate"`
	AreaCoveredM2  float64       `json:"area_covered_m2"`
	ShotsInTarget  int           `json:"shots_in_target"`
	TotalShots     int           `json:"total_shots"`
	TimeWindow     float64       `json:"time_window_seconds"`
	KEConcentrated float64       `json:"ke_concentrated_joules"`
	CollisionsDetected int       `json:"collisions_detected"`
	SeparationWarnings int       `json:"separation_warnings"`
	AvgFireDelayMs float64       `json:"avg_fire_delay_ms"`
}


type AimTarget struct {
	X            float64 `json:"x"`
	Distance     float64 `json:"distance"`
	Height       float64 `json:"height"`
	Name         string  `json:"name"`
	ArmorType    string  `json:"armor_type"`
}

type AimShootRequest struct {
	Target         AimTarget `json:"target"`
	CrossbowType   string    `json:"crossbow_type"`
	ArrowType      string    `json:"arrow_type"`
	WindSpeed      float64   `json:"wind_speed"`
	WindDir        float64   `json:"wind_direction"`
	OperatorSkill  float64   `json:"operator_skill,omitempty"`
	UserElevation  float64   `json:"user_elevation_deg,omitempty"`
	UserAzimuth    float64   `json:"user_azimuth_deg,omitempty"`
	CalibrationRun bool     `json:"calibration_run,omitempty"`
}

type AimShootResponse struct {
	Success          bool              `json:"success"`
	Hit              bool              `json:"hit"`
	HitQuality       string            `json:"hit_quality,omitempty"`
	RequiredElevation float64          `json:"required_elevation"`
	RequiredAzimuth  float64           `json:"required_azimuth"`
	ActualRange      float64           `json:"actual_range"`
	FlightTime       float64           `json:"flight_time"`
	MaxHeight        float64           `json:"max_height"`
	ImpactVelocity   float64           `json:"impact_velocity"`
	KineticEnergy    float64           `json:"kinetic_energy"`
	ImpactX          float64           `json:"impact_x"`
	ImpactY          float64           `json:"impact_y"`
	ImpactZ          float64           `json:"impact_z,omitempty"`
	WindDriftX       float64           `json:"wind_drift_x"`
	WindDriftY       float64           `json:"wind_drift_y"`
	RangeErrorM      float64           `json:"range_error_m,omitempty"`
	HeightErrorM     float64           `json:"height_error_m,omitempty"`
	LateralErrorM    float64           `json:"lateral_error_m,omitempty"`
	TargetToleranceM float64           `json:"target_tolerance_m,omitempty"`
	PenetrationDepth float64           `json:"penetration_depth_mm"`
	PenetrationSuccess bool            `json:"penetration_success"`
	Trajectory       []TrajectoryPoint `json:"trajectory"`
	Message          string            `json:"message"`
	Score            int               `json:"score"`
	MaxPossibleScore int               `json:"max_possible_score,omitempty"`
	OperatorAppliedErrorElev float64    `json:"operator_error_elev_deg,omitempty"`
	OperatorAppliedErrorAzi  float64    `json:"operator_error_azi_deg,omitempty"`
}

type AimTargetPreset struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Distance   float64 `json:"distance"`
	Height     float64 `json:"height"`
	ArmorType  string  `json:"armor_type"`
	Difficulty string  `json:"difficulty"`
	Points     int     `json:"points"`
	Icon       string  `json:"icon"`
}
