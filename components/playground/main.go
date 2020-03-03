package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/c4pt0r/tiup/components/playground/instance"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

func check(component string) {
	if _, err := os.Stat(path.Join(os.Getenv("TIUP_HOME"), "components", component)); err != nil {
		c := exec.Command("tiup", "install", component)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			panic("install " + component + " failed")
		}
	}
}

func main() {
	tidbNum := 1
	tikvNum := 1
	pdNum := 1

	if os.Getenv("TIUP_INSTANCE") == "" {
		os.Setenv("TIUP_INSTANCE", "default")
	}

	for _, comp := range []string{"pd", "tikv", "tidb"} {
		check(comp)
	}

	rootCmd := &cobra.Command{
		Use:   "playground",
		Short: "Bootstrap a TiDB cluster in your local host",
		RunE: func(cmd *cobra.Command, args []string) error {
			insts := []instance.Instance{}
			pds := []*instance.PDInstance{}
			kvs := []*instance.TiKVInstance{}
			dbs := []*instance.TiDBInstance{}
			for i := 0; i < pdNum; i++ {
				pds = append(pds, instance.NewPDInstance(i))
				insts = append(insts, pds[i])
			}
			for _, pd := range pds {
				pd.Join(pds)
			}
			for i := 0; i < tikvNum; i++ {
				kvs = append(kvs, instance.NewTiKVInstance(i, pds))
				insts = append(insts, kvs[i])
			}
			for i := 0; i < tidbNum; i++ {
				dbs = append(dbs, instance.NewTiDBInstance(i, pds))
				insts = append(insts, dbs[i])
			}

			for _, inst := range insts {
				if err := inst.Start(); err != nil {
					return err
				}
			}

			fmt.Println("bootstraping...")
			dsn := fmt.Sprintf("root:@tcp(%s)/", dbs[0].Addr())
			bootstrap(dsn)

			for _, inst := range insts {
				inst.Wait()
			}

			return nil
		},
	}

	rootCmd.Flags().IntVarP(&tidbNum, "db", "", 1, "TiDB instance number")
	rootCmd.Flags().IntVarP(&tikvNum, "kv", "", 1, "TiKV instance number")
	rootCmd.Flags().IntVarP(&pdNum, "pd", "", 1, "PD instance number")
	rootCmd.Execute()
}

func tryConnect(dsn string) error {
	cli, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer cli.Close()

	conn, err := cli.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

func bootstrap(dsn string) {
	for i := 0; i < 60; i++ {
		if err := tryConnect(dsn); err != nil {
			time.Sleep(time.Second)
		} else {
			fmt.Println("now you can connect tidb with dsn:", dsn)
			break
		}
	}
}
