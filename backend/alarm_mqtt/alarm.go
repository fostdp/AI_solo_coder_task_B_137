package alarm_mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"ballistics-system/models"
)

type AlarmService struct {
	pusher         *AlertPusher
	checker        *AlertChecker
	deformationMax float64
	minRange       float64
	Metrics        AlarmMetricsHooks
}

type AlarmMetricsHooks interface {
	IncAlert(level, typ string)
	IncMQTTReconnect()
	IncMQTTPublish()
}

type noopAlarmMetrics struct{}

func (noopAlarmMetrics) IncAlert(string, string)     {}
func (noopAlarmMetrics) IncMQTTReconnect()          {}
func (noopAlarmMetrics) IncMQTTPublish()            {}

func NewAlarmService(broker, clientID, topic, username, password string, deformationMax, minRange float64) *AlarmService {
	pusher := NewAlertPusher(broker, clientID, topic, username, password)
	checker := NewAlertChecker(deformationMax, minRange)

	return &AlarmService{
		pusher:         pusher,
		checker:        checker,
		deformationMax: deformationMax,
		minRange:       minRange,
		Metrics:        noopAlarmMetrics{},
	}
}

func (s *AlarmService) WithMetrics(m AlarmMetricsHooks) *AlarmService {
	s.Metrics = m
	if s.pusher != nil {
		s.pusher.metrics = m
	}
	return s
}

func (s *AlarmService) CheckSensor(data *models.SensorData) []*models.Alert {
	return s.checker.CheckSensor(data)
}

func (s *AlarmService) CheckRange(deviceID string, actualRange float64) *models.Alert {
	return s.checker.CheckRange(deviceID, actualRange)
}

func (s *AlarmService) Push(alert *models.Alert) {
	s.pusher.Push(alert)
}

func (s *AlarmService) PushAlerts(alerts []*models.Alert) {
	for _, a := range alerts {
		s.Push(a)
	}
}

func (s *AlarmService) Stop() {
	s.pusher.Stop()
}

func (s *AlarmService) RunAlertWorker(alertCh <-chan *models.Alert, storeFn func(*models.Alert)) {
	for alert := range alertCh {
		s.Metrics.IncAlert(alert.AlertLevel, alert.AlertType)
		if storeFn != nil {
			storeFn(alert)
		}
		s.Push(alert)
		log.Printf("[alarm_mqtt] ALERT [%s] %s: %s", alert.AlertLevel, alert.AlertType, alert.Message)
	}
}

type AlertPusher struct {
	client    mqtt.Client
	topic     string
	broker    string
	alertChan chan *models.Alert
	metrics   AlarmMetricsHooks
}

func NewAlertPusher(broker, clientID, topic, username, password string) *AlertPusher {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetCleanSession(true)

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("[alarm_mqtt] Connected to %s", broker)
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("[alarm_mqtt] Connection lost: %v", err)
		if pusher.metrics != nil {
			pusher.metrics.IncMQTTReconnect()
		}
	}

	pusher := &AlertPusher{
		broker:    broker,
		topic:     topic,
		alertChan: make(chan *models.Alert, 100),
	}
	pusher.client = mqtt.NewClient(opts)

	go pusher.connectLoop()
	go pusher.publishLoop()

	return pusher
}

func (p *AlertPusher) connectLoop() {
	for {
		if token := p.client.Connect(); token.Wait() && token.Error() != nil {
			log.Printf("[alarm_mqtt] Connect error: %v, retrying in 5s", token.Error())
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
}

func (p *AlertPusher) publishLoop() {
	for alert := range p.alertChan {
		p.publishAlert(alert)
	}
}

func (p *AlertPusher) publishAlert(alert *models.Alert) {
	if !p.client.IsConnected() {
		log.Println("[alarm_mqtt] Not connected, alert queued")
		return
	}

	payload, err := json.Marshal(alert)
	if err != nil {
		log.Printf("[alarm_mqtt] JSON marshal error: %v", err)
		return
	}

	topic := fmt.Sprintf("%s/%s/%s", p.topic, alert.AlertType, alert.DeviceID)
	token := p.client.Publish(topic, 1, false, payload)
	go func() {
		token.Wait()
		if token.Error() != nil {
			log.Printf("[alarm_mqtt] Publish error: %v", token.Error())
		} else {
			if p.metrics != nil {
				p.metrics.IncMQTTPublish()
			}
			log.Printf("[alarm_mqtt] Published: %s - %s", alert.AlertType, alert.Message)
		}
	}()
}

func (p *AlertPusher) Push(alert *models.Alert) {
	select {
	case p.alertChan <- alert:
	default:
		log.Println("[alarm_mqtt] Alert channel full, dropping alert")
	}
}

func (p *AlertPusher) Stop() {
	if p.client.IsConnected() {
		p.client.Disconnect(250)
	}
	close(p.alertChan)
}

type AlertChecker struct {
	deformationMax float64
	minRange       float64
	alertChan      chan<- *models.Alert
}

func NewAlertChecker(deformationMax, minRange float64) *AlertChecker {
	return &AlertChecker{
		deformationMax: deformationMax,
		minRange:       minRange,
	}
}

func (c *AlertChecker) CheckSensor(data *models.SensorData) []*models.Alert {
	var alerts []*models.Alert

	if data.ArmDeformation > c.deformationMax {
		level := "warning"
		if data.ArmDeformation > c.deformationMax*1.2 {
			level = "critical"
		}
		alert := &models.Alert{
			DeviceID:    data.DeviceID,
			Timestamp:   data.Timestamp,
			AlertType:   "arm_crack_risk",
			AlertLevel:  level,
			Message:     fmt.Sprintf("弩臂变形 %.2f mm 超过阈值 %.2f mm，存在裂纹风险", data.ArmDeformation, c.deformationMax),
			SensorValue: data.ArmDeformation,
			Threshold:   c.deformationMax,
		}
		alerts = append(alerts, alert)
	}

	return alerts
}

func (c *AlertChecker) CheckRange(deviceID string, actualRange float64) *models.Alert {
	if actualRange < c.minRange {
		level := "warning"
		if actualRange < c.minRange*0.7 {
			level = "critical"
		}
		alert := &models.Alert{
			DeviceID:    deviceID,
			Timestamp:   time.Now(),
			AlertType:   "insufficient_range",
			AlertLevel:  level,
			Message:     fmt.Sprintf("射程 %.2f m 低于最低要求 %.2f m", actualRange, c.minRange),
			SensorValue: actualRange,
			Threshold:   c.minRange,
		}
		return alert
	}
	return nil
}
