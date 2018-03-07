export interface DateTime {
  year: number;

  month: string;
  monthValue: number;

  dayOfMonth: number;
  dayOfWeek: string;
  dayOfYear: number;

  hour: number;
  minute: number;
  second: number;
  nano: number;

  chronology: {
    id: string;
    calendarType: string;
  };
}
