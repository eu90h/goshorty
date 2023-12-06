// Run a goshorty API shortening server
package main

import (
	"log"

	goshorty "github.com/eu90h/goshorty/pkg"
)

func main() {
	shorty := goshorty.NewShortyApp(nil)
	r := shorty.SetupRouter()
	if r == nil {
		log.Fatal("router was nil")
	}
	err := r.Run(shorty.Config.ListenAddr) // TODO: change to RunTLS for HTTPS support.
	if err != nil {
		log.Fatal(err)
	}
}
