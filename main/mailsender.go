package main

import (
	"flag"
	"log"
	"os"

	"github.com/sony/micro-mail-sender/mailsender"
)

func main() {
	pstandalone := flag.Bool("standalone", false, "Invoke necessary daemons")

	flag.Parse()

	config, err := mailsender.ParseConfig(os.Getenv("MAILSENDER_CONFIG"))
	if err != nil {
		log.Fatal("Couldn't parse MAILSENDER_CONFIG string", err)
	}

	if *pstandalone {
		if !mailsender.StartDaemons(config) {
			log.Fatal("Failed to run daemons.  Exitting.")
		}
	}

	log.Fatal(mailsender.RunServer(config))
}
