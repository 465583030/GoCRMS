import { Injectable } from '@angular/core';

@Injectable()
export class ConfigService {

    private readonly uriBase = '/api';

    uri(relativeUrl: string): string {
        return this.uriBase + '/' + relativeUrl;
    }
}
