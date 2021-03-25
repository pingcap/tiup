package backup

import (
	"fmt"
	"path"
	"time"

	"gorm.io/gorm"
)

const (
	// timeLayout = "2006-01-02 15:04:05"
	// timeLayout = "20060102150405"
	timeLayout     = "2006-0102-1504"
	statusNotStart = "not_start"
	statusRunning  = "running"
	statusSuccess  = "success"
	statusFail     = "fail"
)

type model struct {
	gorm.Model
	ClusterName string     `json:"cluster_name"`
	PlanTime    time.Time  `json:"plan_time"`
	StartTime   *time.Time `json:"start_time"`
	DayMinutes  int        `json:"day_minutes"`
	Folder      string     `json:"folder"`
	SubFolder   string     `json:"sub_folder"`
	Status      string     `json:"status"`  // statusNotStart, statusRunning, statusSuccess, statusFail
	Message     string     `json:"message"` // fail reason
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&model{})
}

func backupList(db *gorm.DB, clusterName string) []model {
	var items []model
	db.Where("cluster_name = ?", clusterName).Order("plan_time desc").Limit(10).Find(&items)
	return items
}

func disableAutoBackup(db *gorm.DB, clusterName string) {
	db.Where("cluster_name = ?", clusterName).Where("status = ?", statusNotStart).Delete(&model{})
}

func enableAutoBackup(db *gorm.DB, dayMinutes int, folder string, clusterName string) {
	disableAutoBackup(db, clusterName)

	// compute nex plan time
	targetHour := dayMinutes / 60
	targetMinute := dayMinutes % 60
	now := time.Now()
	planTime := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, time.Local)
	nowHour, nowMinute, _ := now.Clock()
	if (nowHour*60 + nowMinute) > dayMinutes {
		planTime = planTime.AddDate(0, 0, 1) // tommorrow
	}
	subFolderName := clusterName + "/" + planTime.Format(timeLayout)

	// create new record
	item := model{
		ClusterName: clusterName,
		PlanTime:    planTime,
		StartTime:   nil,
		DayMinutes:  dayMinutes,
		Folder:      folder,
		SubFolder:   subFolderName,
		Status:      statusNotStart,
	}
	db.Create(&item)
}

func checkBackup(db *gorm.DB) {
	// first, check whether has running backup
	// if does, return
	var item model
	ret := db.Where("status = ?", statusRunning).Find(&item)
	if ret.Error != nil {
		fmt.Println("meet error:", ret.Error)
		return
	}
	if item.Status != "" {
		fmt.Println("has running backup task:", item.ClusterName)
		return
	}
	ret = db.Where(`strftime("%s", plan_time) < ?`, time.Now().Unix()).Where("status = ?", statusNotStart).Find(&item)
	if ret.Error != nil {
		fmt.Println("meet error:", ret.Error)
		return
	}
	if item.Status == "" {
		fmt.Println("has no backup task")
		return
	}
	startBackup(db, item)
}

func startBackup(db *gorm.DB, item model) {
	// update status
	item.Status = statusRunning
	*item.StartTime = time.Now()
	db.Save(&item)

	// run
	fmt.Println("start to backup...")
	fullPath := path.Join(item.Folder, item.SubFolder)
	fmt.Println("backup folder:", fullPath)
	time.Sleep(time.Second * 60)
	fmt.Println("finish backup")

	// done
	item.Status = statusSuccess
	db.Save(&item)
}
