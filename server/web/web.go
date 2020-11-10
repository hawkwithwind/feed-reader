package web

import (
	"fmt"
	"log"
	//"io"
	"context"
	//"encoding/json"
	//"net"
	"net/http"
	//"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/handlers"
	//"github.com/gorilla/sessions"
	"github.com/hawkwithwind/mux"
	"github.com/hawkwithwind/logger"
	//"github.com/jmoiron/sqlx"
)

type WebConfig struct {
	Host      string
	Port      string
	Baseurl   string

	SecretPhrase string
	Log         *logger.LoggerConfig
	
	//Database     utils.DatabaseConfig
	//Sentry       string
	//GithubOAuth  GithubOAuthConfig
	AllowOrigin  []string
}

type WebServer struct {
	logger.Logger
	
	Config  WebConfig
	Hubport string
	Hubhost string

	//restfulclient *http.Client
	//db            *dbx.Database
	//store         *sessions.CookieStore
	//accounts      Accounts
}

func (web *WebServer) init() error {
	if web.Config.Log != nil {
		web.Logger.Config = *web.Config.Log
	} else {
		web.Logger.SetDefault("WEB")
	}
	web.Logger.Init()
	
	//ctx.restfulclient = httpx.NewHttpClient()

	//retryTimes := 7
	//gap := 2
	//for i := 0; i < retryTimes+1; i++ {
	//	o := &ErrorHandler{}
	//	if o.Connect(ctx.db, "mysql", ctx.Config.Database.DataSourceName); o.Err != nil {
	//		if i < retryTimes {
	//			ctx.Info("wait for mysql server establish...")
	//			time.Sleep(time.Duration(gap) * time.Second)
	//			gap = gap * 2
	//			o.Err = nil
	//		} else {
	//			ctx.Error(o.Err, "connect to database failed")
	//			return o.Err
	//		}
	//	}
	//}

	//if ctx.Config.Database.MaxConnectNum > 0 {
	//ctx.Info("set database max conn %d", ctx.Config.Database.MaxConnectNum)
	//	ctx.db.Conn.SetMaxOpenConns(ctx.Config.Database.MaxConnectNum)
	//}

	//go func(db *sqlx.DB) {
	//for {
	//time.Sleep(time.Duration(60) * time.Second)
	//fmt.Println("[WEB] database stats ", o.ToJson(db.Stats()))
	//}
	//}(ctx.db.Conn)

	return nil
}

func (ctx *WebServer) hello(w http.ResponseWriter, r *http.Request) {
	//o := &ErrorHandler{}
	//defer o.WebError(w)
	//o.ok(w, "hello", nil)
}

type key int

const (
	requestIDKey key = 0
)

var (
	healthy int32
)

func sentryContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//raven.SetHttpContext(raven.NewHttp(r))
		next.ServeHTTP(w, r)
	})
}

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (web *WebServer) serveHTTP(ctx context.Context) error {
	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	r := mux.NewRouter()

	r.Handle("/healthz", healthz())
	//r.HandleFunc("/echo", web.echo).Methods("Post")
	//r.HandleFunc("/hello", web.validate(web.hello)).Methods("GET")

	r.Use(mux.CORSMethodMiddleware(r))
	r.Use(handlers.CORS(
		handlers.AllowCredentials(),
		handlers.AllowedHeaders([]string{"Content-Type", "X-Requested-With"}),
		handlers.AllowedOrigins(web.Config.AllowOrigin)))
	r.Use(tracing(nextRequestID))
	r.Use(logging(web.Logger.Logger()))
	//r.Use(sentryContext)

	addr := fmt.Sprintf("%s:%s", web.Config.Host, web.Config.Port)
	web.Info("http server listen: %s", addr)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      r,
		ErrorLog:     web.Logger.Logger(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	running := true

	go func() {
		<-ctx.Done()

		if !running {
			return
		}

		web.Info("http server is shutting down")
		atomic.StoreInt32(&healthy, 0)

		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		httpServer.SetKeepAlivesEnabled(false)
		if err := httpServer.Shutdown(c); err != nil {
			web.Error(err, "Could not gracefully shutdown http server")
		}
	}()

	web.Info("http server starts")
	atomic.StoreInt32(&healthy, 1)

	var result error

	// err is ErrServerClosed if shut down gracefully
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		web.Error(err, "http server listen failed")
		result = err
	}
	
	web.Info("http server stopped")

	running = false

	return result
}


func (web *WebServer) Serve() {
	if web.init() != nil {
		return
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)

		<-quit

		cancelFunc()
	}()

	var waitGroup sync.WaitGroup
	
	go func() {
		waitGroup.Add(1)
		for true {
			if r := recover(); r != nil {
				web.Error(fmt.Errorf(fmt.Sprintf("%v", r)), "Web server recovers from panic")
			}

			_ = web.serveHTTP(ctx)
		}

		cancelFunc()

		waitGroup.Done()
	}()

	waitGroup.Wait()

	web.Info("web server ends")
}
