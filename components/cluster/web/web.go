package web

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pingcap/tiup/components/cluster/web/uiserver"

	"github.com/pingcap/tiup/pkg/cluster"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/report"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	cors "github.com/rs/cors/wrapper/gin"
)

var (
	tidbSpec    *spec.SpecManager
	manager     *cluster.Manager
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
func Run(_tidbSpec *spec.SpecManager, _manager *cluster.Manager, _gOpt operator.Options) {
	tidbSpec = _tidbSpec
	manager = _manager
	gOpt = _gOpt

	router := gin.Default()
	router.Use(cors.AllowAll())
	router.Use(MWHandleErrors())
	// Backend API
	api := router.Group("/api")
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

		api.GET("/mirror", mirrorHandler)
		api.POST("/mirror", setMirrorHandler)
	}
	// Frontend assets
	router.StaticFS("/tiup", uiserver.Assets)
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tiup")
		c.Abort()
	})

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
	status := manager.GetOperationStatus()
	c.JSON(http.StatusOK, status)
}

func deployHandler(c *gin.Context) {
	var req DeployReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}

	// create temp topo yaml file
	tmpfile, err := ioutil.TempFile("", "topo")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.TopoYaml))
	topoFilePath := tmpfile.Name()
	tmpfile.Close()

	opt := cluster.DeployOptions{
		User: req.GlobalLoginOptions.Username,
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

	go func() {
		manager.DoDeploy(
			req.ClusterName,
			req.TiDBVersion,
			topoFilePath,
			opt,
			postDeployHook,
			skipConfirm,
			gOpt.OptTimeout,
			gOpt.SSHTimeout,
			gOpt.SSHType,
		)
	}()

	c.Status(http.StatusNoContent)
}

func clustersHandler(c *gin.Context) {
	clusters, err := manager.GetClusterList()
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, clusters)
}

func clusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	instInfos, err := manager.GetClusterTopology(clusterName, gOpt)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, instInfos)
}

func destroyClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	go func() {
		manager.DoDestroyCluster(clusterName, gOpt, operator.Options{}, skipConfirm)
	}()

	c.Status(http.StatusNoContent)
}

func startClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	go func() {
		manager.DoStartCluster(clusterName, gOpt, func(b *task.Builder, metadata spec.Metadata) {
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
	go func() {
		manager.DoStopCluster(clusterName, gOpt)
	}()

	c.Status(http.StatusNoContent)
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

			if !opt.Force {
				b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, opt, tlsCfg).
					UpdateMeta(clusterName, metadata, operator.AsyncNodes(metadata.Topology, opt.Nodes, false)).
					UpdateTopology(
						clusterName,
						tidbSpec.Path(clusterName),
						metadata,
						operator.AsyncNodes(metadata.Topology, opt.Nodes, false), /* deleteNodeIds */
					)
			} else {
				b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, opt, tlsCfg).
					UpdateMeta(clusterName, metadata, opt.Nodes).
					UpdateTopology(
						clusterName,
						tidbSpec.Path(clusterName),
						metadata,
						opt.Nodes,
					)
			}
		}

		manager.DoScaleIn(
			clusterName,
			skipConfirm,
			opt.OptTimeout,
			opt.SSHTimeout,
			opt.SSHType,
			opt.Force,
			opt.Nodes,
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
	tmpfile, err := ioutil.TempFile("", "topo")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.TopoYaml))
	topoFilePath := tmpfile.Name()
	tmpfile.Close()

	opt := cluster.ScaleOutOptions{
		User: req.GlobalLoginOptions.Username,
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

	go func() {
		manager.DoScaleOut(
			clusterName,
			topoFilePath,
			postScaleOutHook,
			final,
			opt,
			skipConfirm,
			gOpt.OptTimeout,
			gOpt.SSHTimeout,
			gOpt.SSHType,
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

//////////////////////////////////////
// The following code are copied from command package to avoid cycle import

// Deprecated
func convertStepDisplaysToTasks(t []*task.StepDisplay) []task.Task {
	tasks := make([]task.Task, 0, len(t))
	for _, sd := range t {
		tasks = append(tasks, sd)
	}
	return tasks
}

func postDeployHook(builder *task.Builder, topo spec.Topology) {
	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx *task.Context) error {
		_, _ = operator.GetNodeInfo(context.Background(), ctx, topo)
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

	if report.Enable() {
		builder.ParallelStep("+ Check status", false, nodeInfoTask)
	}

	enableTask := task.NewBuilder().Func("Enable cluster", func(ctx *task.Context) error {
		return operator.Enable(ctx, topo, operator.Options{}, true)
	}).BuildAsStep("Enable cluster").SetHidden(true)

	builder.ParallelStep("+ Enable cluster", false, enableTask)
}

func postScaleOutHook(builder *task.Builder, newPart spec.Topology) {
	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx *task.Context) error {
		_, _ = operator.GetNodeInfo(context.Background(), ctx, newPart)
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

	if report.Enable() {
		builder.Parallel(false, convertStepDisplaysToTasks([]*task.StepDisplay{nodeInfoTask})...)
	}
}
