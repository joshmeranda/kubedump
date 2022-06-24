package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	kubedump "kubedump/pkg"
	"net/http"
)

func HandleHealth(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func HandleTar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// todo: generate tar
	default:
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(fmt.Sprintf("unsupported method '%s'", r.Method)))
	}
}

func HandleStart(w http.ResponseWriter, r *http.Request) {
}

func HandleStop(w http.ResponseWriter, r *http.Request) {
}

func main() {
	http.HandleFunc("/health", HandleHealth)
	http.HandleFunc("/tar", HandleTar)

	err := http.ListenAndServe(fmt.Sprintf(":%d", kubedump.Port), nil)

	if err != nil {
		logrus.Fatal("error starting http server: %s", err)
	}
}
