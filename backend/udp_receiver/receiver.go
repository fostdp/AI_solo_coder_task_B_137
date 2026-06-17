package udp_receiver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"ballistics-system/backend/models"
)

type MetricsHooks interface {
	ObserveUDPSize(n int)
	IncSensorPackets()
	IncInvalidPackets()
	SetPendingSensor(n int)
}

type noopMetrics struct{}

func (noopMetrics) ObserveUDPSize(int)           {}
func (noopMetrics) IncSensorPackets()            {}
func (noopMetrics) IncInvalidPackets()           {}
func (noopMetrics) SetPendingSensor(int)         {}

type Receiver struct {
	port     int
	conn     *net.UDPConn
	dataChan chan<- *models.ValidatedSensorData
	metrics  MetricsHooks
}

func NewReceiver(port int, dataChan chan<- *models.ValidatedSensorData) *Receiver {
	return &Receiver{
		port:     port,
		dataChan: dataChan,
		metrics:  noopMetrics{},
	}
}

func (r *Receiver) WithMetrics(m MetricsHooks) *Receiver {
	r.metrics = m
	return r
}

func (r *Receiver) Start() error {
	addr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(r.port))
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	r.conn = conn
	log.Printf("[udp_receiver] Listening on port %d", r.port)

	go r.receiveLoop()
	return nil
}

func (r *Receiver) receiveLoop() {
	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("[udp_receiver] Read error from %v: %v", remoteAddr, err)
			continue
		}
		r.metrics.ObserveUDPSize(n)
		r.metrics.IncSensorPackets()

		var data models.SensorData
		if err := json.Unmarshal(buf[:n], &data); err != nil {
			log.Printf("[udp_receiver] JSON parse error: %v, raw: %s", err, string(buf[:n]))
			r.metrics.IncInvalidPackets()
			continue
		}

		if data.Timestamp.IsZero() {
			data.Timestamp = time.Now()
		}

		validated := r.validate(data)
		if validated.IsValid {
			r.metrics.SetPendingSensor(len(r.dataChan))
			r.dataChan <- validated
		} else {
			r.metrics.IncInvalidPackets()
			log.Printf("[udp_receiver] Invalid data from %s: %v", data.DeviceID, validated.Errors)
		}
	}
}

func (r *Receiver) validate(data models.SensorData) *models.ValidatedSensorData {
	var errors []string

	if data.DeviceID == "" {
		errors = append(errors, "device_id is required")
	}

	if data.BowstringTension < 0 {
		errors = append(errors, fmt.Sprintf("bowstring_tension %.2f is negative", data.BowstringTension))
	}

	if data.BowstringTension > 100000 {
		errors = append(errors, fmt.Sprintf("bowstring_tension %.2f exceeds physical limit", data.BowstringTension))
	}

	if data.ArmDeformation < 0 {
		errors = append(errors, fmt.Sprintf("arm_deformation %.3f is negative", data.ArmDeformation))
	}

	if data.ArrowInitialVelocity < 0 {
		errors = append(errors, fmt.Sprintf("arrow_initial_velocity %.2f is negative", data.ArrowInitialVelocity))
	}

	if data.ArrowInitialVelocity > 500 {
		errors = append(errors, fmt.Sprintf("arrow_initial_velocity %.2f exceeds physical limit", data.ArrowInitialVelocity))
	}

	if data.Temperature < -60 || data.Temperature > 80 {
		errors = append(errors, fmt.Sprintf("temperature %.1f out of range [-60, 80]", data.Temperature))
	}

	if data.Humidity < 0 || data.Humidity > 100 {
		errors = append(errors, fmt.Sprintf("humidity %.1f out of range [0, 100]", data.Humidity))
	}

	return &models.ValidatedSensorData{
		Data:    &data,
		IsValid: len(errors) == 0,
		Errors:  errors,
	}
}

func (r *Receiver) Stop() {
	if r.conn != nil {
		r.conn.Close()
	}
}
