import { Pipe, PipeTransform } from '@angular/core';
import { DateTime } from '../utils/datetime';

@Pipe({
  name: 'datetime'
})
export class DatetimePipe implements PipeTransform {

  transform(v: DateTime, args?: any): any {
    return '';
    // if (v.year === 999999999) {
    //   return '';
    // }
    // return `${v.year}-${v.monthValue}-${v.dayOfMonth} ${v.hour}:${v.minute}:${v.second}.${v.nano / 1000000}`;
  }

}
