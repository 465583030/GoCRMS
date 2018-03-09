import { Component, OnInit, Input } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { CwmService } from '../../service/cwm.service';
import { Observable } from 'rxjs/Observable';
import { JobDetail } from '../../model/job.detail';
import { Server } from '../../model/server';

@Component({
  selector: 'app-jobs',
  templateUrl: './jobs.component.html',
  styleUrls: ['./jobs.component.css']
})
export class JobsComponent implements OnInit {

  @Input() server: Server;
  @Input() jobs: JobDetail[];

  ngOnInit() {
  }

  jobClass(jobDetail: JobDetail): any {
    return {
      'done': jobDetail.status === 'DONE',
      'fail': jobDetail.status === 'FAIL',
      'running': jobDetail.status === 'RUNNING'
    };
  }

  output(jobDetail: JobDetail): string {
    return jobDetail.stdouterr; // .replace(/\n/g, '<br>'); //TODO: <br> also cannot work
  }
}
