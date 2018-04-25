package crmsd

import (
	"github.com/WenzheLiu/GoCRMS/crmsd/cmd"
	"github.com/WenzheLiu/GoCRMS/gocrms"
	"github.com/coreos/etcd/clientv3"
	"log"
)

func RunServer(gf *cmd.GlobalFlags) {
	crms, err := gocrms.NewCrms(clientv3.Config{
		Endpoints: gf.Endpoints,
		DialTimeout: gf.DialTimeout,
	}, gf.RequestTimeout)
	if err != nil {
		log.Fatalln(err)
	}
	defer crms.Close()

	server := gocrms.Server{
		Name: gf.Name,
		SlotCount: gf.SlotCount,
	}
	crms.UpdateServer()
}
