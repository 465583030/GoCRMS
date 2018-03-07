import { Component, OnInit, Input } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { CwmService } from '../../service/cwm.service';
import { Observable } from 'rxjs/Observable';
import { Worker } from '../../model/worker';
import { Server } from '../../model/server';

@Component({
  selector: 'app-workers',
  templateUrl: './workers.component.html',
  styleUrls: ['./workers.component.css']
})
export class WorkersComponent implements OnInit {

  @Input() server: Server;
  @Input() workers: Worker[];

  ngOnInit() {
  }

}
