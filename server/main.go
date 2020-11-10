package main

import (
	"flag"
	//"fmt"
	"os"
	//"strconv"
	"sync"
	"time"

	_ "github.com/getsentry/raven-go"
	"gopkg.in/yaml.v2"

	"github.com/hawkwithwind/feed-reader/server/web"
	"github.com/hawkwithwind/logger"
)

type MainConfig struct {
	Web       web.WebConfig
}

var (
	configPath = flag.String("c", "/config/config.yml", "config file path")
	startcmd   = flag.String("s", "", "start command: web")
	config     MainConfig
)

func loadConfig(configPath string) (MainConfig, error) {
	c := MainConfig{}

	config, err := os.Open(configPath)
	defer config.Close()
	if err != nil {
		return c, err
	}

	data := make([]byte, 16*1024)
	len := 0
	for {
		n, _ := config.Read(data)
		if 0 == n {
			break
		}
		len += n
	}

	err = yaml.Unmarshal(data[:len], &c)
	if err != nil {
		return c, err
	}

	//dbuser := os.Getenv("DB_USER")
	//dbpassword := os.Getenv("DB_PASSWORD")
	//dbname := os.Getenv("DB_NAME")
	//dblink := os.Getenv("DB_ALIAS")
	//dbparams := os.Getenv("DB_PARAMS")
	//dbmaxconn := os.Getenv("DB_MAXCONN")

	//c.Web.Database.DataSourceName = fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", dbuser, dbpassword, dblink, dbname, dbparams)

	//if dbmaxconn != "" {
	//maxconn, err := strconv.Atoi(dbmaxconn)
	//	if err == nil {
	//		c.Web.Database.MaxConnectNum = maxconn
	//	}
	//}

	return c, nil
}

func main() {
	flag.Parse()
	
	l := &logger.Logger{}
	l.SetDefault("MAIN")
	l.Init()
		
	l.Info("config path %s", *configPath)
	
	var wg sync.WaitGroup
	l.Info("server %s starts.", *startcmd)

	var err error
	if config, err = loadConfig(*configPath); err != nil {
		l.Error(err, "failed to open config file %s, exit.", *configPath)
		return
	}
	
	//raven.SetDSN(config.Web.Sentry)

	if *startcmd == "web" {
		wg.Add(1)

		go func() {
			defer wg.Done()
			
			webserver := web.WebServer{
				Config:  config.Web,
			}
			webserver.Serve()
		}()
	}

	time.Sleep(5 * time.Second)
	wg.Wait()
	l.Info("server ends.")
}
