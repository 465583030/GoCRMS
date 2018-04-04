// Author: Wenzhe Liu
// usage (bash command): etcdstarter 3 12379 12380 ~/.gocrms/etcdstarter
// usage (lsf command): bsub -q pwodebug "etcdstarter 3 12379 12380 ~/.gocrms/etcdstarter"
package main

import (
	"os/exec"
	"log"
	"io/ioutil"
	"fmt"
	"flag"
	"os"
	"path"
	"strconv"
	"bytes"
	"io"
	"net"
	"errors"
	"time"
	"context"
	"bufio"
)

type etcdStarter struct {
	name string
	ip string
	cliPort int
	ppPort int
	cluster string // format like: fnode404=http://172.20.1.14:12380,fnode408=http://172.20.1.18:12380
}

// command format:
// etcd --name fnode404 --initial-advertise-peer-urls http://172.20.1.14:12380
// --listen-peer-urls http://172.20.1.14:12380 --listen-client-urls http://172.20.1.14:12379
// --advertise-client-urls http://172.20.1.14:12379 --initial-cluster-token etcd-cluster-gocrms
// --initial-cluster fnode404=http://172.20.1.14:12380,fnode408=http://172.20.1.18:12380,fnode402=http://172.20.1.12:12380
// --initial-cluster-state new
func (etcd *etcdStarter) startEtcd() {
	site := "http://%s:%d"
	cli := fmt.Sprintf(site, etcd.ip, etcd.cliPort)
	pp := fmt.Sprintf(site, etcd.ip, etcd.ppPort)
	cmd := exec.Command("etcd",
		"--name", etcd.name,
		"--initial-advertise-peer-urls", pp,
		"--listen-peer-urls", pp,
		"--listen-client-urls", cli,
		"--advertise-client-urls", cli,
		"--initial-cluster-token", "etcd-cluster-gocrms",
		"--initial-cluster", etcd.cluster,
		"--initial-cluster-state", "new")
	log.Println(cmd.Args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}

type hostIp struct {
	host string
	ip string
}

type etcdRegister struct {
	clusterSize int
	cliPort int
	ppPort int
	regPath string // the file is used for communication in multiple etcd server starter (for --initial-cluster)
	hostIp
}

// return the etcd count that has been registered
func (reg *etcdRegister) register() int {
	os.MkdirAll(path.Dir(reg.regPath), 0775)
	lineCount, err := countFileLines(reg.regPath)
	if err != nil {
		log.Fatalln(err)
	}

	// check already registered or not
	if lineCount > 0 && lineCount < reg.clusterSize && reg.hasRegistered() {
		log.Fatalf("%s (%s) has already registered as etcd host", reg.host, reg.ip)
	}

	// add host, ip as register
	mode := os.O_CREATE | os.O_WRONLY
	if lineCount < reg.clusterSize {
		mode |= os.O_APPEND
	} else {
		mode |= os.O_TRUNC
		lineCount = 0
	}
	regFile, err := os.OpenFile(reg.regPath, mode, 0664)
	if err != nil {
		log.Fatalln("Fail to open file", err)
	}
	defer regFile.Close()
	regFile.WriteString(fmt.Sprintf("%s\t%s\n", reg.host, reg.ip))
	return lineCount + 1
}

// param count: etcd count that has been registered
func (reg *etcdRegister) waitForReady(count int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for count < reg.clusterSize {
		if _, ok := <- watchFileChange(ctx, reg.regPath); !ok {
			log.Fatalln("watch file change is closed without reaching cluster size")
		}
		var err error
		count, err = countFileLines(reg.regPath)
		if err != nil {
			log.Fatalln(err)
		}
	}
}


// assert the file exist
// return string format: fnode404=http://172.20.1.14:12380,fnode408=http://172.20.1.18:12380
func (reg *etcdRegister) parseCluster() (string, error) {
	file, err := os.OpenFile(reg.regPath, os.O_RDONLY, 0664)
	if err != nil {
		return "", err
	}
	defer file.Close()
	buff := bufio.NewReader(file)
	var cluster bytes.Buffer
	hosts := make(map[string]bool, reg.clusterSize * 2)
	ips := make(map[string]bool, reg.clusterSize * 2)
	for i := 0; i < reg.clusterSize; i++ {
		tag, err := buff.ReadBytes('\t')
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		host := string(tag[:len(tag)-1]) // trim the last char \t

		tag, err = buff.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		ip := string(tag[:len(tag)-1]) // trim the last char \n

		// check duplicate
		if hosts[host] {
			continue
		} else {
			hosts[host] = true
		}
		if ips[ip] {
			continue
		} else {
			ips[ip] = true
		}

		// write cluster string
		if i > 0 {
			cluster.WriteRune(',')
		}
		cluster.WriteString(host)
		cluster.WriteString("=http://")
		cluster.WriteString(ip)
		cluster.WriteRune(':')
		cluster.WriteString(strconv.Itoa(reg.ppPort))
	}
	return cluster.String(), nil
}

// assert the file exist and 0 < line count < reg.clusterSize
func (reg *etcdRegister) hasRegistered() bool {
	file, err := os.OpenFile(reg.regPath, os.O_RDONLY, 0664)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()
	buff := bufio.NewReader(file)
	for i := 0; i < reg.clusterSize - 1; i++ {
		host, err := buff.ReadBytes('\t')
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		if reg.host == string(host[:len(host)-1]) {
			return true
		}

		ip, err := buff.ReadBytes('\n')

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		if reg.ip == string(ip[:len(ip)-1]) {
			return true
		}
	}
	return false
}

// cannot use github.com/fsnotify/fsnotify because it cannot watch file event modified by other servers
// so we use timely check
func watchFileChange(ctx context.Context, path string) <-chan os.FileInfo {
	changed := make(chan os.FileInfo)
	go func() {
		defer close(changed)
		initialStat, err := os.Stat(path)
		if err != nil {
			log.Println(err)
			return
		}
		for {
			select {
			case <- ctx.Done():
				return
			default:
			}
			stat, err := os.Stat(path)
			if err != nil {
				log.Println(err)
				return
			}

			if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
				changed <- stat
				initialStat = stat
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return changed
}

func countLines(r io.Reader) (int, error) {
	buf := make([]byte, 1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func countFileLines(path string) (lineCount int, err error) {
	regFile, err := os.OpenFile(path, os.O_RDONLY, 0664)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		} else {
			return -1, err
		}
	}
	defer regFile.Close()
	return countLines(regFile)
}

func readJobs() {
	cmd := exec.Command("bjobs")
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		log.Fatalln(err)
	}
	opBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(opBytes))
}

func getIp() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("Are you connected to the network?")
}

func main() {
	// bsub -R "type==any" -q pwodebug "sleep 300"
	// exec.Command("bsub", "-q", "pwodebug", "sleep 300")
	flag.Parse()
	clusterSize, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		log.Fatalln(err)
	}
	cliPort, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		log.Fatalln(err)
	}
	ppPort, err := strconv.Atoi(flag.Arg(2))
	if err != nil {
		log.Fatalln(err)
	}
	regPath := flag.Arg(3)
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	ip, err := getIp()
	if err != nil {
		log.Fatalln(err)
	}
	reg := etcdRegister{clusterSize, cliPort, ppPort, regPath, hostIp{hostname, ip}}
	count := reg.register()
	reg.waitForReady(count)
	cluster, err := reg.parseCluster()
	if err != nil {
		log.Fatalln(err)
	}
	etcd := etcdStarter{reg.host, reg.ip, reg.cliPort, reg.ppPort, cluster}
	etcd.startEtcd()
}
