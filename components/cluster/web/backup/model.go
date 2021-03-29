package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
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

func deleteBackup(db *gorm.DB, id string) {
	// db.Delete(&model{}, id)

	var item model
	db.First(&item, id)
	if item.ID > 0 {
		db.Delete(&item)
		// remove folders
		_ = os.RemoveAll(path.Join(item.Folder, item.SubFolder))
	}
}

func disableAutoBackup(db *gorm.DB, clusterName string) {
	db.Where("cluster_name = ?", clusterName).Where("status = ?", statusNotStart).Delete(&model{})
}

func enableAutoBackup(db *gorm.DB, dayMinutes int, folder string, clusterName string) {
	disableAutoBackup(db, clusterName)
	createNextBackupRecord(db, dayMinutes, folder, clusterName, false)
}

func createNextBackupRecord(db *gorm.DB, dayMinutes int, folder string, clusterName string, forceNextDay bool) {
	// compute nex plan time
	targetHour := dayMinutes / 60
	targetMinute := dayMinutes % 60
	now := time.Now()
	planTime := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, time.Local)
	nowHour, nowMinute, _ := now.Clock()
	if (nowHour*60+nowMinute) > dayMinutes || forceNextDay {
		planTime = planTime.AddDate(0, 0, 1) // tomorrow
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

func checkBackup(db *gorm.DB, cm *manager.Manager, gOpt operator.Options) {
	var items []model
	// ret := db.Where(`strftime("%s", plan_time) < ?`, time.Now().Unix()).Where("status = ?", statusNotStart).Find(&items)
	ret := db.Where("status = ?", statusNotStart).Order("plan_time desc").Find(&items)
	if ret.Error != nil {
		fmt.Println("meet error:", ret.Error)
		return
	}
	now := time.Now()
	filteredItems := make([]model, 0)
	for _, v := range items {
		if now.Sub(v.PlanTime) > 0 {
			filteredItems = append(filteredItems, v)
		}
	}
	if len(filteredItems) == 0 {
		fmt.Println("has no backup task")
		return
	}
	for _, v := range filteredItems {
		go startBackup(db, v, cm, gOpt)
	}
}

func startBackup(db *gorm.DB, item model, cm *manager.Manager, gOpt operator.Options) {
	// update status
	item.Status = statusRunning
	item.StartTime = new(time.Time)
	*item.StartTime = time.Now()
	db.Save(&item)

	// create next backup plan
	createNextBackupRecord(db, item.DayMinutes, item.Folder, item.ClusterName, true)

	// run
	fmt.Println("start to backup...")
	fullPath := path.Join(item.Folder, item.SubFolder)
	fmt.Println("backup folder:", fullPath)
	time.Sleep(time.Second * 10)

	// get pd
	instInfos, err := cm.GetClusterTopology(item.ClusterName, gOpt)
	if err != nil {
		fmt.Println("back failed:", err)
		item.Status = statusFail
		item.Message = err.Error()
		db.Save(&item)
		return
	}
	var availabePD *manager.InstInfo = nil
	for _, v := range instInfos {
		if v.Role == "pd" && strings.HasPrefix(v.Status, "Up") {
			availabePD = &v
			break
		}
	}
	if availabePD == nil {
		fmt.Println("back failed:", "has no availabel PD node")
		item.Status = statusFail
		item.Message = "has no avaialbe PD node"
		db.Save(&item)
		return
	}

	logFileName := strings.ReplaceAll(item.SubFolder, "/", "_")
	fullCmd := fmt.Sprintf(`tiup br backup full --pd "%s" -s "%s" --log-file "%s.log"`, availabePD.ID, fullPath, logFileName)
	// tiup br backup full --pd "10.0.1.21:2379" -s "/var/nfs/multi-host/2021-0326-1022" --log-file "multi-host_2021-0326-1022.log"
	fmt.Println("full cmd:", fullCmd)

	// run command by os/exec
	out, err := exec.Command("tiup", "br", "backup", "full", "--pd", availabePD.ID, "-s", fullPath, "--log-file", logFileName+".log").Output()
	if err != nil {
		fmt.Println("back failed:", err)
		item.Status = statusFail
		item.Message = err.Error() + string(out)
		db.Save(&item)
		return
	}
	fmt.Println("finish backup")

	// done
	item.Status = statusSuccess
	db.Save(&item)
}
