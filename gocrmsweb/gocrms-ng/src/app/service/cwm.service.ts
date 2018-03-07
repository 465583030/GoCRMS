import { Injectable } from '@angular/core';
import { Headers, Http } from '@angular/http';
import { Observable } from 'rxjs/Observable';
import { ConfigService } from './config.service';
import { Server } from '../model/server';
import { Worker } from '../model/worker';
import { JobDetail } from '../model/job.detail';

@Injectable()
export class CwmService {

  private readonly headers = new Headers({
    'Content-Type': 'application/json'
  });

  constructor(private http: Http, private config: ConfigService) { }

  servers(): Observable<Server[]> {
    return this.get('servers');
  }

  workers(host: string, port: number): Observable<Worker[]> {
    return this.get('workers', {
      host: host,
      port: port
    });
  }

  jobs(host: string, port: number): Observable<JobDetail[]> {
    return this.get('jobs', {
      host: host,
      port: port
    });
  }

  runJob(job: string[], servers: Server[]) {
    if (job.length === 0 || servers.length === 0) {
      return;
    }
    const serverHostPorts = servers.map(server => `${server.host}:${server.port}`);
    console.log(`run job ${job} on servers ${serverHostPorts}`);
    this.post('run', {
      job: job,
      hostPorts: serverHostPorts
    }).subscribe();
  }

  shutdown(servers: Server[]) {
    if (servers.length === 0) {
      return;
    }
    const hostPorts = servers.map(server => `${server.host}:${server.port}`);
    console.log(`shutdown ${hostPorts}`);
    this.post('shutdown', hostPorts)
    .subscribe();
  }

  private post(action: string, data: any = {}, params: any = {}): Observable<any> {
    return this.http.post(this.config.uri(action), data, {
      headers: this.headers,
      params: params
    });
  }

  private get(action: string, params: any = {}): Observable<any> {
    return this.http.get(this.config.uri(action), {
      headers: this.headers,
      params: params
    })
    .map(res => res.json());
  }
}
