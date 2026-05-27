import { Show, createSignal } from "solid-js";
import { Portal } from "solid-js/web";
import { bugClient } from "~/data/clients";
import { useCurrentHouseId } from "~/stores/auth";
import { bugReportsEnabled } from "~/stores/settings";
import { Bug } from "./Icons";

/**
 * Icon-button + modal for the in-app bug reporter. Only renders when the
 * per-house `bug_reports_enabled` setting is on; the modal collects a
 * title and (optional) description and posts to BugService.ReportBug.
 * The server attributes the reporter in the description itself — the SPA
 * doesn't add anything visible to the reporter on top.
 */
export const BugReportButton = () => {
  const houseId = useCurrentHouseId();
  const [open, setOpen] = createSignal(false);
  const [title, setTitle] = createSignal("");
  const [description, setDescription] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal<string | null>(null);
  const [done, setDone] = createSignal(false);

  const reset = () => {
    setTitle("");
    setDescription("");
    setErr(null);
    setDone(false);
  };

  const close = () => {
    setOpen(false);
    reset();
  };

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    const hid = houseId();
    if (!hid || !title().trim() || busy()) return;
    setBusy(true);
    setErr(null);
    try {
      await bugClient.reportBug({
        houseId: hid,
        title: title().trim(),
        description: description().trim() || undefined,
      });
      setDone(true);
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Show when={bugReportsEnabled()}>
      <button
        class="icon-btn"
        onClick={() => setOpen(true)}
        aria-label="Report a bug"
        title="Report a bug"
      >
        <Bug />
      </button>

      <Show when={open()}>
        <Portal>
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Report a bug"
            style="position:fixed;inset:0;background:rgba(0,0,0,0.45);display:flex;align-items:center;justify-content:center;z-index:1000"
            onClick={(e) => {
              if (e.target === e.currentTarget) close();
            }}
          >
            <div style="background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-high);padding:22px 24px;width:min(520px,calc(100% - 32px));max-height:calc(100% - 32px);overflow:auto">
              <h2 style="margin:0 0 4px;font-family:var(--display);font-size:22px;color:var(--grass-4)">
                Report a bug
              </h2>
              <p style="margin:0 0 16px;color:var(--ink-mute);font-size:13px">
                Tell us what went wrong. A house admin will see this as a task
                in the bug-fixes project.
              </p>

              <Show
                when={!done()}
                fallback={
                  <div>
                    <p style="margin:0 0 16px;color:var(--grass-4)">
                      Thanks — your report was submitted.
                    </p>
                    <div style="display:flex;justify-content:flex-end;gap:8px">
                      <button class="btn btn-ghost" type="button" onClick={close}>
                        Close
                      </button>
                    </div>
                  </div>
                }
              >
                <form onSubmit={submit} style="display:flex;flex-direction:column;gap:12px">
                  <label style="display:flex;flex-direction:column;gap:4px">
                    <span style="font-size:12px;color:var(--ink-mute)">Title</span>
                    <input
                      type="text"
                      value={title()}
                      onInput={(e) => setTitle(e.currentTarget.value)}
                      required
                      autofocus
                      maxLength={512}
                      placeholder="Short summary"
                      style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
                    />
                  </label>
                  <label style="display:flex;flex-direction:column;gap:4px">
                    <span style="font-size:12px;color:var(--ink-mute)">Description (optional)</span>
                    <textarea
                      value={description()}
                      onInput={(e) => setDescription(e.currentTarget.value)}
                      rows={5}
                      placeholder="What did you expect? What actually happened?"
                      style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;font-family:inherit;resize:vertical"
                    />
                  </label>
                  <Show when={err()}>
                    {(m) => <span style="color:var(--rust);font-size:13px">{m()}</span>}
                  </Show>
                  <div style="display:flex;justify-content:flex-end;gap:8px;margin-top:4px">
                    <button class="btn btn-ghost" type="button" onClick={close} disabled={busy()}>
                      Cancel
                    </button>
                    <button class="btn btn-primary" type="submit" disabled={busy() || !title().trim()}>
                      {busy() ? "Submitting…" : "Submit report"}
                    </button>
                  </div>
                </form>
              </Show>
            </div>
          </div>
        </Portal>
      </Show>
    </Show>
  );
};
