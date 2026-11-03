package main

import (
	"fmt"
	"log"

	"github.com/zrygan/media-upload-service/producer/config"
)

func main() {
	config, err := config.ReadConfigFromJSON(".configs/producer.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(config.ThreadCount, config.FolderList, config.ConsumerAddress)
}
