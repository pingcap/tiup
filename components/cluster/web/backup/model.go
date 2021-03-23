package backup

import (
	"time"

	"gorm.io/gorm"
)

// const (
// 	// timeLayout = "2006-01-02 15:04:05"
// 	timeLayout = "20060102150405"
// )

type model struct {
	gorm.Model
	PlanTime  time.Time  `json:"plan_time"`
	StartTime *time.Time `json:"start_time"`
	Folder    string     `json:"folder"`
	SubFolder string     `json:"sub_folder"`
	Status    string     `json:"status"`  // not_start, running, success, fail
	Message   string     `json:"message"` // fail reason
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&model{})
}

// func enableAutoBackup(db *gorm.DB, planTime time.Time, folder string) error {
// 	// delete all not_start
// 	db.Where("status = ?", "not_start").Delete(model{})

// 	subFolderName := planTime.Format(timeLayout)

// 	// create new record
// 	item := model{
// 		PlanTime:  planTime,
// 		StartTime: nil,
// 		Folder:    folder,
// 		SubFolder: subFolderName,
// 		Status:    "not_start",
// 	}
// 	return db.Create(&item).Error
// }

// func disableAutoBackup(db *gorm.DB) {
// 	db.Where("status = ?", "not_start").Delete(model{})
// }

func nextBackup(db *gorm.DB) *model {
	var item *model
	db.Where("status = ?", "not_start").Where("PlanTime >= ?", time.Now()).Find(item)
	return item
}

func backupList(db *gorm.DB) []model {
	var items []model
	db.Where("status != ?", "not_start").Find(&items)
	return items
}
