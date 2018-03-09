import { Job } from './job';

export interface JobDetail {
  job: Job;
  status: string;
  // startTime: any;
  // endTime: any;
  stdouterr: string;
}
