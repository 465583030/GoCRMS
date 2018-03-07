import {Routes} from '@angular/router';
import { ServersComponent } from './ui/servers/servers.component';
import { WorkersComponent } from './ui/workers/workers.component';
import { JobsComponent } from './ui/jobs/jobs.component';

export const AppRoutes: Routes = [
  {path: '', redirectTo: 'servers', pathMatch: 'full'},
  {path: 'servers', component: ServersComponent}
];
