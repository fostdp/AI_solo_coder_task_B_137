package api

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	ch "ballistics-system/backend/clickhouse"
	"ballistics-system/backend/config"
	"ballistics-system/backend/models"

	ballistic_simulator "ballistics-system/backend/ballistic_simulator"
	penetration_analyzer "ballistics-system/backend/penetration_analyzer"
	"ballistics-system/backend/power_comparator"
	"ballistics-system/backend/era_comparator"
	"ballistics-system/backend/salvo_optimizer"
	"ballistics-system/backend/vr_crossbow"
)

type Server struct {
	engine          *gin.Engine
	store           *ch.Store
	simEngine       *ballistic_simulator.Simulator
	penAnalyzer     *penetration_analyzer.Analyzer
	dynamicsCfg     *config.DynamicsConfig
	addr            string
	powerComparator *power_comparator.PowerComparator
	eraComparator   *era_comparator.EraComparator
	salvoOptimizer  *salvo_optimizer.SalvoOptimizer
	vrCrossbow      *vr_crossbow.VRCrossbow
}

func NewServer(addr string, store *ch.Store, simEngine *ballistic_simulator.Simulator, penAnalyzer *penetration_analyzer.Analyzer, dynamicsCfg *config.DynamicsConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	pc := power_comparator.NewPowerComparator(dynamicsCfg, simEngine, penAnalyzer)
	ec := era_comparator.NewEraComparator(dynamicsCfg, simEngine, penAnalyzer)
	so := salvo_optimizer.NewSalvoOptimizerWithWorkers(dynamicsCfg, simEngine, 4)
	vr := vr_crossbow.NewVRCrossbow(dynamicsCfg, simEngine, penAnalyzer, store)

	s := &Server{
		engine:          r,
		store:           store,
		simEngine:       simEngine,
		penAnalyzer:     penAnalyzer,
		dynamicsCfg:     dynamicsCfg,
		addr:            addr,
		powerComparator: pc,
		eraComparator:   ec,
		salvoOptimizer:  so,
		vrCrossbow:      vr,
	}

	r.Use(CORS())
	s.setupRoutes()
	return s
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func (s *Server) setupRoutes() {
	v1 := s.engine.Group("/api/v1")

	v1.GET("/health", s.health)

	v1.GET("/sensor/:device_id", s.getSensorData)
	v1.POST("/sensor", s.postSensorData)

	v1.POST("/simulate", s.simulate)
	v1.GET("/simulations", s.getSimulations)

	v1.POST("/penetrate", s.analyzePenetration)
	v1.POST("/penetrate/compare", s.compareArmors)
	v1.GET("/armors", s.getArmorTypes)
	v1.GET("/arrowheads", s.getArrowHeadTypes)
	v1.GET("/armor/:type/performance", s.getArmorPerformance)

	v1.GET("/alerts", s.getAlerts)
	v1.GET("/alerts/unacknowledged", s.getUnacknowledgedAlerts)

	v1.GET("/crossbows", s.listCrossbows)
	v1.POST("/compare/crossbows", s.compareCrossbows)
	v1.GET("/weapons/modern", s.listModernWeapons)
	v1.POST("/compare/era", s.compareEraWeapons)
	v1.POST("/barrage/optimize", s.optimizeBarrage)
	v1.GET("/aim/targets", s.listAimTargets)
	v1.POST("/aim/shoot", s.aimShoot)
}

func (s *Server) Start() error {
	return s.engine.Run(s.addr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	_ = srv
	return nil
}

func (s *Server) health(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "ok",
		"timestamp": time.Now(),
		"service":   "ballistics-system",
	})
}

func (s *Server) getSensorData(c *gin.Context) {
	deviceID := c.Param("device_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	data, err := s.store.QuerySensorData(ctx, deviceID, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": data, "count": len(data)})
}

func (s *Server) postSensorData(c *gin.Context) {
	var data models.SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := s.store.InsertSensorData(ctx, &data); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"status": "ok", "data": data})
}

