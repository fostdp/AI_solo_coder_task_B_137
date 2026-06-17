package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"ballistics-system/backend/models"
)

type Store struct {
	conn    driver.Conn
	Metrics StoreMetricsHooks
}

type StoreMetricsHooks interface {
	IncDBInsert(table string)
	IncDBInsertError(table string)
}

type noopStoreMetrics struct{}

func (noopStoreMetrics) IncDBInsert(string)      {}
func (noopStoreMetrics) IncDBInsertError(string) {}

func NewStore(dsn string) (*Store, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DSN: %w", err)
	}

	opts.DialTimeout = 10 * time.Second
	opts.MaxOpenConns = 10
	opts.MaxIdleConns = 5

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	log.Println("ClickHouse connected successfully")
	return &Store{conn: conn, Metrics: noopStoreMetrics{}}, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) InsertSensorData(ctx context.Context, data *models.SensorData) error {
	err := s.conn.Exec(ctx, `
		INSERT INTO sensor_data (device_id, timestamp, bowstring_tension, arm_deformation, arrow_initial_velocity, arrow_spin_rate, penetration_depth, temperature, humidity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, data.DeviceID, data.Timestamp, data.BowstringTension, data.ArmDeformation,
		data.ArrowInitialVelocity, data.ArrowSpinRate, data.PenetrationDepth, data.Temperature, data.Humidity)
	if err == nil {
		s.Metrics.IncDBInsert("sensor_data")
	} else {
		s.Metrics.IncDBInsertError("sensor_data")
	}
	return err
}

func (s *Store) InsertSimulationResult(ctx context.Context, result *models.SimulationResult) error {
	trajJSON, _ := json.Marshal(result.Trajectory)
	err := s.conn.Exec(ctx, `
		INSERT INTO simulation_results (device_id, timestamp, initial_velocity, launch_angle, flight_time, max_height, range, impact_velocity, kinetic_energy, impact_spin_rate, impact_gyro_stab, trajectory, armor_type, penetration_depth, penetration_success)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.DeviceID, result.Timestamp, result.InitialVelocity, result.LaunchAngle,
		result.FlightTime, result.MaxHeight, result.Range, result.ImpactVelocity,
		result.KineticEnergy, result.ImpactSpinRate, result.ImpactGyroStab,
		string(trajJSON), result.ArmorType, result.PenetrationDepth, result.PenetrationSuccess)
	if err == nil {
		s.Metrics.IncDBInsert("simulation_results")
	} else {
		s.Metrics.IncDBInsertError("simulation_results")
	}
	return err
}

