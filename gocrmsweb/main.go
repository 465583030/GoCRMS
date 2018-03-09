package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	ws "github.com/WenzheLiu/GoCRMS/gocrmsweb/webservice"
	"github.com/julienschmidt/httprouter"
)

var webservice *ws.WebService

func main() {
	// get argument
	flag.Parse()
	webPort := flag.Arg(0)
	if webPort == "" {
		webPort = "8080"
	}
	endPoint := flag.Arg(0)
	if endPoint == "" {
		endPoint = "localhost:2379"
	}
	endpoints := []string{endPoint}
	webservice = ws.NewWebService(endpoints)

	router := httprouter.New()
	router.NotFound = http.FileServer(http.Dir("./dist"))

	router.GET("/api/servers", getServers)
	router.GET("/api/workers", getWorkers)
	router.GET("/api/jobs", getJobs)
	router.POST("/api/run", runJob)
	router.POST("/api/shutdown", shutdown)
	router.GET("/api/nodes", getNodes)

	log.Fatal(http.ListenAndServe(":"+webPort, router))
}

func reply(w http.ResponseWriter, v interface{}) {
	output, err := json.Marshal(v)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	replyBytes(w, output)
}

func replyBytes(w http.ResponseWriter, output []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func replyWithIndent(w http.ResponseWriter, v interface{}) {
	output, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	replyBytes(w, output)
}

func parseBody(r *http.Request, v interface{}) error {
	n := r.ContentLength
	body := make([]byte, n)
	r.Body.Read(body)
	return json.Unmarshal(body, v)
}

func getServers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	servers, err := webservice.GetServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reply(w, servers)
}

func getWorkers(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	r.ParseForm()
	host := r.FormValue("host")
	workers, err := webservice.GetWorkers(host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reply(w, workers)
}

func getJobs(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	r.ParseForm()
	worker := r.FormValue("host")
	jobs, err := webservice.GetJobsByWorker(worker)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reply(w, jobs)
}

func runJob(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var v ws.RunCommand
	err := parseBody(r, &v)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = webservice.RunJob(v)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(200)
}

func shutdown(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var v []string
	if err := parseBody(r, &v); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	webservice.Shutdown(v)
	w.WriteHeader(200)
}

func getNodes(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	nodes, err := webservice.GetNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	replyWithIndent(w, nodes)
}
