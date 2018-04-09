// usage: gocrmsstarter <server_count> <server_name> <parellel_count> <endpoint>

package main

import (
	"flag"
	"strconv"
	"os/exec"
	"log"
	"fmt"
	"sync"
)

func main() {
	flag.Parse()
	serverCount, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		log.Fatalln(err)
	}
	serverName := flag.Arg(3)
	args := make([]string, len(flag.Args()) - 1)
	copy(args[1:3], flag.Args()[1:3])
	var wg sync.WaitGroup
	wg.Add(serverCount)
	for i := 0; i < serverCount; i++ {
		args[0] = fmt.Sprintf("%s_%02d", serverName, i)
		cmd := exec.Command("GoCRMS", args...)
		//defer fmt.Println(cmd))
		go func(c *exec.Cmd) {
			if err := c.Run(); err != nil {
				log.Fatalln(err)
			}
			wg.Done()
		}(cmd)
		//if err := cmd.Start(); err != nil {
		//	log.Fatalln(err)
		//}
		//defer cmd.Wait()
	}
	wg.Wait()
}
