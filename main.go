package main

import (
	"os"
	"net/http"
	"fmt"
	"runtime"
	"gylib/common"
	"appcenter/handle"
)

func main(){
	var port string = ""
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	var MULTICORE int = runtime.NumCPU() //number of core
	runtime.GOMAXPROCS(MULTICORE)        //running in multicore
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	//http.Handle("/favicon.ico",http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", handle.ApiHandler)
	//http.HandleFunc("/admin/", handle.IndexHandler)
	data := common.Getini("conf/app.ini", "server", map[string]string{"port": ""})
	if port == "" {
		port = data["port"]
	}
	fmt.Println(data)
	http.ListenAndServe(":"+port, nil)
}