func (s *Store) InsertAlert(ctx context.Context, alert *models.Alert) error {
	err := s.conn.Exec(ctx, `
		INSERT INTO alerts (device_id, timestamp, alert_type, alert_level, message, sensor_value, threshold, acknowledged)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, alert.DeviceID, alert.Timestamp, alert.AlertType, alert.AlertLevel,
		alert.Message, alert.SensorValue, alert.Threshold, alert.Acknowledged)
	if err == nil {
		s.Metrics.IncDBInsert("alerts")
	} else {
		s.Metrics.IncDBInsertError("alerts")
	}
	return err
}

func (s *Store) InsertArmorPerformance(ctx context.Context, p *models.ArmorPerformance) error {
	err := s.conn.Exec(ctx, `
		INSERT INTO armor_performance (timestamp, armor_type, armor_thickness, impact_velocity, arrow_mass, arrow_diameter, arrow_length, spin_rate, gyro_stability, yaw_angle, effective_area, arrow_head_type, penetration_depth, residual_velocity, energy_absorbed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.Timestamp, p.ArmorType, p.ArmorThickness, p.ImpactVelocity, p.ArrowMass,
		p.ArrowDiameter, p.ArrowLength, p.SpinRate, p.GyroStability,
		p.YawAngle, p.EffectiveArea, p.ArrowHeadType,
		p.PenetrationDepth, p.ResidualVelocity, p.EnergyAbsorbed)
	if err == nil {
		s.Metrics.IncDBInsert("armor_performance")
	} else {
		s.Metrics.IncDBInsertError("armor_performance")
	}
	return err
}

func (s *Store) InsertBowReleaseEnergy(ctx context.Context, e *models.BowReleaseEnergy) error {
	err := s.conn.Exec(ctx, `
		INSERT INTO bow_release_energy (device_id, timestamp, initial_potential_energy, arrow_ke, arm_ke, string_ke, hysteresis_loss, viscous_loss, internal_loss, nonlinear_loss, total_dissipated, efficiency, release_time, exit_velocity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.DeviceID, e.Timestamp, e.InitialPotentialEnergy, e.ArrowKE, e.ArmKE,
		e.StringKE, e.HysteresisLoss, e.ViscousLoss, e.InternalLoss,
		e.NonlinearLoss, e.TotalDissipated, e.Efficiency, e.ReleaseTime, e.ExitVelocity)
	if err == nil {
		s.Metrics.IncDBInsert("bow_release_energy")
	} else {
		s.Metrics.IncDBInsertError("bow_release_energy")
	}
	return err
}

func (s *Store) QuerySensorData(ctx context.Context, deviceID string, limit int) ([]models.SensorData, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT device_id, timestamp, bowstring_tension, arm_deformation, arrow_initial_velocity, arrow_spin_rate, penetration_depth, temperature, humidity
		FROM sensor_data
		WHERE device_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SensorData
	for rows.Next() {
		var d models.SensorData
		if err := rows.Scan(&d.DeviceID, &d.Timestamp, &d.BowstringTension, &d.ArmDeformation,
			&d.ArrowInitialVelocity, &d.ArrowSpinRate, &d.PenetrationDepth, &d.Temperature, &d.Humidity); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, nil
}

func (s *Store) QueryRecentSimulations(ctx context.Context, limit int) ([]models.SimulationResult, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT device_id, timestamp, initial_velocity, launch_angle, flight_time, max_height, range, impact_velocity, kinetic_energy, impact_spin_rate, impact_gyro_stab, trajectory, armor_type, penetration_depth, penetration_success
		FROM simulation_results
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SimulationResult
	for rows.Next() {
		var r models.SimulationResult
		var trajStr string
		if err := rows.Scan(&r.DeviceID, &r.Timestamp, &r.InitialVelocity, &r.LaunchAngle,
			&r.FlightTime, &r.MaxHeight, &r.Range, &r.ImpactVelocity, &r.KineticEnergy,
			&r.ImpactSpinRate, &r.ImpactGyroStab,
			&trajStr, &r.ArmorType, &r.PenetrationDepth, &r.PenetrationSuccess); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(trajStr), &r.Trajectory)
		results = append(results, r)
	}
	return results, nil
}

func (s *Store) QueryAlerts(ctx context.Context, acknowledged *bool, limit int) ([]models.Alert, error) {
	var query string
	var args []interface{}
	if acknowledged != nil {
		query = `SELECT device_id, timestamp, alert_type, alert_level, message, sensor_value, threshold, acknowledged FROM alerts WHERE acknowledged = ? ORDER BY timestamp DESC LIMIT ?`
		args = []interface{}{*acknowledged, limit}
	} else {
		query = `SELECT device_id, timestamp, alert_type, alert_level, message, sensor_value, threshold, acknowledged FROM alerts ORDER BY timestamp DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		if err := rows.Scan(&a.DeviceID, &a.Timestamp, &a.AlertType, &a.AlertLevel,
			&a.Message, &a.SensorValue, &a.Threshold, &a.Acknowledged); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (s *Store) QueryArmorPerformance(ctx context.Context, armorType string, limit int) ([]models.ArmorPerformance, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT timestamp, armor_type, armor_thickness, impact_velocity, arrow_mass, arrow_diameter, arrow_length, spin_rate, gyro_stability, yaw_angle, effective_area, arrow_head_type, penetration_depth, residual_velocity, energy_absorbed
		FROM armor_performance
		WHERE armor_type = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, armorType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ArmorPerformance
	for rows.Next() {
		var p models.ArmorPerformance
		if err := rows.Scan(&p.Timestamp, &p.ArmorType, &p.ArmorThickness, &p.ImpactVelocity,
			&p.ArrowMass, &p.ArrowDiameter, &p.ArrowLength, &p.SpinRate,
			&p.GyroStability, &p.YawAngle, &p.EffectiveArea,
			&p.ArrowHeadType, &p.PenetrationDepth, &p.ResidualVelocity, &p.EnergyAbsorbed); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}
