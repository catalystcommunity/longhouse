import { DateTimePicker } from '@catalystcommunity/date-time-picker';

// Keep the date value as a local calendar date. The Longhouse date/time
// component combines it with the chosen clock time before it sends an instant.
export class LonghouseDateField extends DateTimePicker {}
customElements.define('longhouse-date-field', LonghouseDateField);
declare global {
  interface HTMLElementTagNameMap { 'longhouse-date-field': LonghouseDateField }
}
