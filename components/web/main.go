package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pingcap/tiup/components/cluster/command"
	"github.com/pingcap/tiup/pkg/cluster"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	cors "github.com/rs/cors/wrapper/gin"
)

var tidbSpec *spec.SpecManager
var manager *cluster.Manager

func main() {
	if err := spec.Initialize("cluster"); err != nil {
		panic("initialize spec failed")
	}
	tidbSpec = spec.GetSpecManager()
	manager = cluster.NewManager("tidb", tidbSpec, spec.TiDBComponentVersion)

	router := gin.Default()
	router.Use(cors.AllowAll())
	api := router.Group("/api")
	{
		api.POST("/deploy", deployHandler)
		api.GET("/status", statusHandler)

		api.GET("/clusters", clustersHandler)
		api.GET("/clusters/:clusterName", clusterHandler)
		api.DELETE("/clusters/:clusterName", destroyClusterHandler)
		api.POST("/clusters/:clusterName/start", startClusterHandler)
		api.POST("/clusters/:clusterName/stop", stopClusterHandler)
		api.POST("/clusters/:clusterName/scale_in", scaleInClusterHandler)
		api.POST("/clusters/:clusterName/scale_out", scaleOutClusterHandler)
	}
	_ = router.Run()
}

// DeployGlobalLoginOptions represents the global options for deploy
type DeployGlobalLoginOptions struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	PrivateKey         string `json:"privateKey"`         // TODO: refine naming style
	PrivateKeyPassword string `json:"privateKeyPassword"` // TODO: refine naming style
}

// DeployReq represents for the request of deploy API
type DeployReq struct {
	ClusterName        string                   `json:"cluster_name"`
	TiDBVersion        string                   `json:"tidb_version"`
	TopoYaml           string                   `json:"topo_yaml"`
	GlobalLoginOptions DeployGlobalLoginOptions `json:"global_login_options"`
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

	// create private key file
	tmpfile, err = ioutil.TempFile("", "private_key")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.GlobalLoginOptions.PrivateKey))
	identifyFile := tmpfile.Name()
	tmpfile.Close()

	go func() {
		manager.DoDeploy(
			req.ClusterName,
			req.TiDBVersion,
			topoFilePath,
			cluster.DeployOptions{
				User:         req.GlobalLoginOptions.Username,
				IdentityFile: identifyFile,
			},
			command.PostDeployHook,
			true,
			120,
			5,
			false,
		)
	}()

	c.Status(http.StatusNoContent)
}

func statusHandler(c *gin.Context) {
	status := manager.GetOperationStatus()
	c.JSON(http.StatusOK, status)
}

func clustersHandler(c *gin.Context) {
	clusters, err := manager.ListCluster()
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, clusters)
}

func clusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	instInfos, err := manager.Display(clusterName, operator.Options{
		SSHTimeout: 5,
		OptTimeout: 120,
		APITimeout: 300,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, instInfos)
}

func destroyClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	go func() {
		manager.DoDestroyCluster(clusterName, operator.Options{
			SSHTimeout: 5,
			OptTimeout: 120,
			APITimeout: 300,
		}, operator.Options{}, true)
	}()

	c.Status(http.StatusNoContent)
}

func startClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")

	go func() {
		manager.DoStartCluster(clusterName, operator.Options{
			SSHTimeout: 5,
			OptTimeout: 120,
			APITimeout: 300,
		}, func(b *task.Builder, metadata spec.Metadata) {
			tidbMeta := metadata.(*spec.ClusterMeta)
			b.UpdateTopology(clusterName, tidbMeta, nil)
		})
	}()

	c.Status(http.StatusNoContent)
}

func stopClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	go func() {
		manager.DoStopCluster(clusterName, operator.Options{
			SSHTimeout: 5,
			OptTimeout: 120,
			APITimeout: 300,
		})
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
		gOpt := operator.Options{
			SSHTimeout: 5,
			OptTimeout: 120,
			APITimeout: 300,
			NativeSSH:  false,
			Force:      req.Force,
			Nodes:      req.Nodes}
		scale := func(b *task.Builder, imetadata spec.Metadata) {
			metadata := imetadata.(*spec.ClusterMeta)
			if !gOpt.Force {
				b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, gOpt).
					UpdateMeta(clusterName, metadata, operator.AsyncNodes(metadata.Topology, gOpt.Nodes, false)).
					UpdateTopology(clusterName, metadata, operator.AsyncNodes(metadata.Topology, gOpt.Nodes, false))
			} else {
				b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, gOpt).
					UpdateMeta(clusterName, metadata, gOpt.Nodes).
					UpdateTopology(clusterName, metadata, gOpt.Nodes)
			}
		}

		manager.DoScaleIn(clusterName, true, gOpt.OptTimeout, gOpt.SSHTimeout, gOpt.NativeSSH, gOpt.Force, gOpt.Nodes, scale)
	}()

	c.Status(http.StatusNoContent)
}

// ScaleOutReq represents the request for scale out
type ScaleOutReq struct {
	TopoYaml           string                   `json:"topo_yaml"`
	GlobalLoginOptions DeployGlobalLoginOptions `json:"global_login_options"`
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

	// create private key file
	tmpfile, err = ioutil.TempFile("", "private_key")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.GlobalLoginOptions.PrivateKey))
	identifyFile := tmpfile.Name()
	tmpfile.Close()

	go func() {
		manager.DoScaleOut(
			clusterName,
			topoFilePath,
			command.PostScaleOutHook,
			command.Final,
			cluster.ScaleOutOptions{
				User:         req.GlobalLoginOptions.Username,
				IdentityFile: identifyFile,
			},
			true,
			120,
			5,
			false,
		)
	}()

	c.Status(http.StatusNoContent)
}
