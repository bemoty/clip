package main

import (
	"log"

	"github.com/bemoty/clip/cmd/client/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	header := &doc.GenManHeader{
		Title:   "CLIP",
		Section: "1",
	}
	if err := doc.GenManTree(cmd.RootCmd, header, "./man"); err != nil {
		log.Fatal(err)
	}
}
