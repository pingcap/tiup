package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
		api.GET("/clusters", clustersHandler)
		api.GET("/clusters/:clusterName", clusterHandler)
		api.DELETE("/clusters/:clusterName", destroyClusterHandler)
		api.POST("/clusters/:clusterName/start", startClusterHandler)
		api.POST("/clusters/:clusterName/stop", stopClusterHandler)

		api.POST("/deploy", deployHandler)
		api.GET("/deploy_status", deployStatusHandler)
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
	// fmt.Println("topo file path:", topoFilePath)

	// create private key file
	tmpfile, err = ioutil.TempFile("", "private_key")
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = tmpfile.WriteString(strings.TrimSpace(req.GlobalLoginOptions.PrivateKey))
	identifyFile := tmpfile.Name()
	tmpfile.Close()

	// parse request parameters
	// topoFilePath = "/Users/baurine/Codes/Work/tiup/examples/manualTestEnv/multiHost/topology.yaml"
	// identifyFile := "/Users/baurine/Codes/Work/tiup/examples/manualTestEnv/_shared/vagrant_key"
	go func() {
		_ = manager.Deploy(
			req.ClusterName,
			req.TiDBVersion,
			topoFilePath,
			cluster.DeployOptions{
				User:         req.GlobalLoginOptions.Username,
				IdentityFile: identifyFile,
			},
			nil,
			true,
			120,
			5,
			false,
		)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func deployStatusHandler(c *gin.Context) {
	status := manager.GetDeployStatus()
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
	err := manager.DestroyCluster(clusterName, operator.Options{
		SSHTimeout: 5,
		OptTimeout: 120,
		APITimeout: 300,
	}, operator.Options{}, true)

	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func startClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	err := manager.StartCluster(clusterName, operator.Options{
		SSHTimeout: 5,
		OptTimeout: 120,
		APITimeout: 300,
	}, func(b *task.Builder, metadata spec.Metadata) {
		tidbMeta := metadata.(*spec.ClusterMeta)
		b.UpdateTopology(clusterName, tidbMeta, nil)
	})

	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func stopClusterHandler(c *gin.Context) {
	clusterName := c.Param("clusterName")
	err := manager.StopCluster(clusterName, operator.Options{
		SSHTimeout: 5,
		OptTimeout: 120,
		APITimeout: 300,
	})

	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
