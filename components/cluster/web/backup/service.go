package backup

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type service struct {
	db *gorm.DB
}

// NewService creates new backup service
func NewService(db *gorm.DB) *service {
	err := autoMigrate(db)
	if err != nil {
		panic("failed to init database")
	}

	return &service{db}
}

// RegisterRouter registers routers for backup
func RegisterRouter(r *gin.RouterGroup, s *service) {
	endpoint := r.Group("/backup")
	{
		endpoint.GET("/:clusterName/backups", s.getBackupList)
		endpoint.POST("/:clusterName/setting", s.updateBackupSetting)
	}
}

func (s *service) getBackupList(c *gin.Context) {
	clusterName := c.Param("clusterName")
	models := backupList(s.db, clusterName)
	c.JSON(http.StatusOK, models)
}

type settingReq struct {
	Enable     bool   `json:"enable"`
	Folder     string `json:"folder"`
	DayMinutes int    `json:"day_minutes"`
}

func (s *service) updateBackupSetting(c *gin.Context) {
	clusterName := c.Param("clusterName")

	var req settingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	if req.Enable {
		enableAutoBackup(s.db, req.DayMinutes, req.Folder, clusterName)
	} else {
		disableAutoBackup(s.db, clusterName)
	}
	c.Status(http.StatusNoContent)
}

/////
// StartTicker run an interval task to check whether should backup
func (s *service) StartTicker() {
	ticker := time.NewTicker(time.Second * 10) // 60
	go func() {
		for t := range ticker.C {
			fmt.Println("Tick at", t)
			checkBackup(s.db)
		}
	}()
}
