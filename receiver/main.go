package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"path/filepath"

	"strings"

	"github.com/eaciit/config"
	"github.com/eaciit/toolkit"
)

var (
	e      error
	log, _ = toolkit.NewLog(true, false, "", "", "")
	cdone  chan bool
	status string
	wd, _  = os.Getwd()

	path        string
	receiverUrl string
)

func main() {
	toolkit.Println("FileSync Receiver Deamon v0.5 (c) EACIIT")

	configFile := filepath.Join(wd, "..", "config", "app.json")
	if e = config.SetConfigFile(configFile); e != nil {
		log.Error("Unable to load config file " + configFile)
		return
	}

	portToMonitor := toolkit.ToInt(config.Get("ReceiverPort"), toolkit.RoundingAuto)
	path = config.GetDefault("ReceiverPath", "").(string)
	if path == "" {
		log.Error("Path is not yet defined")
		return
	}
	log.Info("Receiver path is " + path)

	cdone = make(chan bool)
	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		cdone <- true
		w.Write([]byte("Daemon will be stopped"))
	})

	http.HandleFunc("/", receive)

	go func() {
		e = http.ListenAndServe(toolkit.Sprintf(":%d", portToMonitor), nil)
		if e != nil {
			log.Error("Can not start daemon REST server. " + e.Error())
			cdone <- true
		}
	}()

	for {
		select {
		case <-cdone:
			status = "Closing"
			log.Info("Daemon will be closed")
			time.Sleep(1 * time.Second)
			return

		default:
			//-- do nothing
		}
	}
}

func receive(w http.ResponseWriter, r *http.Request) {
	status := "OK"
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		status = "NOK Filename is not available"
	} else {
		status += " " + filename
		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)
		if len(body) == 0 {
			status = "NOK " + filename + " No bytes"
		} else {
			dst := filepath.Join(path, filename)
			log.Info("Try to writing " + dst)

			e := ioutil.WriteFile(dst, body, 0644)
			if e != nil {
				status = toolkit.Sprintf("NOK Unable to write: %s %s", dst, e.Error())
			}
		}
	}
	w.Write([]byte(status))
	if strings.HasPrefix(status, "OK") {
		log.Info(status)
	} else {
		log.Error(status)
	}
}
