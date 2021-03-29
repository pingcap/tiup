package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pingcap/tiup/cmd"
	"github.com/pingcap/tiup/components/cluster/web/backup"
	"github.com/pingcap/tiup/components/cluster/web/uiserver"

	"github.com/pingcap/tiup/pkg/cluster/audit"
	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/report"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/repository"
	cors "github.com/rs/cors/wrapper/gin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	tidbSpec    *spec.SpecManager
	cm          *manager.Manager
	gOpt        operator.Options
	skipConfirm = true
)

// MWHandleErrors creates a middleware that turns (last) error in the context into an APIError json response.
// In handlers, `c.Error(err)` can be used to attach the error to the context.
// When error is attached in the context:
// - The handler can optionally assign the HTTP status code.
// - The handler must not self-generate a response body.
func MWHandleErrors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		err := c.Errors.Last()
		if err == nil {
			return
		}

		statusCode := c.Writer.Status()
		if statusCode == http.StatusOK {
			statusCode = http.StatusInternalServerError
		}

		c.AbortWithStatusJSON(statusCode, gin.H{
			"message": err.Error(),
		})
	}
}

// Run starts web ui for cluster
func Run(_tidbSpec *spec.SpecManager, _manager *manager.Manager, _gOpt operator.Options) {
	tidbSpec = _tidbSpec
	cm = _manager
	gOpt = _gOpt

	// sqlite db
	db, err := gorm.Open(sqlite.Open("tiup-ui.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	backupSrv := backup.NewService(db, cm, gOpt)
	backupSrv.StartTicker()

	router := gin.Default()
	router.Use(cors.AllowAll())
	router.Use(MWHandleErrors())
	// Backend API
	api := router.Group("/api")
	backup.RegisterRouter(api, backupSrv)
	{
		api.GET("/status", statusHandler)

		api.POST("/deploy", deployHandler)
		api.GET("/clusters", clustersHandler)
		api.GET("/clusters/:clusterName", clusterHandler)
		api.DELETE("/clusters/:clusterName", destroyClusterHandler)
		api.POST("/clusters/:clusterName/start", startClusterHandler)
		api.POST("/clusters/:clusterName/stop", stopClusterHandler)
		api.POST("/clusters/:clusterName/scale_in", scaleInClusterHandler)
		api.POST("/clusters/:clusterName/scale_out", scaleOutClusterHandler)
		api.POST("/clusters/:clusterName/check", checkClusterHandler)
		api.GET("/clusters/:clusterName/check_result", checkResultHandler)
		api.POST("/clusters/:clusterName/upgrade", upgradeClusterHandler)
		api.POST("/clusters/:clusterName/downgrade", downgradeClusterHandler)

		api.GET("/mirror", mirrorHandler)
		api.POST("/mirror", setMirrorHandler)

		api.GET("/tidb_versions", tidbVersionsHandler)

		api.GET("/audit", auditLogListHandler)
	}
	// Frontend assets
	router.StaticFS("/tiup", uiserver.Assets)
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tiup")
		c.Abort()
	})

	test()

	_ = router.Run()
}

// GlobalLoginOptions represents the global options for deploy
type GlobalLoginOptions struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	PrivateKey         string `json:"privateKey"`         // TODO: refine naming style
	PrivateKeyPassword string `json:"privateKeyPassword"` // TODO: refine naming style
}

// DeployReq represents for the request of deploy API
type DeployReq struct {
	ClusterName        string             `json:"cluster_name"`
	TiDBVersion        string             `json:"tidb_version"`
	TopoYaml           string             `json:"topo_yaml"`
	GlobalLoginOptions GlobalLoginOptions `json:"global_login_options"`
}

func statusHandler(c *gin.Context) {
	status := cm.GetOperationStatus()
	c.JSON(http.StatusOK, status)
}

func deployHandler(c *gin.Context) {
	var req DeployReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	// create temp topo yaml file
	// TODO: move yaml to ~/.tiup folder
	tmpfile, err := ioutil.TempFile("", "topo")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.TopoYaml))
	topoFilePath := tmpfile.Name()
	tmpfile.Close()

	opt := manager.DeployOptions{
		User:     req.GlobalLoginOptions.Username,
		NoLabels: true,
	}
	if req.GlobalLoginOptions.Password != "" {
		opt.UsePassword = true
		opt.Pass = &req.GlobalLoginOptions.Password
	} else {
		// create private key file
		tmpfile, err = ioutil.TempFile("", "private_key")
		if err != nil {
			_ = c.Error(err)
			return
		}
		_, _ = tmpfile.WriteString(strings.TrimSpace(req.GlobalLoginOptions.PrivateKey))
		identifyFile := tmpfile.Name()
		tmpfile.Close()

		opt.UsePassword = false
		opt.IdentityFile = identifyFile
		opt.Pass = &req.GlobalLoginOptions.PrivateKeyPassword
	}

	// audit
	// logger.AddCustomAuditLog(fmt.Sprintf("[web] deploy %s %s %s", req.ClusterName, req.TiDBVersion, topoFilePath))
	logger.AddCustomAuditLog(fmt.Sprintf("[web] deploy %s %s", req.ClusterName, req.TiDBVersion))

	go func() {
		cm.DoDeploy(
			req.ClusterName,
			req.TiDBVersion,
			topoFilePath,
			opt,
			postDeployHook,
			skipConfirm,
			gOpt,
		)
	}()

	c.Status(http.StatusNoContent)
}

