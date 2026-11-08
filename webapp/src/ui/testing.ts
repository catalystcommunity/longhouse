import { getQueriesForElement } from '@testing-library/dom';
import { mount, type Content } from './dom';
import { resetLocation } from './router';
export { fireEvent, waitFor } from '@testing-library/dom';
const mounted = new Set<() => void>();
export function render(view: () => Content) {
  const container = document.createElement('div'); document.body.append(container);
  const dispose = mount(view, container);
  const unmount = () => { dispose(); container.remove(); mounted.delete(unmount); };
  mounted.add(unmount);
  return { container, unmount, ...getQueriesForElement(container) };
}
export function cleanup() { for (const remove of mounted) remove(); resetLocation(); }
export function createTestHistory() {
  return { set: ({value}: {value: string}) => resetLocation(value), get: () => location.pathname + location.search + location.hash };
}
