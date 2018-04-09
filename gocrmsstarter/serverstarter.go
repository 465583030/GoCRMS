// usage: gocrmsstarter <server_count> <server_name> <parellel_count> <endpoint>

package main

import (
	"flag"
	"strconv"
	"os/exec"
	"log"
	"fmt"
)

func main() {
	flag.Parse()
	serverCount, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		log.Fatalln(err)
	}
	serverName := flag.Arg(1)
	args := make([]string, len(flag.Args()) - 1)
	copy(args[1:], flag.Args()[2:])
	for i := 0; i < serverCount; i++ {
		args[0] = fmt.Sprintf("%s_%02d", serverName, i)
		cmd := exec.Command("GoCRMS", args...)
		//defer fmt.Println(cmd))
		if err := cmd.Start(); err != nil {
			log.Fatalln(err)
		}
		defer cmd.Wait()
	}
}