func clustersHandler(c *gin.Context) {
	clusters, err := cm.GetClusterList()
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, clusters)
}

func clusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	instInfos, err := cm.GetClusterTopology(clusterName, gOpt)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, instInfos)
}

func destroyClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] destroy %s", clusterName))

	go func() {
		cm.DoDestroyCluster(clusterName, gOpt, operator.Options{}, skipConfirm)
	}()

	c.Status(http.StatusNoContent)
}

func startClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] start %s", clusterName))

	go func() {
		cm.DoStartCluster(clusterName, gOpt, func(b *task.Builder, metadata spec.Metadata) {
			b.UpdateTopology(
				clusterName,
				tidbSpec.Path(clusterName),
				metadata.(*spec.ClusterMeta),
				nil, /* deleteNodeIds */
			)
		})
	}()

	c.Status(http.StatusNoContent)
}

func stopClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] stop %s", clusterName))

	go func() {
		cm.DoStopCluster(clusterName, gOpt)
	}()

	c.Status(http.StatusNoContent)
}

func checkClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	checkType := c.Query("type")
	if checkType == "" {
		checkType = "upgrade"
	}
	if checkType != "upgrade" && checkType != "downgrade" {
		_ = c.Error(errors.New("request parameters are not correct"))
		return
	}

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] check %s", clusterName))

	go func() {
		opt := manager.CheckOptions{
			Opr:          &operator.CheckOptions{},
			ExistCluster: true,
		}
		cm.DoCheckCluster(clusterName, checkType == "upgrade", opt, gOpt)
	}()

	c.Status(http.StatusNoContent)
}

func checkResultHandler(c *gin.Context) {
	extra := cm.GetOperationExtra()
	if extra == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "checking",
		})
	} else {
		results := extra.([]manager.HostCheckResult)
		c.JSON(http.StatusOK, results)
	}
}

// ScaleInReq represents the request for scale in cluster
type ScaleInReq struct {
	Nodes []string `json:"nodes"`
	Force bool     `json:"force"`
}

func scaleInClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	var req ScaleInReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] scale_in %s -N %s", clusterName, strings.Join(req.Nodes, ",")))

	go func() {
		opt := operator.Options{
			SSHTimeout: gOpt.SSHTimeout,
			OptTimeout: gOpt.OptTimeout,
			APITimeout: gOpt.APITimeout,
			NativeSSH:  gOpt.NativeSSH,
			SSHType:    gOpt.SSHType,
			Force:      req.Force,
			Nodes:      req.Nodes}

		// TODO, duplicated
		scale := func(b *task.Builder, imetadata spec.Metadata, tlsCfg *tls.Config) {
			metadata := imetadata.(*spec.ClusterMeta)

			nodes := opt.Nodes
			if !opt.Force {
				nodes = operator.AsyncNodes(metadata.Topology, nodes, false)
			}

			b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, opt, tlsCfg).
				UpdateMeta(clusterName, metadata, nodes).
				UpdateTopology(clusterName, tidbSpec.Path(clusterName), metadata, nodes)
		}

		cm.DoScaleIn(
			clusterName,
			skipConfirm,
			opt,
			scale,
		)
	}()

	c.Status(http.StatusNoContent)
}

// ScaleOutReq represents the request for scale out
type ScaleOutReq struct {
	TopoYaml           string             `json:"topo_yaml"`
	GlobalLoginOptions GlobalLoginOptions `json:"global_login_options"`
}

func scaleOutClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	var req ScaleOutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	// create temp topo yaml file
	// TODO: move yaml to ~/.tiup folder
	tmpfile, err := ioutil.TempFile("", "topo")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.TopoYaml))
	topoFilePath := tmpfile.Name()
	tmpfile.Close()

	opt := manager.ScaleOutOptions{
		User:     req.GlobalLoginOptions.Username,
		NoLabels: true,
	}
	if req.GlobalLoginOptions.Password != "" {
		opt.UsePassword = true
		opt.Pass = &req.GlobalLoginOptions.Password
	} else {
		// create private key file
		tmpfile, err = ioutil.TempFile("", "private_key")
		if err != nil {
			_ = c.Error(err)
			return
		}
		_, _ = tmpfile.WriteString(strings.TrimSpace(req.GlobalLoginOptions.PrivateKey))
		identifyFile := tmpfile.Name()
		tmpfile.Close()

		opt.UsePassword = false
		opt.IdentityFile = identifyFile
		opt.Pass = &req.GlobalLoginOptions.PrivateKeyPassword
	}

	final := func(builder *task.Builder, name string, meta spec.Metadata) {
		builder.UpdateTopology(name,
			tidbSpec.Path(name),
			meta.(*spec.ClusterMeta),
			nil, /* deleteNodeIds */
		)
	}

	// audit
	// logger.AddCustomAuditLog(fmt.Sprintf("[web] scale_out %s %s", clusterName, topoFilePath))
	logger.AddCustomAuditLog(fmt.Sprintf("[web] scale_out %s", clusterName))

	go func() {
		cm.DoScaleOut(
			clusterName,
			topoFilePath,
			postScaleOutHook,
			final,
			opt,
			skipConfirm,
			gOpt,
		)
	}()

	c.Status(http.StatusNoContent)
}

// UpOrDowngradeReq represents the request for upgrade or downgrade a cluster
type UpOrDowngradeReq struct {
	TargetVersion  string `json:"target_version"`
	SiblingVersion string `json:"sibling_version"`
}

func upgradeClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	var req UpOrDowngradeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] upgrade %s", clusterName))

	go func() {
		cm.DoUpgrade(
			clusterName,
			req.TargetVersion,
			gOpt,
		)
	}()

	c.Status(http.StatusNoContent)
}

func downgradeClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	var req UpOrDowngradeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}
	if req.SiblingVersion == "" {
		_ = c.Error(errors.New("sibling_version is empty"))
		return
	}

	// audit
	logger.AddCustomAuditLog(fmt.Sprintf("[web] downgrade %s", clusterName))

	go func() {
		cm.DoDowngrade(
			clusterName,
			req.TargetVersion,
			req.SiblingVersion,
			gOpt,
		)
	}()

	c.Status(http.StatusNoContent)
}

func mirrorHandler(c *gin.Context) {
	mirror := environment.Mirror()
	c.JSON(http.StatusOK, gin.H{
		"mirror_address": mirror,
	})
}

// MirrorReq represents for the request of setting mirror address
type MirrorReq struct {
	MirrorAddress string `json:"mirror_address"`
}

func setMirrorHandler(c *gin.Context) {
	var req MirrorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	// set mirror address
	req.MirrorAddress = strings.TrimSpace(req.MirrorAddress)
	if req.MirrorAddress == "" {
		req.MirrorAddress = repository.DefaultMirror
	}
	if req.MirrorAddress != environment.Mirror() {
		os.Setenv("TIUP_MIRRORS", "")
		profile := environment.GlobalEnv().Profile()
		if err := profile.ResetMirror(req.MirrorAddress, ""); err != nil {
			_ = c.Error(err)
			return
		}
	}

	c.Status(http.StatusNoContent)
}

func tidbVersionsHandler(c *gin.Context) {
	env := environment.GlobalEnv()
	var opt cmd.ListOptions
	result, err := cmd.ShowComponentVersions(env, "tidb", opt)
	if err != nil {
		_ = c.Error(err)
		return
	}

	versions := []string{}
	for _, lines := range result.CmpTable {
		for _, col := range lines {
			if strings.HasPrefix(col, "v") {
				versions = append(versions, col)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"versions": versions,
	})
}

func auditLogListHandler(c *gin.Context) {
	auditLogList, err := audit.GetAuditList(spec.AuditDir())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, auditLogList)
}

//////////////////////////////////////
// The following code are copied from command package to avoid cycle import

func postDeployHook(builder *task.Builder, topo spec.Topology) {
	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx context.Context) error {
		var err error
		// teleNodeInfos, err = operator.GetNodeInfo(ctx, topo)
		_, err = operator.GetNodeInfo(ctx, topo)
		_ = err
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

	if report.Enable() {
		builder.ParallelStep("+ Check status", false, nodeInfoTask)
	}

	enableTask := task.NewBuilder().Func("Setting service auto start on boot", func(ctx context.Context) error {
		return operator.Enable(ctx, topo, operator.Options{}, true)
	}).BuildAsStep("Enable service").SetHidden(true)

	builder.Parallel(false, enableTask)
}

func postScaleOutHook(builder *task.Builder, newPart spec.Topology) {
	postDeployHook(builder, newPart)
}

//////////////////////////

func test() {
	// func Now() Time
	fmt.Println(time.Now())

	// func Parse(layout, value string) (Time, error)
	t, _ := time.Parse("2006-01-02 15:04:05", "2018-04-23 12:24:51")
	fmt.Println(t)

	// func ParseInLocation(layout, value string, loc *Location) (Time, error) (layout已带时区时可直接用Parse)
	t, _ = time.ParseInLocation("2006-01-02 15:04:05", "2017-05-11 14:06:06", time.Local)
	fmt.Println(t)
}
