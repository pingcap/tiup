package backup

import (
	"net/http"

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
		endpoint.GET("/:clusterName/next_backup", s.getNextBackup)
		endpoint.GET("/:clusterName/backups", s.getBackupList)
	}
}

func (s *service) getNextBackup(c *gin.Context) {
	model := nextBackup(s.db)
	c.JSON(http.StatusOK, gin.H{
		"enable_backup": model != nil,
		"next":          model,
	})
}

func (s *service) getBackupList(c *gin.Context) {
	models := backupList(s.db)
	c.JSON(http.StatusOK, models)
}
