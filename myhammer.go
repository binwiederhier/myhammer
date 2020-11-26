package main

import (
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)
import _ "database/sql"
import _ "github.com/go-sql-driver/mysql"

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) < 2 {
		fmt.Println("Syntax: myhammer (clean|run)")
		os.Exit(1)
	}

	if os.Args[1] == "clean" {
		flags := flag.NewFlagSet("clean", flag.ExitOnError)
		host := flags.String("h", "127.0.0.1", "Hostname")
		port := flags.Int("P", 3306, "Port")
		user := flags.String("u", "root", "User")
		pass := flags.String("p", "", "Password")

		if err := flags.Parse(os.Args[2:]); err != nil {
			panic(err)
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", *user, *pass, *host, *port)
		clean(dsn)
	} else if os.Args[1] == "run" {
		flags := flag.NewFlagSet("run", flag.ExitOnError)
		workers := flags.Int("workers", 20, "Number of concurrent workers")
		host := flags.String("h", "127.0.0.1", "Hostname")
		port := flags.Int("P", 3306, "Port")
		user := flags.String("u", "root", "User")
		pass := flags.String("p", "", "Password")

		if err := flags.Parse(os.Args[2:]); err != nil {
			panic(err)
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", *user, *pass, *host, *port)
		clean(dsn)
		run(dsn, *workers)
	} else {
		fmt.Println("Syntax: myhammer (clean|run)")
		os.Exit(1)
	}
}

func clean(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec("drop database if exists myhammer")
	if err != nil {
		panic(err)
	}
}

func run(dsn string, workers int) {
	// Create db and table
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec("create database if not exists myhammer")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("create table if not exists myhammer.t1 (k BIGINT AUTO_INCREMENT PRIMARY KEY, worker BIGINT, value BIGINT)")
	if err != nil {
		panic(err)
	}

	// Start workers
	wg := &sync.WaitGroup{}
	responses := make(chan int64)
	max := int64(-1)

	for i := 0; i < workers; i++{
		wg.Add(1)
		go hammer(dsn, i, responses, wg)
		time.Sleep(100 * time.Millisecond)
	}

	go func() {
		for r := range responses {
			if r > max {
				max = r
			}
		}
	}()

	wg.Wait()
	fmt.Printf("Program exited. Max key = %d\n", max)
}

func hammer(dsn string, worker int, responses chan int64, wg *sync.WaitGroup) {
	fmt.Println("Starting request loop ...")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	stmt, err := db.Prepare("insert into myhammer.t1 (worker, value) values (?, ?)")
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	value := 0
	for {
		result, err := stmt.Exec(worker, value)
		if err != nil {
			fmt.Printf("error: %v, stopping worker %d\n", err, worker)
			break
		}
		last, err := result.LastInsertId()
		if err != nil {
			fmt.Printf("error: %v, stopping worker %d\n", err, worker)
			break
		}

		fmt.Printf("worker=%d value=%d key=%d\n", worker, value, last)
		responses <- last
		value++
	}

	wg.Done()
}