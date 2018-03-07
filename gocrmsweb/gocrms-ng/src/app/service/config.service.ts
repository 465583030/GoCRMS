import { Injectable } from '@angular/core';

@Injectable()
export class ConfigService {

    private uriBase = 'api'; // use for product
    // private uriBase = 'http://localhost:8080/api'; // use for dev independently (actual server)
    // private uriBase = 'http://localhost:8090'; // use for dev independently (json server)

    uri(relativeUrl: string): string {
        if (this.uriBase === '') {
            return relativeUrl;
        } else {
            return this.uriBase + '/' + relativeUrl;
        }
    }
}