func (s *Server) simulate(c *gin.Context) {
	var params models.SimulationParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := s.simEngine.Simulate(&params)

	deviceID := c.Query("device_id")
	if deviceID == "" {
		deviceID = "api-sim"
	}
	result.DeviceID = deviceID

	armorType := c.Query("armor")
	if armorType == "" {
		armorType = "plate"
	}
	arrowType := c.Query("arrow")
	if arrowType == "" {
		arrowType = "bodkin"
	}

	penResult := s.penAnalyzer.AnalyzeWithSpin(
		result.ImpactVelocity,
		params.ArrowMass,
		params.ArrowDiameter,
		params.ArrowLength,
		result.ImpactSpinRate,
		armorType,
		arrowType,
		0,
	)
	result.ArmorType = armorType
	result.PenetrationDepth = penResult.PenetrationDepth
	result.PenetrationSuccess = penResult.Success

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if s.store != nil {
		if err := s.store.InsertSimulationResult(ctx, result); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if err := s.store.InsertArmorPerformance(ctx, s.penAnalyzer.ToArmorPerformance(penResult)); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(200, gin.H{
		"simulation":  result,
		"penetration": penResult,
	})
}

func (s *Server) getSimulations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	results, err := s.store.QueryRecentSimulations(ctx, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": results, "count": len(results)})
}

func (s *Server) analyzePenetration(c *gin.Context) {
	var req struct {
		ImpactVelocity float64 `json:"impact_velocity" binding:"required"`
		ArrowMass      float64 `json:"arrow_mass"`
		ArrowDiameter  float64 `json:"arrow_diameter"`
		ArrowLength    float64 `json:"arrow_length"`
		SpinRate       float64 `json:"spin_rate"`
		ArmorType      string  `json:"armor_type" binding:"required"`
		ArrowHeadType  string  `json:"arrow_head_type"`
		ArmorThickness float64 `json:"armor_thickness"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ArrowMass == 0 {
		req.ArrowMass = 0.2
	}
	if req.ArrowDiameter == 0 {
		req.ArrowDiameter = 0.012
	}
	if req.ArrowLength == 0 {
		req.ArrowLength = 1.0
	}
	if req.SpinRate == 0 {
		req.SpinRate = 25.0
	}
	if req.ArrowHeadType == "" {
		req.ArrowHeadType = "bodkin"
	}

	result := s.penAnalyzer.AnalyzeWithSpin(
		req.ImpactVelocity,
		req.ArrowMass,
		req.ArrowDiameter,
		req.ArrowLength,
		req.SpinRate,
		req.ArmorType,
		req.ArrowHeadType,
		req.ArmorThickness,
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if s.store != nil {
		_ = s.store.InsertArmorPerformance(ctx, s.penAnalyzer.ToArmorPerformance(result))
	}

	c.JSON(200, result)
}

func (s *Server) compareArmors(c *gin.Context) {
	var req struct {
		ImpactVelocity float64 `json:"impact_velocity" binding:"required"`
		ArrowMass      float64 `json:"arrow_mass"`
		ArrowDiameter  float64 `json:"arrow_diameter"`
		ArrowLength    float64 `json:"arrow_length"`
		SpinRate       float64 `json:"spin_rate"`
		ArrowHeadType  string  `json:"arrow_head_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ArrowMass == 0 {
		req.ArrowMass = 0.2
	}
	if req.ArrowDiameter == 0 {
		req.ArrowDiameter = 0.012
	}
	if req.ArrowLength == 0 {
		req.ArrowLength = 1.0
	}
	if req.SpinRate == 0 {
		req.SpinRate = 25.0
	}
	if req.ArrowHeadType == "" {
		req.ArrowHeadType = "bodkin"
	}

	results := s.penAnalyzer.CompareArmorsWithSpin(
		req.ImpactVelocity,
		req.ArrowMass,
		req.ArrowDiameter,
		req.ArrowLength,
		req.SpinRate,
		req.ArrowHeadType,
	)
	c.JSON(200, gin.H{"results": results})
}

func (s *Server) getArmorTypes(c *gin.Context) {
	armors := s.penAnalyzer.ListArmorTypes()
	c.JSON(200, gin.H{"armors": armors})
}

func (s *Server) getArrowHeadTypes(c *gin.Context) {
	arrows := s.penAnalyzer.ListArrowHeadTypes()
	c.JSON(200, gin.H{"arrow_heads": arrows})
}

func (s *Server) getArmorPerformance(c *gin.Context) {
	armorType := c.Param("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	results, err := s.store.QueryArmorPerformance(ctx, armorType, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": results, "count": len(results)})
}

