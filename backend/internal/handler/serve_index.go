package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
)

const positionUIScript = `<script src="/static/position-ui.js?v=position-aware-v2"></script>`

func (h *Handler) ServeIndex(c *gin.Context) {
	raw, err := os.ReadFile("./static/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load local UI")
		return
	}
	html := string(raw)
	if !strings.Contains(html, positionUIScript) {
		html = strings.Replace(html, "</body>", "    "+positionUIScript+"\n</body>", 1)
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
