package handler

import (
	"encoding/json"
	"fgo-calc-backend/internal/config"
	"fgo-calc-backend/internal/model"
	"fgo-calc-backend/internal/repository"
	"fgo-calc-backend/internal/service"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"sync"
)

type Handler struct {
	dataMu   sync.RWMutex
	syncMu   sync.Mutex
	repo     *repository.Repository
	service  *service.CalculatorService
	cfg      *config.Config
	data     []byte
	dataETag string
}

func NewHandler(repo *repository.Repository, service *service.CalculatorService, cfg *config.Config) (*Handler, error) {
	h := &Handler{repo: repo, service: service, cfg: cfg}
	if err := h.refreshDataCache(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Handler) Register(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.File("./static/index.html")
	})

	static := r.Group("/static")
	static.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Next()
	})
	static.Static("/", "./static")

	api := r.Group("/api")
	{
		api.GET("/data", h.GetData)
		api.GET("/data-sync/status", h.DataSyncStatus)
		api.POST("/data-sync/update", h.DataSyncUpdate)
		api.POST("/filtertraits", h.FilterTraits)
		api.POST("/calculate", h.Calculate)
	}

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "给我玩FGO"})
	})
}

func (h *Handler) GetData(c *gin.Context) {
	h.dataMu.RLock()
	data := h.data
	etag := h.dataETag
	h.dataMu.RUnlock()
	c.Header("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

func (h *Handler) FilterTraits(c *gin.Context) {
	traits := mapStr2Int(c.PostFormArray("traits"))
	server := c.PostForm("server")
	if server == "" {
		server = "CN"
	}
	results := h.service.FilterServants(traits, []int{}, []int{}, server)
	c.JSON(http.StatusOK, results)
}

func (h *Handler) Calculate(c *gin.Context) {
	costLimit, _ := strconv.Atoi(c.PostForm("costlimit"))
	svtLimit, _ := strconv.Atoi(c.PostForm("svtlimit"))
	ceLimit, _ := strconv.Atoi(c.PostForm("celimit"))
	allowTraits := mapStr2Int(c.PostFormArray("allowtraits"))
	grandMode := c.PostForm("grandmode") == "true"

	if grandMode && len(allowTraits) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "冠位戴冠战模式必须选择对应职阶"})
		return
	}
	if grandMode && ceLimit > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "冠位戴冠战模式的普通自备礼装最多为5张；额外0 Cost礼装由求解器自动加入"})
		return
	}

	if !grandMode && ceLimit >= h.cfg.MaxCeLimit && len(allowTraits) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "戴冠战必须进行职阶筛选"})
		return
	}

	if ceLimit > h.cfg.MaxCeLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("礼装数量不能超过%d个", h.cfg.MaxCeLimit)})
		return
	}
	if svtLimit > h.cfg.MaxSvtLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("从者数量不能超过%d个", h.cfg.MaxSvtLimit)})
		return
	}
	if costLimit > h.cfg.MaxCost {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("总cost不能超过%d", h.cfg.MaxCost)})
		return
	}

	baseBond, _ := strconv.Atoi(c.PostForm("basebond"))
	supportLimit, _ := strconv.Atoi(c.PostForm("supportlimit"))
	if grandMode {
		// 冠位サポートは通常枠 + 報酬アップ追加枠の2枚を同時に持てる。
		supportLimit = 2
	}
	includeSupportCe := mapStr2Int(c.PostFormArray("includesupportce"))
	excludeSupportCe := mapStr2Int(c.PostFormArray("excludesupportce"))
	if supportLimit < 0 {
		supportLimit = 0
	}
	if supportLimit > h.cfg.MaxSupportLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("助战礼装数量不能超过%d个", h.cfg.MaxSupportLimit)})
		return
	}
	if len(includeSupportCe) > supportLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": "锁定助战礼装数量不能超过助战礼装数量"})
		return
	}

	server := c.PostForm("server")
	if server == "" {
		server = "CN"
	}
	enableEventBonus := c.PostForm("enable_event_bonus") == "true"
	selectedEvents := mapStr2Int(c.PostFormArray("selected_events"))
	bond10Svt := mapStr2Int(c.PostFormArray("bond10svt"))
	bond15Svt := mapStr2Int(c.PostFormArray("bond15svt"))
	optimizationMode := c.PostForm("optimizemode")
	if optimizationMode == "" {
		optimizationMode = "max"
	}
	var optimizationProfiles []model.ServantOptimizationProfile
	if raw := c.PostForm("bondprofiles"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &optimizationProfiles); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "羁绊培养数据格式无效"})
			return
		}
	}

	results, duration := h.service.OptimizeAdvancedWithProfile(
		costLimit,
		svtLimit,
		ceLimit,
		supportLimit,
		includeSupportCe,
		excludeSupportCe,
		allowTraits,
		mapStr2Int(c.PostFormArray("includesvt")),
		c.PostFormArray("includesvtdiff"),
		mapStr2Int(c.PostFormArray("excludesvt")),
		mapStr2Int(c.PostFormArray("includece")),
		mapStr2Int(c.PostFormArray("excludece")),
		baseBond,
		server,
		enableEventBonus,
		selectedEvents,
		grandMode,
		bond15Svt,
		bond10Svt,
		optimizationMode,
		optimizationProfiles,
	)

	c.JSON(http.StatusOK, gin.H{
		"teams":    results,
		"duration": duration.Milliseconds(),
	})
}

func mapStr2Int(data []string) []int {
	result := []int{}
	for _, d := range data {
		if v, err := strconv.Atoi(d); err == nil {
			result = append(result, v)
		}
	}
	return result
}
