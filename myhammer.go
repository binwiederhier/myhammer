package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
		prefix := flags.String("prefix", "", "Query prefix")

		if err := flags.Parse(os.Args[2:]); err != nil {
			panic(err)
		}

		if *prefix != "" {
			*prefix += " "
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", *user, *pass, *host, *port)
		clean(dsn, *prefix)
	} else if os.Args[1] == "run" {
		flags := flag.NewFlagSet("run", flag.ExitOnError)
		workers := flags.Int("workers", 20, "Number of concurrent workers")
		host := flags.String("h", "127.0.0.1", "Hostname")
		port := flags.Int("P", 3306, "Port")
		user := flags.String("u", "root", "User")
		pass := flags.String("p", "", "Password")
		prefix := flags.String("prefix", "", "Query prefix")

		if err := flags.Parse(os.Args[2:]); err != nil {
			panic(err)
		}

		if *prefix != "" {
			*prefix += " "
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", *user, *pass, *host, *port)
		clean(dsn, *prefix)
		run(dsn, *prefix, *workers)
	} else {
		fmt.Println("Syntax: myhammer (clean|run)")
		os.Exit(1)
	}
}

func clean(dsn string, prefix string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("%sdrop database if exists myhammer", prefix))
	if err != nil {
		panic(err)
	}
}

func run(dsn string, prefix string, workers int) {
	// Create db and table
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("%screate database if not exists myhammer", prefix))
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(fmt.Sprintf("%screate table if not exists myhammer.t1 (k BIGINT AUTO_INCREMENT PRIMARY KEY, worker BIGINT, value BIGINT)", prefix))
	if err != nil {
		panic(err)
	}

	//
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Interrupt received. Stopping workers.")
		cancel()
	}()

	// Start workers
	wg := &sync.WaitGroup{}
	responses := make(chan int64)
	max := int64(-1)

	for i := 0; i < workers; i++{
		wg.Add(1)
		go hammer(dsn, prefix, i, responses, ctx, wg)
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

func hammer(dsn string, prefix string, worker int, responses chan int64, ctx context.Context, wg *sync.WaitGroup) {
	fmt.Println("Starting request loop ...")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	value := 0
	loop: for {
		select {
		case <-ctx.Done():
			fmt.Printf("cancelled: stopping worker %d\n", worker)
			break loop
		default:
			stmt, err := db.Prepare(fmt.Sprintf("%sinsert into myhammer.t1 (worker, value) values (?, ?)", prefix))
			if err != nil {
				fmt.Printf("error preparing statement in worker %d: %v\n", worker, err)
				continue loop
			}

			result, err := stmt.Exec(worker, value)
			if err != nil {
				fmt.Printf("error executing query in worker %d: %v\n", worker, err)
				stmt.Close()
				continue loop
			}
			last, err := result.LastInsertId()
			if err != nil {
				fmt.Printf("error getting last insert id in worker %d: %v\n", worker, err)
				stmt.Close()
				continue loop
			}

			stmt.Close()
			fmt.Printf("worker=%d value=%d key=%d\n", worker, value, last)
			responses <- last
			value++
		}
	}

	wg.Done()
}