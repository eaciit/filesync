package main

import (
	"net/http"
	"os"
	"path/filepath"

	"time"

	"strings"

	"io/ioutil"

	"github.com/eaciit/config"
	"github.com/eaciit/toolkit"
	"github.com/eaciit/uklam"
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
	toolkit.Println("FileSync Sender Deamon v0.5 (c) EACIIT")

	configFile := filepath.Join(wd, "..", "config", "app.json")
	if e = config.SetConfigFile(configFile); e != nil {
		log.Error("Unable to load config file " + configFile)
		return
	}

	portToMonitor := toolkit.ToInt(config.Get("SenderPort"), toolkit.RoundingAuto)
	path = config.GetDefault("SenderPath", filepath.Join(wd, "data")).(string)
	receiverUrl = config.GetDefault("ReceiverUri", "").(string)
	if receiverUrl == "" {
		log.Error("No receiver has been defined")
		return
	}

	toolkit.Printfn("Run http://localhost:%d/stop to stop the daemon", portToMonitor)
	toolkit.Printfn("Sender path is: %s", path)
	log, _ = toolkit.NewLog(true, false, "", "", "")
	defer func() {
		log.Info("Closing daemon")
	}()

	winbox := prepareInbox()
	winbox.Start()
	defer winbox.Stop()

	wrun := prepareRunning()
	wrun.Start()
	defer wrun.Stop()

	cdone = make(chan bool)
	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		cdone <- true
		w.Write([]byte("Daemon will be stopped"))
	})

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
			time.Sleep(1 * time.Second)
			return

		default:
			//-- do nothing
		}
	}
}

func prepareInbox() *uklam.FSWalker {
	w := uklam.NewFS(filepath.Join(path, "inbox"))
	w.EachFn = func(dw uklam.IDataWalker, in toolkit.M, info os.FileInfo, r *toolkit.Result) {
		name := info.Name()
		sourcename := filepath.Join(path, "inbox", name)
		dstname := filepath.Join(path, "running", name)
		//log.Info(toolkit.Sprintf("Processing " + sourcename))
		e := uklam.FSCopy(sourcename, dstname, true)
		if e != nil {
			log.Error("Processing " + sourcename + " NOK " + e.Error())
		} else {
			log.Info("Processing " + sourcename + " OK ")
		}
	}
	return w
}

func prepareRunning() *uklam.FSWalker {
	w2 := uklam.NewFS(filepath.Join(path, "running"))
	w2.EachFn = func(dw uklam.IDataWalker, in toolkit.M, info os.FileInfo, r *toolkit.Result) {
		name := info.Name()
		//if strings.HasSuffix(name, ".csv") {
		sourcename := filepath.Join(path, "running", name)
		dstnameOK := filepath.Join(path, "success", name)
		dstnameNOK := filepath.Join(path, "fail", name)
		//log.Info(toolkit.Sprintf("Reading %s", sourcename))
		e := streamDo(sourcename, info)
		if e == nil {
			if e = uklam.FSCopy(sourcename, dstnameOK, true); e != nil {
				log.Error(toolkit.Sprintf("Run %s NOK: %s", sourcename, e.Error()))
			} else {
				//log.Info(toolkit.Sprintf("Run %s OK", sourcename))
			}
		} else {
			uklam.FSCopy(sourcename, dstnameNOK, true)
			log.Error(toolkit.Sprintf("Run %s NOK: %s", sourcename, e.Error()))
		}
		//}
	}
	return w2
}

func streamDo(sourceName string, info os.FileInfo) error {
	url := strings.Replace(receiverUrl, "{0}", info.Name(), 1)
	filename := info.Name()
	log.Info(toolkit.Sprintf("Sending %s ==> %s", filename, url))

	bufs := []byte{}
	bufs, e := ioutil.ReadFile(sourceName)
	if e != nil {
		return toolkit.Errorf("Reading %s %s", filename, e.Error())
	}
	if len(bufs) == 0 {
		return toolkit.Errorf("Reading %s no bytes", filename)
	}

	_, e = toolkit.HttpCall(url, "GET", bufs, toolkit.M{})
	if e != nil {
		return toolkit.Errorf("REST %s %s", filename, e.Error())
	}
	return nil
}
