package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const dataSyncTimeout = 15 * time.Minute

func (h *Handler) buildDataPayload() ([]byte, string, error) {
	meta := map[string]any{}
	metaPath := filepath.Join(h.cfg.DataDir, "source_meta.json")
	if raw, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(raw, &meta)
	}
	data, err := json.Marshal(gin.H{
		"servants":       h.repo.GetServants("JP"),
		"cnServants":     h.repo.GetCNOverrides(),
		"cnUnavailable":  h.repo.GetCNUnavailable(),
		"craftEssences":  h.repo.GetCraftEssences(),
		"traits":         h.repo.GetTraits(),
		"dataUpdatedAt":  h.repo.GetDataUpdatedAt(),
		"dataSourceMeta": meta,
	})
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(data)
	return data, fmt.Sprintf(`"%x"`, hash), nil
}

func (h *Handler) refreshDataCache() error {
	data, etag, err := h.buildDataPayload()
	if err != nil {
		return err
	}
	h.dataMu.Lock()
	h.data = data
	h.dataETag = etag
	h.dataMu.Unlock()
	return nil
}

func loopbackRequest(c *gin.Context) bool {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func projectRootFromDataDir(dataDir string) (string, error) {
	absData, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absData), nil
}

func pythonCommand(ctx context.Context, root string, checkOnly bool) (*exec.Cmd, error) {
	args := []string{"update_data.py"}
	if checkOnly {
		args = append(args, "--check")
	}
	args = append(args, "--json")

	if path, err := exec.LookPath("python"); err == nil {
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Dir = root
		return cmd, nil
	}
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("py"); err == nil {
			cmd := exec.CommandContext(ctx, path, append([]string{"-3"}, args...)...)
			cmd.Dir = root
			return cmd, nil
		}
	}
	if path, err := exec.LookPath("python3"); err == nil {
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Dir = root
		return cmd, nil
	}
	return nil, errors.New("Python was not found in PATH; Data Sync needs Python 3.10+ and Git")
}

func runDataSyncScript(root string, checkOnly bool) (map[string]any, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataSyncTimeout)
	defer cancel()
	cmd, err := pythonCommand(ctx, root, checkOnly)
	if err != nil {
		return nil, "", err
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		text := output.String()
		if ctx.Err() == context.DeadlineExceeded {
			return nil, text, errors.New("Data Sync timed out after 15 minutes")
		}
		var payload map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(text)), &payload) == nil {
			if msg, _ := payload["error"].(string); msg != "" {
				return payload, text, errors.New(msg)
			}
		}
		return nil, text, fmt.Errorf("Data Sync process failed: %w", err)
	}
	text := strings.TrimSpace(output.String())
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, text, fmt.Errorf("invalid Data Sync response: %w", err)
	}
	if ok, _ := payload["ok"].(bool); !ok {
		msg, _ := payload["error"].(string)
		if msg == "" {
			msg = "Data Sync failed"
		}
		return payload, text, errors.New(msg)
	}
	return payload, text, nil
}

func optionalDataSyncUnavailable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"python was not found in path",
		"python 3 was not found in path",
		"exit status 9009",
		"not recognized as an internal or external command",
		"the system cannot find the file specified",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func optionalDataSyncStatusPayload(reason string) gin.H {
	return gin.H{
		"ok":                    true,
		"source":                "Bundled release data",
		"connected":             false,
		"atlasAvailable":        false,
		"atlasError":            "",
		"updateAvailable":       false,
		"updateSignal":          "offline",
		"optionalSyncAvailable": false,
		"optionalSyncReason":    reason,
		"optionalSyncNeeds":     []string{"Python 3.10+", "Git"},
	}
}

func (h *Handler) DataSyncStatus(c *gin.Context) {
	if !loopbackRequest(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Data Sync is only available from localhost"})
		return
	}
	root, err := projectRootFromDataDir(h.cfg.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	payload, _, err := runDataSyncScript(root, true)
	if err != nil {
		if optionalDataSyncUnavailable(err) {
			c.JSON(http.StatusOK, optionalDataSyncStatusPayload(err.Error()))
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *Handler) DataSyncUpdate(c *gin.Context) {
	if !loopbackRequest(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Data Sync is only available from localhost"})
		return
	}
	if !h.syncMu.TryLock() {
		c.JSON(http.StatusConflict, gin.H{"error": "Data Sync is already running"})
		return
	}
	defer h.syncMu.Unlock()

	root, err := projectRootFromDataDir(h.cfg.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	payload, raw, err := runDataSyncScript(root, false)
	if err != nil {
		if len(raw) > 6000 {
			raw = raw[len(raw)-6000:]
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "detail": raw})
		return
	}
	if err := h.repo.ReloadData(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data files updated but live reload failed: " + err.Error()})
		return
	}
	if err := h.refreshDataCache(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data files updated but API cache refresh failed: " + err.Error()})
		return
	}
	payload["liveReloaded"] = true
	c.JSON(http.StatusOK, payload)
}
