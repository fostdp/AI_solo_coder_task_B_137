package api

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	ch "ballistics-system/clickhouse"
	"ballistics-system/config"
	"ballistics-system/models"

	ballistic_simulator "ballistics-system/ballistic_simulator"
	penetration_analyzer "ballistics-system/penetration_analyzer"
)

type Server struct {
	engine        *gin.Engine
	store         *ch.Store
	simEngine     *ballistic_simulator.Simulator
	penAnalyzer   *penetration_analyzer.Analyzer
	dynamicsCfg   *config.DynamicsConfig
	addr          string
}

func NewServer(addr string, store *ch.Store, simEngine *ballistic_simulator.Simulator, penAnalyzer *penetration_analyzer.Analyzer, dynamicsCfg *config.DynamicsConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	s := &Server{
		engine:      r,
		store:       store,
		simEngine:   simEngine,
		penAnalyzer: penAnalyzer,
		dynamicsCfg: dynamicsCfg,
		addr:        addr,
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
	crossbows := make([]map[string]interface{}, 0, len(s.dynamicsCfg.CrossbowTypes))
	for _, cfg := range s.dynamicsCfg.CrossbowTypes {
		crossbows = append(crossbows, map[string]interface{}{
			"type":           cfg.Type,
			"name":           cfg.Name,
			"description":    cfg.Description,
			"era":            cfg.Era,
			"draw_force_n":   cfg.DrawForce,
			"draw_length_m":  cfg.DrawLength,
			"arrow_mass_kg":  cfg.ArrowMass,
			"arrow_length_m": cfg.ArrowLength,
			"arrow_dia_mm":   cfg.ArrowDiameter * 1000,
			"typical_v_ms":   cfg.TypicalVelocity,
			"typical_range_m": cfg.TypicalRange,
			"spin_rate_hz":   cfg.SpinRate,
			"bow_efficiency": cfg.BowEfficiency,
			"crew_size":      cfg.CrewSize,
			"reload_seconds": cfg.ReloadSeconds,
		})
	}
	sort.Slice(crossbows, func(i, j int) bool {
		return crossbows[i]["draw_force_n"].(float64) < crossbows[j]["draw_force_n"].(float64)
	})
	c.JSON(200, gin.H{"crossbows": crossbows, "count": len(crossbows)})
}

func (s *Server) listModernWeapons(c *gin.Context) {
	weapons := make([]map[string]interface{}, 0, len(s.dynamicsCfg.ModernWeapons))
	for _, cfg := range s.dynamicsCfg.ModernWeapons {
		weapons = append(weapons, map[string]interface{}{
			"type":              cfg.Type,
			"name":              cfg.Name,
			"description":       cfg.Description,
			"era":               cfg.Era,
			"bullet_mass_kg":    cfg.BulletMass,
			"bullet_dia_mm":     cfg.BulletDiameter * 1000,
			"bullet_length_mm":  cfg.BulletLength * 1000,
			"muzzle_velocity_ms": cfg.MuzzleVelocity,
			"max_range_m":       cfg.MaxRange,
			"effective_range_m": cfg.EffectiveRange,
			"drag_coef":         cfg.DragCoefficient,
			"spin_rate_hz":      cfg.SpinRate,
			"hardness_bhn":      cfg.Hardness,
			"tip_area_m2":       cfg.TipArea,
			"crew_size":         cfg.CrewSize,
			"reload_seconds":    cfg.ReloadSeconds,
		})
	}
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
	if req.LaunchAngle == 0 {
		req.LaunchAngle = 45.0
	}
	if req.ArrowHeadType == "" {
		req.ArrowHeadType = "bodkin"
	}

	resultList := make([]models.CrossbowComparisonItem, 0)
	armorTypes := s.penAnalyzer.ArmorTypeKeys()

	var maxKE float64 = 0

	for key, cfg := range s.dynamicsCfg.CrossbowTypes {
		if len(req.CrossbowTypes) > 0 {
			found := false
			for _, t := range req.CrossbowTypes {
				if t == key {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		simResult := s.simEngine.SimulateWithCrossbowType(key, &cfg, req.LaunchAngle, 0.0)
		pens := make(map[string]float64)
		penSuccess := make(map[string]bool)
		for _, armorKey := range s.penAnalyzer.ArmorTypeKeys() {
			pen := s.penAnalyzer.AnalyzeWithSpin(
				simResult.ImpactVelocity,
				cfg.ArrowMass,
				cfg.ArrowDiameter,
				cfg.ArrowLength,
				simResult.ImpactSpinRate,
				armorKey,
				req.ArrowHeadType,
				0,
			)
			pens[armorKey] = pen.PenetrationDepth * 1000
			penSuccess[armorKey] = pen.Success
		}

		shotsPerMin := 60.0 / cfg.ReloadSeconds
		kePerMin := simResult.KineticEnergy * shotsPerMin
		item := models.CrossbowComparisonItem{
			CrossbowType:      cfg.Type,
			CrossbowName:      cfg.Name,
			Description:       cfg.Description,
			Era:               cfg.Era,
			DrawForce:         cfg.DrawForce,
			DrawLength:        cfg.DrawLength,
			ArrowMass:         cfg.ArrowMass,
			ArrowDiameter:     cfg.ArrowDiameter,
			ArrowLength:       cfg.ArrowLength,
			SpinRate:          cfg.SpinRate,
			BowEfficiency:     cfg.BowEfficiency,
			CrewSize:          cfg.CrewSize,
			ReloadSeconds:     cfg.ReloadSeconds,
			InitialVelocity:   simResult.InitialVelocity,
			Range:             simResult.Range,
			FlightTime:        simResult.FlightTime,
			MaxHeight:         simResult.MaxHeight,
			ImpactVelocity:    simResult.ImpactVelocity,
			KineticEnergy:     simResult.KineticEnergy,
			ImpactSpinRate:    simResult.ImpactSpinRate,
			ImpactGyroStab:    simResult.ImpactGyroStab,
			Penetrations:      pens,
			PenetrationSuccess: penSuccess,
			ShotsPerMinute:    shotsPerMin,
			KEPerMinute:       kePerMin,
		}
		if item.KineticEnergy > maxKE {
			maxKE = item.KineticEnergy
		}
		resultList = append(resultList, item)
	}

	for i := range resultList {
		if maxKE > 0 {
			resultList[i].PowerIndex = (resultList[i].KineticEnergy / maxKE) * (resultList[i].Range / 1500.0) * 100
		}
	}

	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].PowerIndex > resultList[j].PowerIndex
	})

	c.JSON(200, models.CrossbowComparisonResponse{
		Crossbows:  resultList,
		ArmorTypes: armorTypes,
	})
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
	if req.CompareRange == 0 {
		req.CompareRange = 1000.0
	}
	if req.ArrowHeadType == "" {
		req.ArrowHeadType = "bodkin"
	}

	resultList := make([]models.WeaponEraComparison, 0)
	armorTypes := s.penAnalyzer.ArmorTypeKeys()

	var bedCrossbowKE float64 = 0

	for key, cfg := range s.dynamicsCfg.CrossbowTypes {
		if len(req.CrossbowTypes) > 0 {
			found := false
			for _, t := range req.CrossbowTypes {
				if t == key {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		elev, simResult := s.simEngine.SolveElevationForDistance(
			req.CompareRange, cfg.TypicalVelocity,
			cfg.ArrowMass, cfg.ArrowDiameter, cfg.ArrowLength, cfg.SpinRate,
		)
		_ = elev

		pens := make(map[string]float64)
		penSuccess := make(map[string]bool)
		for _, armorKey := range s.penAnalyzer.ArmorTypeKeys() {
			pen := s.penAnalyzer.AnalyzeWithSpin(
				simResult.ImpactVelocity,
				cfg.ArrowMass,
				cfg.ArrowDiameter,
				cfg.ArrowLength,
				simResult.ImpactSpinRate,
				armorKey,
				req.ArrowHeadType,
				0,
			)
			pens[armorKey] = pen.PenetrationDepth * 1000
			penSuccess[armorKey] = pen.Success
		}

		muzzleKE := 0.5 * cfg.ArrowMass * cfg.TypicalVelocity * cfg.TypicalVelocity
		shotsPerMin := 60.0 / cfg.ReloadSeconds

		item := models.WeaponEraComparison{
			WeaponType:       cfg.Type,
			WeaponName:       cfg.Name,
			Description:      cfg.Description,
			Era:              cfg.Era,
			IsModern:         false,
			ProjectileMass:   cfg.ArrowMass,
			ProjectileDia:    cfg.ArrowDiameter,
			ProjectileLen:    cfg.ArrowLength,
			MuzzleVelocity:   cfg.TypicalVelocity,
			MaxRange:         cfg.TypicalRange,
			EffectiveRange:   cfg.TypicalRange * 0.7,
			SpinRate:         cfg.SpinRate,
			KineticEnergy:    muzzleKE,
			ImpactVelocity:   simResult.ImpactVelocity,
			ImpactKE:         simResult.KineticEnergy,
			Penetrations:     pens,
			PenetrationSuccess: penSuccess,
			CrewSize:         cfg.CrewSize,
			ReloadSeconds:    cfg.ReloadSeconds,
			ShotsPerMinute:   shotsPerMin,
			KEPerMinute:      muzzleKE * shotsPerMin,
		}
		if key == "bed_crossbow_triple" {
			bedCrossbowKE = simResult.KineticEnergy
		}
		resultList = append(resultList, item)
	}

	for key, cfg := range s.dynamicsCfg.ModernWeapons {
		if len(req.ModernTypes) > 0 {
			found := false
			for _, t := range req.ModernTypes {
				if t == key {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		g := s.dynamicsCfg.Simulation.Gravity
		v0 := cfg.MuzzleVelocity
		crossArea := math.Pi * math.Pow(cfg.BulletDiameter/2.0, 2)
		dragFactor := 0.5 * cfg.DragCoefficient * s.dynamicsCfg.Simulation.AirDensitySea * crossArea / cfg.BulletMass

		vx := v0
		vy := 0.0
		x := 0.0
		estImpactVel := v0 * 0.5
		for t := 0.0; t < 10.0; t += 0.001 {
			v := math.Sqrt(vx*vx + vy*vy)
			if x >= req.CompareRange {
				estImpactVel = v
				break
			}
			ax := -dragFactor * v * vx
			ay := -g - dragFactor*v*vy
			vx += ax * 0.001
			vy += ay * 0.001
			x += vx * 0.001
		}

		pens := make(map[string]float64)
		penSuccess := make(map[string]bool)
		for _, armorKey := range s.penAnalyzer.ArmorTypeKeys() {
			pen := s.penAnalyzer.AnalyzeModernBullet(estImpactVel, &cfg, armorKey, 0)
			pens[armorKey] = pen.PenetrationDepth * 1000
			penSuccess[armorKey] = pen.Success
		}

		muzzleKE := 0.5 * cfg.BulletMass * cfg.MuzzleVelocity * cfg.MuzzleVelocity
		impactKE := 0.5 * cfg.BulletMass * estImpactVel * estImpactVel
		shotsPerMin := 60.0 / cfg.ReloadSeconds

		item := models.WeaponEraComparison{
			WeaponType:       cfg.Type,
			WeaponName:       cfg.Name,
			Description:      cfg.Description,
			Era:              cfg.Era,
			IsModern:         true,
			ProjectileMass:   cfg.BulletMass,
			ProjectileDia:    cfg.BulletDiameter,
			ProjectileLen:    cfg.BulletLength,
			MuzzleVelocity:   cfg.MuzzleVelocity,
			MaxRange:         cfg.MaxRange,
			EffectiveRange:   cfg.EffectiveRange,
			SpinRate:         cfg.SpinRate,
			KineticEnergy:    muzzleKE,
			ImpactVelocity:   estImpactVel,
			ImpactKE:         impactKE,
			Penetrations:     pens,
			PenetrationSuccess: penSuccess,
			CrewSize:         cfg.CrewSize,
			ReloadSeconds:    cfg.ReloadSeconds,
			ShotsPerMinute:   shotsPerMin,
			KEPerMinute:      muzzleKE * shotsPerMin,
		}
		resultList = append(resultList, item)
	}

	if bedCrossbowKE > 0 {
		for i := range resultList {
			resultList[i].PowerRatio = resultList[i].ImpactKE / bedCrossbowKE
		}
	}

	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].ImpactKE > resultList[j].ImpactKE
	})

	c.JSON(200, models.EraComparisonResponse{
		Weapons:    resultList,
		ArmorTypes: armorTypes,
	})
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

	resp := s.simEngine.OptimizeBarrage(&req, s.dynamicsCfg.CrossbowTypes)
	c.JSON(200, resp)
}

func (s *Server) listAimTargets(c *gin.Context) {
	presets := []models.AimTargetPreset{
		{ID: "training", Name: "训练靶 (50m)", Distance: 50, Height: 1.5, ArmorType: "leather", Difficulty: "easy", Points: 10, Icon: "🎯"},
		{ID: "soldier", Name: "敌军步兵 (200m)", Distance: 200, Height: 1.7, ArmorType: "lamellar", Difficulty: "medium", Points: 50, Icon: "🛡️"},
		{ID: "rider", Name: "敌方骑兵 (350m)", Distance: 350, Height: 2.0, ArmorType: "mail", Difficulty: "hard", Points: 100, Icon: "🐴"},
		{ID: "gate", Name: "城门木盾 (500m)", Distance: 500, Height: 3.0, ArmorType: "leather", Difficulty: "hard", Points: 150, Icon: "🚪"},
		{ID: "tower", Name: "瞭望塔守卫 (650m)", Distance: 650, Height: 8.0, ArmorType: "lamellar", Difficulty: "expert", Points: 250, Icon: "🏰"},
		{ID: "commander", Name: "敌将 (800m)", Distance: 800, Height: 1.7, ArmorType: "plate", Difficulty: "legendary", Points: 500, Icon: "👑"},
	}
	c.JSON(200, gin.H{"targets": presets, "count": len(presets)})
}

func (s *Server) aimShoot(c *gin.Context) {
	var req models.AimShootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.CrossbowType == "" {
		req.CrossbowType = "bed_crossbow_triple"
	}
	if req.ArrowType == "" {
		req.ArrowType = "bodkin"
	}
	if req.Target.Distance <= 0 {
		req.Target.Distance = 200
	}

	cfg, ok := s.dynamicsCfg.CrossbowTypes[req.CrossbowType]
	if !ok {
		cfg = s.dynamicsCfg.CrossbowTypes["bed_crossbow_triple"]
	}

	requiredElev, requiredAzimuth, simResult := s.simEngine.SolveElevationWithWind(
		req.Target.Distance,
		req.Target.Height,
		cfg.TypicalVelocity,
		cfg.ArrowMass,
		cfg.ArrowDiameter,
		cfg.ArrowLength,
		cfg.SpinRate,
		req.WindSpeed,
		req.WindDir,
	)

	rangeError := math.Abs(simResult.Range - req.Target.Distance)
	hit := rangeError < 3.0 && math.Abs(simResult.MaxHeight-req.Target.Height) < 5.0

	armorType := req.Target.ArmorType
	if armorType == "" {
		armorType = "leather"
	}
	penResult := s.penAnalyzer.AnalyzeWithSpin(
		simResult.ImpactVelocity,
		cfg.ArrowMass,
		cfg.ArrowDiameter,
		cfg.ArrowLength,
		simResult.ImpactSpinRate,
		armorType,
		req.ArrowType,
		0,
	)

	score := 0
	message := ""
	if hit && penResult.Success {
		score = 100
		message = "完美命中并穿透！"
		if rangeError < 0.5 {
			score += 50
			message = "直击靶心！完全穿透！"
		}
	} else if hit && !penResult.Success {
		score = 50
		message = "命中目标，但未能穿透铠甲"
	} else if rangeError < 10 {
		score = 20
		message = "接近目标，但未命中"
	} else {
		message = "未命中目标"
	}

	aziRad := requiredAzimuth * math.Pi / 180.0
	impactX := simResult.Range * math.Cos(aziRad)
	impactY := simResult.Range * math.Sin(aziRad)

	resp := &models.AimShootResponse{
		Success:           true,
		Hit:               hit,
		RequiredElevation: requiredElev,
		RequiredAzimuth:   requiredAzimuth,
		ActualRange:       simResult.Range,
		FlightTime:        simResult.FlightTime,
		MaxHeight:         simResult.MaxHeight,
		ImpactVelocity:    simResult.ImpactVelocity,
		KineticEnergy:     simResult.KineticEnergy,
		ImpactX:           impactX,
		ImpactY:           impactY,
		WindDriftX:        impactX - req.Target.Distance,
		WindDriftY:        impactY,
		PenetrationDepth:  penResult.PenetrationDepth * 1000,
		PenetrationSuccess: penResult.Success,
		Trajectory:        simResult.Trajectory,
		Message:           message,
		Score:             score,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if s.store != nil {
		simResult.DeviceID = "aim-game-" + req.CrossbowType
		simResult.ArmorType = armorType
		simResult.PenetrationDepth = penResult.PenetrationDepth
		simResult.PenetrationSuccess = penResult.Success
		_ = s.store.InsertSimulationResult(ctx, simResult)
	}

	c.JSON(200, resp)
}

var _ = http.StatusOK
