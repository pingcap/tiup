package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pingcap/tiup/pkg/cluster"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
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
		api.DELETE("/clusters/:clusterName", destroyClusterHandler)

		api.POST("/deploy", deployHandler)
		api.GET("/deploy_status", deployStatusHandler)
	}
	_ = router.Run()
}

// DeployReq represents for the request of deploy API
type DeployReq struct {
	ClusterName string `json:"cluster_name"`
	TiDBVersion string `json:"tidb_version"`
	TopoYaml    string `json:"topo_yaml"`
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
	defer tmpfile.Close()
	_, _ = tmpfile.WriteString(req.TopoYaml)
	topoFilePath := tmpfile.Name()
	fmt.Println("topo file path:", topoFilePath)

	// parse request parameters
	topoFilePath = "/Users/baurine/Codes/Work/tiup/examples/manualTestEnv/multiHost/topology.yaml"
	identifyFile := "/Users/baurine/Codes/Work/tiup/examples/manualTestEnv/_shared/vagrant_key"
	go func() {
		_ = manager.Deploy(
			req.ClusterName,
			req.TiDBVersion,
			topoFilePath,
			cluster.DeployOptions{
				User:         "vagrant",
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
