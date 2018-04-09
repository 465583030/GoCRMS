import performance_test as pf
import sys


def main():
    gocrms_server_count = int(sys.argv[1])
    gocrms_count_per_lsf_server = int(sys.argv[2])
    etcd_host_port = sys.argv[3]
    pf.start_multiple_servers_by_lsf(etcd_host_port, gocrms_server_count, gocrms_count_per_lsf_server)


if __name__ == "__main__":
    main()
