package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/WenzheLiu/GoCRMS/gocrmsweb/webservice"
	"github.com/julienschmidt/httprouter"
)

func main() {
	router := httprouter.New()
	router.NotFound = http.FileServer(http.Dir("./dist"))

	router.GET("/api/servers", getServers)
	router.GET("/api/workers", getWorkers)
	router.GET("/api/jobs", getJobs)
	router.POST("/api/run", runJob)
	router.POST("/api/shutdown", shutdown)

	log.Fatal(http.ListenAndServe(":8080", router))
}

func reply(w http.ResponseWriter, v interface{}) {
	output, err := json.Marshal(v)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
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
	var v webservice.RunCommand
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
