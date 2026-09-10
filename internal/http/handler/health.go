package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/version"
)

// HealthHandler answers the container health check and the deploy script's
// "did the new build actually take over?" question. It deliberately touches no
// service and no database: it reports that this process is up and which commit
// it was built from, nothing else.
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

type healthData struct {
	Status     string `json:"status"`
	Commit     string `json:"commit"`
	BuildTime  string `json:"build_time,omitempty"`
	APIVersion string `json:"api_version"`
}

func (h *HealthHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, healthData{
		Status:     "ok",
		Commit:     version.Commit,
		BuildTime:  version.BuildTime,
		APIVersion: "v1",
	})
}
