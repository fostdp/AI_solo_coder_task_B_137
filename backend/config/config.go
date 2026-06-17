package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

type Config struct {
	UDPPort        int
	HTTPAddr       string
	MetricsAddr    string
	ClickHouseDSN  string
	MQTTBroker     string
	MQTTClientID   string
	MQTTTopic      string
	MQTTUsername   string
	MQTTPassword   string
	DeformationMax float64
	MinRange       float64
	DynamicsPath   string
	ArmorPath      string
}

type BowConfig struct {
	ArmLength        float64 `json:"arm_length"`
	ArmThickness     float64 `json:"arm_thickness"`
	ArmWidth         float64 `json:"arm_width"`
	ArmMass          float64 `json:"arm_mass"`
	StringLength     float64 `json:"string_length"`
	StringMass       float64 `json:"string_mass"`
	StringYoungMod   float64 `json:"string_young_mod"`
	StringCrossArea  float64 `json:"string_cross_area"`
	DrawLength       float64 `json:"draw_length"`
	PeakTension      float64 `json:"peak_tension"`
	NonlinearDamping float64 `json:"nonlinear_damping"`
	HysteresisFactor float64 `json:"hysteresis_factor"`
	ViscousDamping   float64 `json:"viscous_damping"`
	InternalDamping  float64 `json:"internal_damping"`
}

type SimulationConfig struct {
	TimeStep         float64 `json:"time_step"`
	ReleaseTimeStep  float64 `json:"release_time_step"`
	MaxSimTime       float64 `json:"max_sim_time"`
	ReleaseDuration  float64 `json:"release_duration"`
	Gravity          float64 `json:"gravity"`
	AirDensitySea    float64 `json:"air_density_sea"`
	YoungModulusWood float64 `json:"young_modulus_wood"`
	PoissonRatioWood float64 `json:"poisson_ratio_wood"`
}

type DefaultsConfig struct {
	ArrowMass       float64 `json:"arrow_mass"`
	ArrowDiameter   float64 `json:"arrow_diameter"`
	ArrowLength     float64 `json:"arrow_length"`
	SpinRate        float64 `json:"spin_rate"`
	DragCoefficient float64 `json:"drag_coefficient"`
	LaunchAngle     float64 `json:"launch_angle"`
	AzimuthAngle    float64 `json:"azimuth_angle"`
}

type AerodynamicsConfig struct {
	LiftCoefficient         float64 `json:"lift_coefficient"`
	MagnusCoefficient       float64 `json:"magnus_coefficient"`
	SpinDecayRate           float64 `json:"spin_decay_rate"`
	PitchDampingBase        float64 `json:"pitch_damping_base"`
	AeroMomentCoefficient   float64 `json:"aero_moment_coefficient"`
}

type CrossbowTypeConfig struct {
	Type           string  `json:"type"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Era            string  `json:"era"`
	DrawForce      float64 `json:"draw_force"`
	DrawLength     float64 `json:"draw_length"`
	ArrowMass      float64 `json:"arrow_mass"`
	ArrowLength    float64 `json:"arrow_length"`
	ArrowDiameter  float64 `json:"arrow_diameter"`
	TypicalVelocity float64 `json:"typical_velocity"`
	TypicalRange   float64 `json:"typical_range"`
	SpinRate       float64 `json:"spin_rate"`
	BowEfficiency  float64 `json:"bow_efficiency"`
	CrewSize       int     `json:"crew_size"`
	ReloadSeconds  float64 `json:"reload_seconds"`
}

type ModernWeaponConfig struct {
	Type            string  `json:"type"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Era             string  `json:"era"`
	BulletMass      float64 `json:"bullet_mass"`
	BulletDiameter  float64 `json:"bullet_diameter"`
	BulletLength    float64 `json:"bullet_length"`
	MuzzleVelocity  float64 `json:"muzzle_velocity"`
	MaxRange        float64 `json:"max_range"`
	EffectiveRange  float64 `json:"effective_range"`
	DragCoefficient float64 `json:"drag_coefficient"`
	SpinRate        float64 `json:"spin_rate"`
	Hardness        float64 `json:"hardness"`
	TipArea         float64 `json:"tip_area"`
	CrewSize        int     `json:"crew_size"`
	ReloadSeconds   float64 `json:"reload_seconds"`
}

