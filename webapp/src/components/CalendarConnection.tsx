import { For, Show, createResource, createSignal, createEffect, onCleanup } from "solid-js";
import type { Event as ApiEvent } from "@longhouse/client";
import { calendarClient, eventClient } from "~/data/clients";
import { hasRole } from "~/stores/auth";

export const CalendarConnection = (props: {
  houseId: string;
  refresh: number;
  onEdit: (event: ApiEvent) => void;
  onEnabled: (enabled: boolean) => void;
}) => {
  const [status, { refetch }] = createResource(() => [props.houseId, props.refresh] as const, async ([id]) => {
    const result = await calendarClient.getStatus(id);
    props.onEnabled(result.enabled);
    return result;
  });
  createEffect(() => {
    const current = status();
    if (current?.enabled && (!current.ready || current.syncPendingSince)) {
      const timer = window.setInterval(() => { void refetch(); }, 5000);
      onCleanup(() => window.clearInterval(timer));
    }
  });
  const [local] = createResource(() => [props.houseId, props.refresh] as const,
    ([id]) => eventClient.listLocalEvents(id));
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [preview, setPreview] = createSignal<Awaited<ReturnType<typeof calendarClient.previewMigration>>>();
  const [trash, setTrash] = createSignal<Awaited<ReturnType<typeof calendarClient.listTrash>>>();
  const run = async (action: () => Promise<unknown>) => {
    setBusy(true); setError("");
    try { await action(); await refetch(); } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  const connect = () => run(async () => {
    const result = await calendarClient.beginConnect(props.houseId);
    sessionStorage.setItem("calendar-connect-house", props.houseId);
    window.location.assign(result.url);
  });
  return <section aria-label="Calendar connection and assignments" style="margin-bottom:16px">
    <Show when={status.error || local.error || error()}>
      <p role="alert">{error() || String(status.error || local.error)}</p>
    </Show>
    <Show when={status()?.available}>
      <Show when={status()?.enabled} fallback={
        <Show when={hasRole("admin")}>
          <button class="btn btn-ghost" disabled={busy()} onClick={() => run(async () => setPreview(await calendarClient.previewMigration(props.houseId)))}>Review catalystcal setup</button>
          <Show when={preview()}>{rows => <div>
            <p>Existing events stay in Longhouse until their owners review and accept them. Stored recurring occurrences need separate review.</p>
            <p>{rows().length} existing events need review.</p>
            <details><summary>Migration report</summary><ul><For each={rows()}>{row => <li>{row.title}: {row.reason}</li>}</For></ul></details>
            <button class="btn btn-primary" disabled={busy()} onClick={() => run(async () => {
              await calendarClient.configureCalendar({ houseId: props.houseId, enabled: true });
              setPreview(undefined);
            })}>Enable catalystcal for this house</button>
          </div>}</Show>
        </Show>
      }>
        <p>{status()?.ready ? "This house uses catalystcal." : "Calendar setup is pending."}</p>
        <Show when={status()?.lastError}><p role="alert">{status()?.lastError}</p></Show>
        <Show when={status()?.syncPendingSince}><p>Calendar access changes are waiting to sync.</p></Show>
        <Show when={status()?.ready}>
          <Show when={status()?.connected} fallback={<button class="btn btn-primary" disabled={busy()} onClick={connect}>Connect your calendar</button>}>
            <button class="btn btn-ghost" disabled={busy()} onClick={() => run(() => calendarClient.disconnect(props.houseId))}>Disconnect calendar</button>
            <button class="btn btn-ghost" disabled={busy()} onClick={() => run(async () => setTrash(await calendarClient.listTrash(props.houseId)))}>Calendar trash</button>
          </Show>
        </Show>
      </Show>
    </Show>
    <Show when={trash()}><ul><For each={trash()}>{entry => <li>{entry.event.title} <button disabled={busy()} onClick={() => run(async () => {
      const restored = await calendarClient.restoreEvent({ houseId: props.houseId, trashId: entry.trashId });
      setTrash(await calendarClient.listTrash(props.houseId)); props.onEdit(restored);
    })}>Restore</button></li>}</For></ul></Show>
    <Show when={(local()?.length ?? 0) > 0}>
      <h3>Assignments and local events</h3>
      <p>These items stay in Longhouse. Review an assignment before you accept it.</p>
      <ul><For each={local()}>{event => <li>
        <button class="btn btn-ghost" onClick={() => props.onEdit(event)}>{event.title}</button>
        <span>{event.editPendingMemberId ? "Calendar edit needs recovery" : event.calendarUnbound ? "Occurrence no longer exists" : event.calendarState === "publishing" ? "Publication needs recovery" : event.calendarState === "remote" ? "Published event unavailable" : event.calendarState === "declined" ? "Declined" : event.startsAt ? "Needs review" : "Unscheduled"}</span>
      </li>}</For></ul>
    </Show>
  </section>;
};
