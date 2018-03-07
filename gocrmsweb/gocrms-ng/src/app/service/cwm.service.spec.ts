import { TestBed, inject } from '@angular/core/testing';

import { CwmService } from './cwm.service';

describe('CwmService', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [CwmService]
    });
  });

  it('should be created', inject([CwmService], (service: CwmService) => {
    expect(service).toBeTruthy();
  }));
});
