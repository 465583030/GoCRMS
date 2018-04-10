import matplotlib.pyplot as plt
import numpy as np
import re

summary = """
total 500 running on 50 servers
average create to running: 0:00:00.004034
total start cost (last running - first create on client) 0:00:01.021000

total 1000 running on 100 servers
average create to running: 0:00:00.004847
total start cost (last running - first create on client) 0:00:02.287000

total 1000 running on 200 servers
average create to running: 0:00:00.014748
total start cost (last running - first create on client) 0:00:05.239000

total 1000 running on 200 servers
average create to running: 0:00:00.018829
total start cost (last running - first create on client) 0:00:04.099000

total 2000 running on 200 servers
average create to running: 0:00:00.022244
total start cost (last running - first create on client) 0:00:05.301000

total 4000 running on 245 servers
average create to running: 0:00:00.025385
total start cost (last running - first create on client) 0:00:10.533000

total 4000 running on 400 servers
average create to running: 0:00:00.009009
total start cost (last running - first create on client) 0:00:11.433000

total 4000 running on 400 servers
average create to running: 0:00:00.005811
total start cost (last running - first create on client) 0:00:09.805000

total 8000 running on 256 servers
average create to running: 0:00:00.011806
total start cost (last running - first create on client) 0:00:20.558000

total 8000 running on 800 servers
average create to running: 0:00:00.042673
total start cost (last running - first create on client) 0:00:25.284000

total 8000 running on 799 servers
average create to running: 0:00:00.265671
total start cost (last running - first create on client) 0:00:22.899000

total 16000 running on 800 servers
average create to running: 0:00:00.099871
total start cost (last running - first create on client) 0:00:48.272000

total 10000 running on 1000 servers
average create to running: 0:00:00.427678
total start cost (last running - first create on client) 0:00:29.359000

total 11000 running on 1100 servers
average create to running: 0:00:00.145534
total start cost (last running - first create on client) 0:00:34.711000

total 12000 running on 1200 servers
average create to running: 0:00:00.108644
total start cost (last running - first create on client) 0:00:36.305000

total 15000 running on 1500 servers
average create to running: 0:00:00.044156
total start cost (last running - first create on client) 0:00:39.068000

total 20000 running on 2000 servers
average create to running: 0:00:00.323321
total start cost (last running - first create on client) 0:00:58.539000

total 20000 running on 2000 servers
average create to running: 0:00:01.533926
total start cost (last running - first create on client) 0:00:58.535000

total 30000 running on 2000 servers
average create to running: 0:00:00.603885
total start cost (last running - first create on client) 0:01:22.229000
"""

class Summary:
    def __init__(self, jobs, servers, avg_cost, total_cost):
        self.jobs = jobs
        self.servers = servers
        self.avg_cost = avg_cost      # unit: us
        self.total_cost = total_cost  # unit: us


def to_us(h, m, s, us):
    return us + 1000000 * (s + 60 * (m + 60 * h))


def parse():
    summaries = []
    pattern = re.compile(r"""total (\d+) running on (\d+) servers
average create to running: (\d+):(\d+):(\d+)\.(\d+)
total start cost \(last running - first create on client\) (\d+):(\d+):(\d+)\.(\d+)""")
    for s in summary.strip().split("\n\n"):
        m = pattern.match(s)
        if m:
            smr = Summary(int(m.group(1)), int(m.group(2)),
                          to_us(int(m.group(3)), int(m.group(4)), int(m.group(5)), int(m.group(6))),
                          to_us(int(m.group(7)), int(m.group(8)), int(m.group(9)), int(m.group(10))))
            summaries.append(smr)
    return summaries


def line_fit(x, y):
    line = np.polyfit(x, y, 1)
    formula = "y = %f x + %f" % (line[0], line[1])
    y_line = np.polyval(line, x)
    plt.plot(x, y_line, 'r')
    return formula


def plot_total_cost(summaries):
    x = [s.jobs for s in summaries]
    y = [s.total_cost for s in summaries]
    plt.scatter(x, y)

    formula = line_fit(x, y)

    plt.title('job count - total cost (%s)' % formula)
    plt.xlabel('job count')
    plt.ylabel('total cost (last running - first create) (us)')


def plot_avg_cost(summaries):
    x = [s.jobs for s in summaries]
    y = [s.avg_cost for s in summaries]
    plt.scatter(x, y)

    formula = line_fit(x, y)

    plt.title('job count - average cost (%s)' % formula)
    plt.xlabel('job count')
    plt.ylabel('average cost (running - create) (us)')


def plot(summaries):
    plt.subplot(211)
    plot_total_cost(summaries)

    plt.subplot(212)
    plot_avg_cost(summaries)


if __name__ == "__main__":
    summaries = parse()
    plot_total_cost(summaries)
    plt.show()
