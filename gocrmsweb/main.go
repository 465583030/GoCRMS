package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/api/servers", getServers)
	http.Handle("/", http.FileServer(http.Dir("./dist")))
	http.ListenAndServe(":8080", nil)
}

type Server struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	IsReachable bool   `json:"isReachable"`
}

func getServers(w http.ResponseWriter, r *http.Request) {
	server := Server{"172.168.58.102", 8080, "new", true}
	servers := []Server{server}
	output, err := json.Marshal(&servers)
	if err != nil {
		log.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}