type DynamicsConfig struct {
	Bow           BowConfig                    `json:"bow"`
	Simulation    SimulationConfig             `json:"simulation"`
	Defaults      DefaultsConfig               `json:"defaults"`
	Aerodynamics  AerodynamicsConfig           `json:"aerodynamics"`
	CrossbowTypes map[string]CrossbowTypeConfig `json:"crossbow_types"`
	ModernWeapons map[string]ModernWeaponConfig `json:"modern_weapons"`
}

type ArmorEntryConfig struct {
	Type          string  `json:"type"`
	Thickness     float64 `json:"thickness"`
	Density       float64 `json:"density"`
	YieldStrength float64 `json:"yield_strength"`
	Hardness      float64 `json:"hardness"`
	Name          string  `json:"name"`
}

type ArrowHeadEntryConfig struct {
	Type        string  `json:"type"`
	TipDiameter float64 `json:"tip_diameter"`
	TipArea     float64 `json:"tip_area"`
	TipMass     float64 `json:"tip_mass"`
	Hardness    float64 `json:"hardness"`
	Name        string  `json:"name"`
}

type GyroConfig struct {
	YawStableThreshold      float64 `json:"yaw_stable_threshold"`
	YawModerateThreshold    float64 `json:"yaw_moderate_threshold"`
	YawMarginalThreshold    float64 `json:"yaw_marginal_threshold"`
	StabilityPenaltyFull    float64 `json:"stability_penalty_full"`
	StabilityPenaltyModerate float64 `json:"stability_penalty_moderate"`
	StabilityPenaltyPoor    float64 `json:"stability_penalty_poor"`
	RotaryEnergyBoostFactor float64 `json:"rotary_energy_boost_factor"`
	StabilityClampMin       float64 `json:"stability_clamp_min"`
	StabilityClampMax       float64 `json:"stability_clamp_max"`
	LowVelocityThreshold    float64 `json:"low_velocity_threshold"`
	LowVelocityStability    float64 `json:"low_velocity_stability"`
}

type ArmorConfig struct {
	Armors     map[string]ArmorEntryConfig `json:"armors"`
	ArrowHeads map[string]ArrowHeadEntryConfig `json:"arrow_heads"`
	Gyro       GyroConfig                  `json:"gyro"`
}

func Load() *Config {
	_ = os.Getenv("GOPATH")
	cfg := &Config{
		UDPPort:        getEnvInt("UDP_PORT", 8080),
		HTTPAddr:       getEnvStr("HTTP_ADDR", ":8081"),
		MetricsAddr:    getEnvStr("METRICS_ADDR", ":9090"),
		ClickHouseDSN:  getEnvStr("CLICKHOUSE_DSN", "clickhouse://localhost:9000?database=ballistics&username=default&password="),
		MQTTBroker:     getEnvStr("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTClientID:   getEnvStr("MQTT_CLIENT_ID", "ballistics-alert"),
		MQTTTopic:      getEnvStr("MQTT_TOPIC", "ballistics/alerts"),
		MQTTUsername:   getEnvStr("MQTT_USERNAME", ""),
		MQTTPassword:   getEnvStr("MQTT_PASSWORD", ""),
		DeformationMax: getEnvFloat("DEFORMATION_MAX", 15.0),
		MinRange:       getEnvFloat("MIN_RANGE", 300.0),
		DynamicsPath:   getEnvStr("DYNAMICS_CONFIG_PATH", "config/dynamics_params.json"),
		ArmorPath:      getEnvStr("ARMOR_CONFIG_PATH", "config/armor_params.json"),
	}
	log.Printf("Config loaded: UDP=%d HTTP=%s METRICS=%s", cfg.UDPPort, cfg.HTTPAddr, cfg.MetricsAddr)
	return cfg
}

func LoadDynamics(path string) *DynamicsConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read dynamics config %s: %v", path, err)
	}
	var cfg DynamicsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse dynamics config: %v", err)
	}
	log.Printf("Dynamics config loaded from %s", path)
	return &cfg
}

func LoadArmor(path string) *ArmorConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read armor config %s: %v", path, err)
	}
	var cfg ArmorConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse armor config: %v", err)
	}
	log.Printf("Armor config loaded from %s (%d armors, %d arrow heads)", path, len(cfg.Armors), len(cfg.ArrowHeads))
	return &cfg
}

func getEnvStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
