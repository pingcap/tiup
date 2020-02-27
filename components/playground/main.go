package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/c4pt0r/tiup/pkg/utils"
	_ "github.com/go-sql-driver/mysql"
)

func startPDServer() int {
	fmt.Println("starting pd server...")
	pd := exec.Command("tiup", "run", "pd")
	if err := pd.Start(); err != nil {
		panic(fmt.Sprintf("start pd server failed: %s", err.Error()))
	}
	return pd.Process.Pid
}

func startTiKVServer() int {
	tiupHome := os.Getenv("TIUP_HOME")
	if tiupHome == "" {
		panic("TIUP_HOME not set")
	}
	if utils.MustDir(path.Join(tiupHome, "data", "playground")) == "" {
		panic("create data directory for playground failed")
	}
	configPath := path.Join(tiupHome, "data", "playground", "tikv.toml")
	cf, err := os.Create(configPath)
	if err != nil {
		panic(err)
	}
	defer cf.Close()
	if err := writeConfig(cf); err != nil {
		panic(err)
	}

	fmt.Println("starting tikv server...")
	tikv := exec.Command("tiup", "run", "tikv", "--", "--pd=127.0.0.1:2379", fmt.Sprintf("--config=%s", configPath))
	if err := tikv.Start(); err != nil {
		panic(fmt.Sprintf("start tikv server failed: %s", err.Error()))
	}

	return tikv.Process.Pid
}

func startTiDBServer() int {
	fmt.Println("starting tidb server...")
	tidb := exec.Command("tiup", "run", "tidb", "--", "--store=tikv", "--path=127.0.0.1:2379")
	if err := tidb.Start(); err != nil {
		panic(fmt.Sprintf("run tidb: %s", err.Error()))
	}

	return tidb.Process.Pid
}

func main() {
	pids := []int{}
	pids = append(pids, startPDServer())
	pids = append(pids, startTiKVServer())
	pids = append(pids, startTiDBServer())

	var err error
	for i := 0; i < 5; i++ {
		if err = tryConnect("root:@tcp(127.0.0.1:4000)/"); err != nil {
			time.Sleep(time.Second * time.Duration(i*3))
		} else {
			break
		}
	}
	if err != nil {
		panic("connect tidb failed")
	} else {
		fmt.Println("now you can connect tidb with dns: root:@tcp(127.0.0.1:4000)/")
	}

	wait(pids)
}

func tryConnect(dsn string) error {
	fmt.Println("try connect: ", dsn)
	if cli, err := sql.Open("mysql", "root:@tcp(127.0.0.1:4000)/"); err != nil {
		fmt.Println(err)
		return err
	} else if _, err := cli.Conn(context.Background()); err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("connect success on", dsn)
	return nil
}

func wait(pids []int) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	for _, pid := range pids {
		fmt.Println("kill process", pid)
		syscall.Kill(pid, 9)
	}
}
