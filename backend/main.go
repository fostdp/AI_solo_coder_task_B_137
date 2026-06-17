package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	ch "ballistics-system/clickhouse"
	"ballistics-system/config"
	"ballistics-system/metrics"
	"ballistics-system/models"

	alarm_mqtt "ballistics-system/alarm_mqtt"
	ballistic_simulator "ballistics-system/ballistic_simulator"
	penetration_analyzer "ballistics-system/penetration_analyzer"
	udp_receiver "ballistics-system/udp_receiver"

	"ballistics-system/api"
)

type metricsAdapter struct {
	m *metrics.Metrics
}

func (a *metricsAdapter) ObserveUDPSize(n int)                       { a.m.UDPSizeBytes.Observe(float64(n)) }
func (a *metricsAdapter) IncSensorPackets()                         { a.m.SensorPacketsTotal.Inc() }
func (a *metricsAdapter) IncInvalidPackets()                        { a.m.SensorPacketsInvalid.Inc() }
func (a *metricsAdapter) SetPendingSensor(n int)                    { a.m.PendingSensorCount.Set(float64(n)) }
func (a *metricsAdapter) IncSimulation()                            { a.m.SimulationsTotal.Inc() }
func (a *metricsAdapter) ObserveSimDuration(d float64)              { a.m.SimDurationSeconds.Observe(d) }
func (a *metricsAdapter) ObserveImpactVelocity(v float64)           { a.m.ImpactVelocity.Observe(v) }
func (a *metricsAdapter) IncActiveSim()                             { a.m.ActiveSimulations.Inc() }
func (a *metricsAdapter) DecActiveSim()                             { a.m.ActiveSimulations.Dec() }
func (a *metricsAdapter) SetPendingSim(n int)                       { a.m.PendingSimCount.Set(float64(n)) }
func (a *metricsAdapter) IncPenetration()                           { a.m.PenetrationsTotal.Inc() }
func (a *metricsAdapter) ObservePenDuration(d float64)              { a.m.PenDurationSeconds.Observe(d) }
func (a *metricsAdapter) ObservePenetrationDepth(mm float64)        { a.m.PenetrationDepth.Observe(mm) }
func (a *metricsAdapter) IncActivePen()                             { a.m.ActivePenetrations.Inc() }
func (a *metricsAdapter) DecActivePen()                             { a.m.ActivePenetrations.Dec() }
func (a *metricsAdapter) SetPendingPen(n int)                       { a.m.PendingPenCount.Set(float64(n)) }
func (a *metricsAdapter) IncAlert(level, typ string)                { a.m.AlertsTotal.WithLabelValues(level, typ).Inc() }
func (a *metricsAdapter) IncMQTTReconnect()                         { a.m.MQTTReconnectsTotal.Inc() }
func (a *metricsAdapter) IncMQTTPublish()                           { a.m.MQTTMessagesTotal.Inc() }
func (a *metricsAdapter) IncDBInsert(table string)                  { a.m.DBInsertsTotal.WithLabelValues(table).Inc() }
func (a *metricsAdapter) IncDBInsertError(table string)             { a.m.DBInsertErrorsTotal.WithLabelValues(table).Inc() }

