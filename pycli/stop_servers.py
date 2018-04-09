import performance_test as pf
import sys
import crmscli


def main():
    etcd_host_port = sys.argv[1]
    with crmscli.CrmsCli(etcd_host_port) as crms:
        pf.stop_servers(crms)


if __name__ == "__main__":
    main()
