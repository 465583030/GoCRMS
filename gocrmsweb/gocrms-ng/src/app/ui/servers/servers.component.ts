import { Component, OnInit } from '@angular/core';
import { CwmService } from '../../service/cwm.service';
import { Observable } from 'rxjs/Observable';
import { Subscription } from 'rxjs/Subscription';
import { Server } from '../../model/server';
import { Worker } from '../../model/worker';
import { JobDetail } from '../../model/job.detail';

@Component({
  selector: 'app-servers',
  templateUrl: './servers.component.html',
  styleUrls: ['./servers.component.css']
})
export class ServersComponent implements OnInit {

  servers: Server[] = [];
  selectedServer = new Set<string>();
  serverWorkersMap: {[key: string]: Worker[]} = {};
  serverJobsMap: {[key: string]: JobDetail[]} = {};
  selectedServerForWorkers?: Server;
  selectedServerForJobs?: Server;
  jobCommand = '';
  autoRefresh = false;
  intervalSecondForRefresh = 5;
  refreshTimer$?: Subscription;

  constructor(private cwmService: CwmService) {
  }

  ngOnInit() {
    this.onRefresh();
  }

  private serverHostPort(server: Server): string {
    return `${server.host}:${server.port}`;
  }

  private reachableServers(): Server[] {
    return this.servers.filter(server => server.isReachable);
  }

  allSelected(): boolean {
    return this.selectedServer.size === this.reachableServers().length;
  }

  onSelectServer(server: Server, event) {
    if (event.target.checked) {
      this.selectedServer.add(this.serverHostPort(server));
    } else {
      this.selectedServer.delete(this.serverHostPort(server));
    }
  }

  isServerSelected(server: Server): boolean {
    return this.selectedServer.has(this.serverHostPort(server));
  }

  onSelectAllServers(event) {
    this.selectedServer.clear();
    if (event.target.checked) {
      this.reachableServers().map(server => this.serverHostPort(server))
      .forEach(s => this.selectedServer.add(s));
    }
  }

  onRunJob() {
    const job = this.jobCommand.split(' ').filter(str => str.length > 0);
    this.cwmService.runJob(job, this.selectedServers());
  }

  onShutdown() {
    this.cwmService.shutdown(this.selectedServers());
  }

  private selectedServers(): Server[] {
    return this.servers.filter(server => this.selectedServer.has(this.serverHostPort(server)));
  }

  workerCount(server: Server): number {
    const hostPort = this.serverHostPort(server);
    const workers = this.serverWorkersMap[hostPort];
    return workers ? workers.length : 0;
  }

  jobCount(server: Server): number {
    const hostPort = this.serverHostPort(server);
    const jobs = this.serverJobsMap[hostPort];
    return jobs ? jobs.length : 0;
  }

  onClickWorkerCount(server: Server) {
    this.selectedServerForJobs = undefined;
    this.selectedServerForWorkers = server;
  }

  onClickJobCount(server: Server) {
    this.selectedServerForWorkers = undefined;
    this.selectedServerForJobs = server;
  }

  workersToShow(): Worker[] {
    if (!this.selectedServerForWorkers) {
      return [];
    }
    return this.serverWorkersMap[this.serverHostPort(this.selectedServerForWorkers)];
  }

  jobsToShow(): JobDetail[] {
    if (!this.selectedServerForJobs) {
      return [];
    }
    return this.serverJobsMap[this.serverHostPort(this.selectedServerForJobs)];
  }

  onRefresh() {
    console.log('refresh');
    this.cwmService.servers().subscribe(servers => {
      this.servers = servers;
      this.servers.filter(server => !server.isReachable)
      .forEach(server => {
        const serverHostPort = this.serverHostPort(server);
        if (this.selectedServer.has(serverHostPort)) {
          this.selectedServer.delete(serverHostPort);
        }
      });
      this.reachableServers().forEach(server => {
        this.cwmService.workers(server.host, server.port)
        .subscribe(workers => {
          this.serverWorkersMap[this.serverHostPort(server)] = workers;
        });
        this.cwmService.jobs(server.host, server.port)
        .subscribe(jobs => {
          this.serverJobsMap[this.serverHostPort(server)] = jobs;
        });
      });
    });
  }

  onClickAutoRefresh(event) {
    if (event.target.checked) {
      this.startTimer();
    } else {
      this.stopTimer();
    }
  }

  private startTimer() {
    this.stopTimer();
    if (this.intervalSecondForRefresh > 0) {
      console.log(`startTimer per ${this.intervalSecondForRefresh} seconds`);
      this.refreshTimer$ = Observable.interval(this.intervalSecondForRefresh * 1000)
      .subscribe(evt => this.onRefresh());
    }
  }

  private stopTimer() {
    console.log('stopTimer');
    if (this.refreshTimer$ && !this.refreshTimer$.closed) {
      this.refreshTimer$.unsubscribe();
    }
  }

  onIntervalSecondForRefreshChanged(event) {
    this.startTimer();
  }
}