func main() {
	cfg := config.Load()

	dynCfg := config.LoadDynamics(cfg.DynamicsPath)
	armorCfg := config.LoadArmor(cfg.ArmorPath)

	m := metrics.New()
	adp := &metricsAdapter{m: m}

	metricsAddr := cfg.MetricsAddr
	if metricsAddr == "" {
		metricsAddr = ":9090"
	}
	metricsSrv := m.StartMetricsServer(metricsAddr)
	log.Printf("Metrics/pprof server on %s (/metrics, /debug/pprof/)", metricsAddr)

	var store *ch.Store
	if s, err := ch.NewStore(cfg.ClickHouseDSN); err != nil {
		log.Printf("Warning: ClickHouse connection failed: %v", err)
		log.Println("Continuing without database...")
	} else {
		s.Metrics = adp
		store = s
	}
	if store != nil {
		defer store.Close()
	}

	simEngine := ballistic_simulator.NewSimulator(dynCfg)
	simEngine.Metrics = adp

	penAnalyzer := penetration_analyzer.NewAnalyzer(armorCfg)
	penAnalyzer.Metrics = adp

	alarmService := alarm_mqtt.NewAlarmService(
		cfg.MQTTBroker, cfg.MQTTClientID, cfg.MQTTTopic,
		cfg.MQTTUsername, cfg.MQTTPassword,
		cfg.DeformationMax, cfg.MinRange,
	).WithMetrics(adp)
	defer alarmService.Stop()

	sensorDataCh := make(chan *models.ValidatedSensorData, 1000)
	simJobCh := make(chan *models.SimJob, 100)
	simResultCh := make(chan *models.SimulationResult, 100)
	penJobCh := make(chan *models.PenJob, 100)
	penResultCh := make(chan *models.PenetrationResult, 100)
	alertCh := make(chan *models.Alert, 100)

	udpReceiver := udp_receiver.NewReceiver(cfg.UDPPort, sensorDataCh)
	udpReceiver = udpReceiver.WithMetrics(adp)
	if err := udpReceiver.Start(); err != nil {
		log.Fatalf("Failed to start UDP receiver: %v", err)
	}
	defer udpReceiver.Stop()

	go simEngine.RunSimulationWorker(simJobCh, simResultCh)
	go penAnalyzer.RunPenetrationWorker(penJobCh, penResultCh)
	go alarmService.RunAlertWorker(alertCh, func(alert *models.Alert) {
		if store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := store.InsertAlert(ctx, alert); err != nil {
				log.Printf("Insert alert error: %v", err)
			}
		}
	})

	orch := &pipelineOrchestrator{
		store:        store,
		simEngine:    simEngine,
		penAnalyzer:  penAnalyzer,
		alarmService: alarmService,
		dynCfg:       dynCfg,
		deformMax:    cfg.DeformationMax,
		minRange:     cfg.MinRange,
		pending:      make(map[string]chan *models.SimulationResult),
		mu:           &sync.Mutex{},
	}

	go orch.routeSimResults(simResultCh)
	go orch.routePenResults(penResultCh)
	go orch.processSensorData(sensorDataCh, simJobCh, penJobCh, alertCh)

	httpServer := api.NewServer(cfg.HTTPAddr, store, simEngine, penAnalyzer, dynCfg)
	go func() {
		log.Printf("HTTP server starting on %s", cfg.HTTPAddr)
		if err := httpServer.Start(); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Println("Ballistics System started successfully")
	log.Printf("  UDP port: %d", cfg.UDPPort)
	log.Printf("  HTTP addr: %s", cfg.HTTPAddr)
	log.Printf("  Metrics/pprof: %s", metricsAddr)
	log.Printf("  MQTT broker: %s", cfg.MQTTBroker)
	log.Printf("  Dynamics config: %s", cfg.DynamicsPath)
	log.Printf("  Armor config: %s", cfg.ArmorPath)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if metricsSrv != nil {
		_ = metricsSrv.Shutdown(shutdownCtx)
	}
	_ = httpServer.Shutdown(shutdownCtx)

	time.Sleep(500 * time.Millisecond)
	log.Println("Ballistics System stopped")
}

type pipelineOrchestrator struct {
	store        *ch.Store
	simEngine    *ballistic_simulator.Simulator
	penAnalyzer  *penetration_analyzer.Analyzer
	alarmService *alarm_mqtt.AlarmService
	dynCfg       *config.DynamicsConfig
	deformMax    float64
	minRange     float64
	pending      map[string]chan *models.SimulationResult
	mu           *sync.Mutex
}

func (o *pipelineOrchestrator) registerPending(deviceID string, ch chan *models.SimulationResult) {
	o.mu.Lock()
	o.pending[deviceID] = ch
	o.mu.Unlock()
}

func (o *pipelineOrchestrator) unregisterPending(deviceID string) {
	o.mu.Lock()
	delete(o.pending, deviceID)
	o.mu.Unlock()
}

func (o *pipelineOrchestrator) routeSimResults(simResultCh <-chan *models.SimulationResult) {
	for result := range simResultCh {
		o.mu.Lock()
		ch, ok := o.pending[result.DeviceID]
		o.mu.Unlock()

		if ok {
			ch <- result
		} else {
			log.Printf("[orchestrator] SimResult for unknown device %s", result.DeviceID)
			if o.store != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = o.store.InsertSimulationResult(ctx, result)
				cancel()
			}
		}
	}
}

func (o *pipelineOrchestrator) routePenResults(penResultCh <-chan *models.PenetrationResult) {
	for penResult := range penResultCh {
		if o.store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := o.store.InsertArmorPerformance(ctx, o.penAnalyzer.ToArmorPerformance(penResult)); err != nil {
				log.Printf("Insert armor performance error: %v", err)
			}
			cancel()
		}
	}
}

