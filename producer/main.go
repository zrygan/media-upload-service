package main

import (
	"fmt"
	"log"

	"github.com/zrygan/media-upload-service/producer/config"
)

func main() {
	pc, err := config.ReadConfigFromJSON(".configs/producer.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(pc.ThreadCount, pc.FolderList, pc.ConsumerAddress)
}