func (s *Server) getAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	alerts, err := s.store.QueryAlerts(ctx, nil, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": alerts, "count": len(alerts)})
}

func (s *Server) getUnacknowledgedAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	unack := false
	alerts, err := s.store.QueryAlerts(ctx, &unack, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": alerts, "count": len(alerts)})
}

func (s *Server) listCrossbows(c *gin.Context) {
	crossbows := s.powerComparator.ListCrossbows()
	c.JSON(200, gin.H{"crossbows": crossbows, "count": len(crossbows)})
}

func (s *Server) listModernWeapons(c *gin.Context) {
	weapons := s.eraComparator.ListModernWeapons()
	c.JSON(200, gin.H{"weapons": weapons, "count": len(weapons)})
}

func (s *Server) compareCrossbows(c *gin.Context) {
	var req struct {
		CrossbowTypes  []string `json:"crossbow_types"`
		ArrowHeadType  string   `json:"arrow_head_type"`
		LaunchAngle    float64  `json:"launch_angle"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	result := s.powerComparator.Compare(req.CrossbowTypes, req.ArrowHeadType, req.LaunchAngle)
	c.JSON(200, result)
}

func (s *Server) compareEraWeapons(c *gin.Context) {
	var req struct {
		CrossbowTypes  []string `json:"crossbow_types"`
		ModernTypes    []string `json:"modern_types"`
		ArrowHeadType  string   `json:"arrow_head_type"`
		CompareRange   float64  `json:"compare_range_m"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	result := s.eraComparator.Compare(req.CrossbowTypes, req.ModernTypes, req.ArrowHeadType, req.CompareRange)
	c.JSON(200, result)
}

func (s *Server) optimizeBarrage(c *gin.Context) {
	var req models.BarrageOptimizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.MaxShotsPerCrossbow <= 0 {
		req.MaxShotsPerCrossbow = 3
	}
	if req.SpreadAngle <= 0 {
		req.SpreadAngle = 10.0
	}
	if len(req.Crossbows) == 0 {
		defaults := []models.BarrageCrossbow{
			{ID: "cb-1", Type: "bed_crossbow_triple", Name: "三弓床弩#1", X: 0, Y: -20, Heading: 0, Elevation: 35},
			{ID: "cb-2", Type: "bed_crossbow_triple", Name: "三弓床弩#2", X: -30, Y: -15, Heading: 15, Elevation: 35},
			{ID: "cb-3", Type: "bed_crossbow_triple", Name: "三弓床弩#3", X: 30, Y: -15, Heading: -15, Elevation: 35},
			{ID: "cb-4", Type: "bed_crossbow_single", Name: "单弓床弩#4", X: -15, Y: -40, Heading: 5, Elevation: 30},
			{ID: "cb-5", Type: "bed_crossbow_single", Name: "单弓床弩#5", X: 15, Y: -40, Heading: -5, Elevation: 30},
		}
		req.Crossbows = defaults
	}
	if req.Target.Radius <= 0 {
		req.Target.Radius = 20
	}

	resp := s.salvoOptimizer.Optimize(&req, s.dynamicsCfg.CrossbowTypes)
	c.JSON(200, resp)
}

func (s *Server) listAimTargets(c *gin.Context) {
	presets := s.vrCrossbow.ListAimTargets()
	c.JSON(200, gin.H{"targets": presets, "count": len(presets)})
}

func (s *Server) aimShoot(c *gin.Context) {
	var req models.AimShootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	resp, err := s.vrCrossbow.AimShoot(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, resp)
}

var _ = http.StatusOK
var _ = rand.Int63