func (o *pipelineOrchestrator) processSensorData(
	sensorCh <-chan *models.ValidatedSensorData,
	simJobCh chan<- *models.SimJob,
	penJobCh chan<- *models.PenJob,
	alertCh chan<- *models.Alert,
) {
	for validated := range sensorCh {
		data := validated.Data

		if o.store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := o.store.InsertSensorData(ctx, data); err != nil {
				log.Printf("Insert sensor data error: %v", err)
			}
			cancel()
		}

		alerts := o.alarmService.CheckSensor(data)
		for _, a := range alerts {
			alertCh <- a
		}

		if data.ArrowInitialVelocity > 0 {
			simParams := o.buildSimParams(data)

			resultCh := make(chan *models.SimulationResult, 1)
			o.registerPending(data.DeviceID, resultCh)

			simJobCh <- &models.SimJob{
				Params:   simParams,
				DeviceID: data.DeviceID,
			}

			go o.awaitSimResult(data.DeviceID, resultCh, simParams, penJobCh, alertCh)
		}
	}
}

func (o *pipelineOrchestrator) buildSimParams(data *models.SensorData) *models.SimulationParams {
	spinRate := o.dynCfg.Defaults.SpinRate
	if data.ArrowSpinRate > 0 {
		spinRate = data.ArrowSpinRate
	}
	return &models.SimulationParams{
		InitialVelocity: data.ArrowInitialVelocity,
		LaunchAngle:     o.dynCfg.Defaults.LaunchAngle,
		AzimuthAngle:    o.dynCfg.Defaults.AzimuthAngle,
		ArrowMass:       o.dynCfg.Defaults.ArrowMass,
		ArrowDiameter:   o.dynCfg.Defaults.ArrowDiameter,
		ArrowLength:     o.dynCfg.Defaults.ArrowLength,
		DragCoefficient: o.dynCfg.Defaults.DragCoefficient,
		AirDensity:      o.dynCfg.Simulation.AirDensitySea,
		SpinRate:        spinRate,
	}
}

func (o *pipelineOrchestrator) awaitSimResult(
	deviceID string,
	resultCh <-chan *models.SimulationResult,
	simParams *models.SimulationParams,
	penJobCh chan<- *models.PenJob,
	alertCh chan<- *models.Alert,
) {
	defer o.unregisterPending(deviceID)

	select {
	case simResult := <-resultCh:
		rangeAlert := o.alarmService.CheckRange(deviceID, simResult.Range)
		if rangeAlert != nil {
			alertCh <- rangeAlert
		}

		penJobCh <- &models.PenJob{
			ImpactVelocity: simResult.ImpactVelocity,
			ArrowMass:      simParams.ArrowMass,
			ArrowDiameter:  simParams.ArrowDiameter,
			ArrowLength:    simParams.ArrowLength,
			SpinRate:       simResult.ImpactSpinRate,
			ArmorType:      "plate",
			ArrowHeadType:  "bodkin",
			ArmorThickness: 0,
			DeviceID:       deviceID,
		}

		exitVel, energyBudget := o.simEngine.SimulateFullRelease(simParams.ArrowMass)
		if o.store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := o.store.InsertSimulationResult(ctx, simResult); err != nil {
				log.Printf("Insert simulation result error: %v", err)
			}
			if energyBudget != nil {
				if err := o.store.InsertBowReleaseEnergy(ctx, &models.BowReleaseEnergy{
					DeviceID:               deviceID,
					Timestamp:              time.Now(),
					InitialPotentialEnergy: energyBudget["initial_potential"],
					ArrowKE:                energyBudget["arrow_ke"],
					ArmKE:                  energyBudget["arm_ke"],
					StringKE:               0,
					HysteresisLoss:         energyBudget["hysteresis_loss"],
					ViscousLoss:            energyBudget["viscous_loss"],
					InternalLoss:           energyBudget["internal_loss"],
					NonlinearLoss:          energyBudget["nonlinear_loss"],
					TotalDissipated:        energyBudget["dissipated"],
					Efficiency:             energyBudget["efficiency"],
					ReleaseTime:            energyBudget["release_time"],
					ExitVelocity:           exitVel,
				}); err != nil {
					log.Printf("Insert bow release energy error: %v", err)
				}
			}
			cancel()
		}

		log.Printf("[orchestrator] [%s] v0=%.1fm/s range=%.1fm impact_v=%.1fm/s KE=%.1fJ efficiency=%.1f%%",
			deviceID, simParams.InitialVelocity, simResult.Range,
			simResult.ImpactVelocity, simResult.KineticEnergy,
			energyBudget["efficiency"]*100)

	case <-time.After(10 * time.Second):
		log.Printf("[orchestrator] Timeout waiting for sim result for %s", deviceID)
	}
}

var _ = fmt.Sprintf
var _ = http.StatusOK
