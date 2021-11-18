package main

import (
	"github.com/apolubotko/image-previewer/internal/proxy"
	log "github.com/sirupsen/logrus"
)

func main() {
	config, err := proxy.NewConfig()
	checkErr(err)
	log.Infof("Config: %#v\n", *config)

	server, err := proxy.NewInstance(config)
	checkErr(err)

	server.Start()
}

func checkErr(err error) {
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
