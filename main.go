package main

import (
	"github.com/julienschmidt/httprouter"
	"github.com/kardianos/osext"
	"log"
	"net/http"
	"sync"
	//"github.com/toqueteos/webbrowser"
	"flag"
	"github.com/boltdb/bolt"
	//isql "github.com/mpdroog/invoiced/sql"
	//"database/sql"
	//_ "github.com/mattn/go-sqlite3"

	"github.com/mpdroog/invoiced/config"
	"github.com/mpdroog/invoiced/hour"
	"github.com/mpdroog/invoiced/invoice"
	"github.com/mpdroog/invoiced/middleware"
	"github.com/mpdroog/invoiced/migrate"
	"github.com/mpdroog/invoiced/rules"
	"github.com/mpdroog/invoiced/metrics"
)

func Index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	prefix := "http://"
	if config.HTTPSOnly {
		prefix = "https://"
	}

	http.Redirect(w, r, prefix+config.HTTPListen+"/static/", 301)
}

func main() {
	var wg sync.WaitGroup
	flag.BoolVar(&config.Verbose, "v", false, "Verbose-mode (log more)")
	flag.StringVar(&config.DbPath, "d", "./billing.db", "Path to BoltDB database")
	flag.StringVar(&config.HTTPListen, "h", "localhost:9999", "HTTP listening port")
	flag.BoolVar(&config.HTTPSOnly, "s", false, "HTTPS Only")
	flag.Parse()

	var e error
	config.CurDir, e = osext.ExecutableFolder()
	if e != nil {
		log.Fatal(e)
	}
	if config.Verbose {
		log.Printf("Curdir=%s\n", config.CurDir)
		log.Printf("BoltDB=%s\n", config.DbPath)
	}
	db, e := bolt.Open(config.DbPath, 0600, nil)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()

	if e := migrate.Convert(db); e != nil {
		log.Fatal(e)
	}

	if e := invoice.Init(db); e != nil {
		log.Fatal(e)
	}
	if e := hour.Init(db); e != nil {
		log.Fatal(e)
	}
	if e := metrics.Init(db); e != nil {
		log.Fatal(e)
	}

	if e := rules.Init(); e != nil {
		log.Fatal(e)
	}

	/*log.Println(home + "/billing.sqlite")
	  db, err := sql.Open("sqlite3", home + "/billing.sqlite")
	  if err != nil {
	      log.Fatal(err)
	  }
	  defer db.Close()*/

	/*if e := isql.Init(db); e != nil {
	    log.Fatal(err)
	}*/

	// TODO: Init db?

	router := httprouter.New()
	router.GET("/", Index)

	//router.GET("/api/sql/all", isql.GetAll)
	//router.GET("/api/sql/row", isql.GetRow)

	router.GET("/api/v1/metrics", metrics.Dashboard)

	router.GET("/api/v1/invoices", invoice.List)
	router.POST("/api/v1/invoice", invoice.Save)
	router.GET("/api/v1/invoice/:id", invoice.Load)
	router.POST("/api/v1/invoice/:id/finalize", invoice.Finalize)
	router.POST("/api/v1/invoice/:id/reset", invoice.Reset)
	router.GET("/api/v1/invoice/:id/pdf", invoice.Pdf)
	router.GET("/api/v1/invoice/:id/credit", invoice.Credit)
	router.POST("/api/v1/invoice/:id/paid", invoice.Paid)
	router.POST("/api/v1/invoice/:id/balance", invoice.Balance)
	router.DELETE("/api/v1/invoice/:id", invoice.Delete)

	router.GET("/api/v1/hours", hour.List)
	router.POST("/api/v1/hour", hour.Save)
	router.GET("/api/v1/hour/:id", hour.Load)
	router.DELETE("/api/v1/hour/:id", hour.Delete)

	router.ServeFiles("/static/*filepath", http.Dir(config.CurDir+"/static"))

	wg.Add(1)
	go func() {
		var e error
		if config.Verbose {
			log.Printf("Listening on %s\n", config.HTTPListen)
			e = http.ListenAndServe(config.HTTPListen, middleware.HTTPLog(router))
		} else {
			e = http.ListenAndServe(config.HTTPListen, router)
		}
		wg.Done()
		if e != nil {
			log.Fatal(e)
		}
	}()
	wg.Wait()
}
