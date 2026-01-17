// Package main provides the entry point for the mail sender service.
package main

import (
	"log"
	"os"

	"github.com/sony/micro-mail-sender/mailsender"
)

func main() {
	config, err := mailsender.ParseConfig(os.Getenv("MAILSENDER_CONFIG"))
	if err != nil {
		log.Fatal("Couldn't parse MAILSENDER_CONFIG string", err)
	}

	log.Fatal(mailsender.RunServer(config))
}
