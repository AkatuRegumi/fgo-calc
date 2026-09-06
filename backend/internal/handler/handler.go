package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fgo-calc-backend/internal/config"
	"fgo-calc-backend/internal/repository"
	"fgo-calc-backend/internal/service"
	"fgo-calc-backend/internal/util"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	repo          *repository.Repository
	service       *service.CalculatorService
	cfg           *config.Config
	data          []byte
	dataETag      string
	announcements []byte
}

func NewHandler(repo *repository.Repository, service *service.CalculatorService, cfg *config.Config) (*Handler, error) {
	data, err := json.Marshal(gin.H{
		"servants":      repo.GetServants("JP"),
		"cnServants":    repo.GetCNOverrides(),
		"cnUnavailable": repo.GetCNUnavailable(),
		"craftEssences": repo.GetCraftEssences(),
		"traits":        repo.GetTraits(),
		"dataUpdatedAt": repo.GetDataUpdatedAt(),
	})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	announcements, err := json.Marshal(gin.H{"announcements": repo.GetAnnouncements()})
	if err != nil {
		return nil, err
	}
	return &Handler{
		repo:          repo,
		service:       service,
		cfg:           cfg,
		data:          data,
		dataETag:      fmt.Sprintf(`"%x"`, hash),
		announcements: announcements,
	}, nil
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
		api.GET("/announcements", h.GetAnnouncements)
		api.POST("/filtertraits", h.FilterTraits)
		api.POST("/calculate", h.Calculate)
		api.POST("/register", h.RegisterUser)
		api.POST("/login", h.LoginUser)
		api.POST("/logout", h.LogoutUser)

		auth := api.Group("/")
		auth.Use(h.AuthMiddleware())
		{
			auth.GET("/me", h.GetMe)
			auth.POST("/state", h.SaveState)
			auth.GET("/state", h.GetState)
			auth.POST("/history", h.AddHistory)
			auth.GET("/history", h.GetHistory)
			auth.POST("/history/:id/pin", h.SetHistoryPinned)
			auth.POST("/history/:id/name", h.RenameHistory)
		}
	}

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "给我玩FGO"})
	})
}

func (h *Handler) GetAnnouncements(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/json; charset=utf-8", h.announcements)
}

func (h *Handler) GetData(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
	c.Header("ETag", h.dataETag)
	if c.GetHeader("If-None-Match") == h.dataETag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", h.data)
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

	if ceLimit > h.cfg.MaxCeLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("礼装数量不能超过%d个", h.cfg.MaxCeLimit)})
		return
	}
	if svtLimit > h.cfg.MaxSvtLimit {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("从者数量不能超过%d个", h.cfg.MaxSvtLimit)})
		return
	}
	if costLimit > h.cfg.MaxCost {
		msgcost := h.cfg.MaxCost
		if len(allowTraits) == 0 {
			msgcost -= 12
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("总cost不能超过%d", msgcost)})
		return
	}

	baseBond, _ := strconv.Atoi(c.PostForm("basebond"))
	supportLimit, _ := strconv.Atoi(c.PostForm("supportlimit"))
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

	bond15Svt := mapStr2Int(c.PostFormArray("bond15svt"))
	bond15FullStr := c.PostFormArray("bond15full")
	// if len(bond15Svt) > h.cfg.MaxSvtLimit*20 {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "15绊从者数量过多"})
	// 	return
	// }
	bond15Full := make([]bool, len(bond15Svt))
	for i := range bond15Full {
		// 缺省按已满处理（国服当前15绊即为上限）
		bond15Full[i] = true
		if i < len(bond15FullStr) {
			bond15Full[i] = bond15FullStr[i] == "true"
		}
	}

	results, duration := h.service.Optimize(
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
		bond15Svt,
		bond15Full,
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

func (h *Handler) RegisterUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username cannot be empty"})
		return
	}
	if err := h.repo.RegisterUser(req.Username, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := util.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.SetCookie("token", token, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "success", "username": req.Username})
}

func (h *Handler) LoginUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	user, err := h.repo.LoginUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, err := util.GenerateToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.SetCookie("token", token, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "success", "state": user.State, "username": user.Username})
}

func (h *Handler) SaveState(c *gin.Context) {
	username := c.GetString("username")
	var req struct {
		State string `json:"state"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.repo.SaveUserState(username, req.State); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *Handler) GetState(c *gin.Context) {
	username := c.GetString("username")
	state, err := h.repo.GetUserState(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"state": state})
}

func (h *Handler) AddHistory(c *gin.Context) {
	username := c.GetString("username")
	var req struct {
		State  string `json:"state"`
		Result string `json:"result"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.repo.AddHistory(username, req.State, req.Result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *Handler) GetHistory(c *gin.Context) {
	username := c.GetString("username")
	history, err := h.repo.GetHistory(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func (h *Handler) SetHistoryPinned(c *gin.Context) {
	username := c.GetString("username")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid history id"})
		return
	}
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.repo.SetHistoryPinned(username, id, req.Pinned); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "pinned history limit reached" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *Handler) RenameHistory(c *gin.Context) {
	username := c.GetString("username")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid history id"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if len([]rune(name)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is too long"})
		return
	}
	if err := h.repo.RenameHistory(username, id, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		username, err := util.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set("username", username)
		c.Next()
	}
}

func (h *Handler) GetMe(c *gin.Context) {
	username := c.GetString("username")
	c.JSON(http.StatusOK, gin.H{"username": username})
}

func (h *Handler) LogoutUser(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
