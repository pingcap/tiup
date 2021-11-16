package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	// mysql package
	_ "github.com/go-sql-driver/mysql"
)

var (
	dbName         string
	host           string
	port           int
	user           string
	password       string
	threads        int
	acThreads      int
	driver         string
	totalTime      time.Duration
	totalCount     int
	dropData       bool
	ignoreError    bool
	outputInterval time.Duration
	isolationLevel int
	silence        bool
	pprofAddr      string
	maxProcs       int

	globalDB  *sql.DB
	globalCtx context.Context
)

const (
	unknownDB   = "Unknown database"
	createDBDDL = "CREATE DATABASE IF NOT EXISTS "
	mysqlDriver = "mysql"
)

func closeDB() {
	if globalDB != nil {
		globalDB.Close()
	}
	globalDB = nil
}

func openDB() error {
	// TODO: support other drivers
	var (
		tmpDB *sql.DB
		err   error
		ds    = fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	)
	// allow multiple statements in one query to allow q15 on the TPC-H
	globalDB, err = sql.Open(mysqlDriver, fmt.Sprintf("%s%s?multiStatements=true", ds, dbName))
	if err != nil {
		return err
	}
	if err := globalDB.Ping(); err != nil {
		errString := err.Error()
		if strings.Contains(errString, unknownDB) {
			tmpDB, _ = sql.Open(mysqlDriver, ds)
			defer tmpDB.Close()
			if _, err := tmpDB.Exec(createDBDDL + dbName); err != nil {
				return fmt.Errorf("failed to create database, err %v", err)
			}
		} else {
			globalDB = nil
			return err
		}
	} else {
		globalDB.SetMaxIdleConns(threads + acThreads + 1)
	}

	return nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   os.Args[0],
		Short: "Benchmark database with different workloads",
	}
	rootCmd.PersistentFlags().IntVar(&maxProcs, "max-procs", 0, "runtime.GOMAXPROCS")
	rootCmd.PersistentFlags().StringVar(&pprofAddr, "pprof", "", "Address of pprof endpoint")
	rootCmd.PersistentFlags().StringVarP(&dbName, "db", "D", "test", "Database name")
	rootCmd.PersistentFlags().StringVarP(&host, "host", "H", "127.0.0.1", "Database host")
	rootCmd.PersistentFlags().StringVarP(&user, "user", "U", "root", "Database user")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "Database password")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "P", 4000, "Database port")
	rootCmd.PersistentFlags().IntVarP(&threads, "threads", "T", 1, "Thread concurrency")
	rootCmd.PersistentFlags().IntVarP(&acThreads, "acThreads", "t", 1, "OLAP client concurrency, only for CH-benCHmark")
	rootCmd.PersistentFlags().StringVarP(&driver, "driver", "d", "", "Database driver: mysql")
	rootCmd.PersistentFlags().DurationVar(&totalTime, "time", 1<<63-1, "Total execution time")
	rootCmd.PersistentFlags().IntVar(&totalCount, "count", 0, "Total execution count, 0 means infinite")
	rootCmd.PersistentFlags().BoolVar(&dropData, "dropdata", false, "Cleanup data before prepare")
	rootCmd.PersistentFlags().BoolVar(&ignoreError, "ignore-error", false, "Ignore error when running workload")
	rootCmd.PersistentFlags().BoolVar(&silence, "silence", false, "Don't print error when running workload")
	rootCmd.PersistentFlags().DurationVar(&outputInterval, "interval", 10*time.Second, "Output interval time")
	rootCmd.PersistentFlags().IntVar(&isolationLevel, "isolation", 0, `Isolation Level 0: Default, 1: ReadUncommitted,
2: ReadCommitted, 3: WriteCommitted, 4: RepeatableRead,
5: Snapshot, 6: Serializable, 7: Linerizable`)

	cobra.EnablePrefixMatching = true

	registerTpcc(rootCmd)
	registerTpch(rootCmd)
	registerCHBenchmark(rootCmd)
	registerYcsb(rootCmd)

	var cancel context.CancelFunc
	globalCtx, cancel = context.WithCancel(context.Background())

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	closeDone := make(chan struct{}, 1)
	go func() {
		sig := <-sc
		fmt.Printf("\nGot signal [%v] to exit.\n", sig)
		cancel()

		select {
		case <-sc:
			// send signal again, return directly
			fmt.Printf("\nGot signal [%v] again to exit.\n", sig)
			os.Exit(1)
		case <-time.After(10 * time.Second):
			fmt.Print("\nWait 10s for closed, force exit\n")
			os.Exit(1)
		case <-closeDone:
			return
		}
	}()

	err := rootCmd.Execute()
	cancel()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
