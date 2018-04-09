import performance_test as pf
import sys


def main():
    name = sys.argv[1]
    gocrms_server_count = int(sys.argv[2])
    gocrms_count_per_lsf_server = int(sys.argv[3])
    etcd_host_port = sys.argv[4]
    pf.start_multiple_servers_by_lsf(name, etcd_host_port, gocrms_server_count, gocrms_count_per_lsf_server)


if __name__ == "__main__":
    main()
