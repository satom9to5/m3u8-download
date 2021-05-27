package main

import (
	"fmt"
	"m3u8-download/m3u8"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("title & url is not found.")
		os.Exit(1)
	}

	m3u8.Init()

	si, _ := m3u8.MaxPriorityStream(os.Args[2])
	si.Download(os.Args[1])
}
