import { For, Show, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { auditClient, trashClient } from "~/data/clients";
import { hasRole, useCurrentHouseId } from "~/stores/auth";
import type { AuditEntry, TrashItem } from "@longhouse/client";

// Admin-only "Activity & Trash": the per-house audit log (who did what) and the
// trash bin (soft-deleted items an admin can restore or purge now). Both calls
// are admin-gated server-side too; this page just hides the controls for
// non-admins. Theming is via CSS vars, so dark mode comes for free.

const when = (ts: string): string => {
  const d = new Date(ts);
  return isNaN(d.getTime()) ? ts : d.toLocaleString();
};

const actionTone = (action: string): string => {
  switch (action) {
    case "delete":
    case "purge":
    case "login_failed":
      return "var(--rust)";
    case "restore":
    case "create":
    case "login":
      return "var(--grass-4)";
    default:
      return "var(--ocean-2)";
  }
};

export const ActivityTrashPage = () => {
  const houseId = useCurrentHouseId();
  const isAdmin = () => hasRole("admin");

  const [audit, { refetch: refetchAudit }] = createResource(
    () => (isAdmin() ? houseId() : null),
    async (h) => (await auditClient.queryAudit({ houseId: h, limit: 100 })).entries,
  );
  const [trash, { refetch: refetchTrash }] = createResource(
    () => (isAdmin() ? houseId() : null),
    async (h) => (await trashClient.listTrash({ houseId: h })).items,
  );

  const [busy, setBusy] = createSignal<string | null>(null);

  const restore = async (item: TrashItem) => {
    const h = houseId();
    if (!h) return;
    setBusy(item.resourceId);
    try {
      await trashClient.restore({
        houseId: h,
        resourceType: item.resourceType,
        resourceId: item.resourceId,
      });
      await Promise.all([refetchTrash(), refetchAudit()]);
    } finally {
      setBusy(null);
    }
  };

  const purge = async (item: TrashItem) => {
    const h = houseId();
    if (!h) return;
    if (!window.confirm(`Permanently delete this ${item.resourceType}? This cannot be undone.`)) return;
    setBusy(item.resourceId);
    try {
      await trashClient.purge({
        houseId: h,
        resourceType: item.resourceType,
        resourceId: item.resourceId,
      });
      await Promise.all([refetchTrash(), refetchAudit()]);
    } finally {
      setBusy(null);
    }
  };

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Activity &amp; Trash <em>house log</em></h2>
        <p class="lead">Who did what, and a recoverable bin for deleted items.</p>
      </div>

      <Show
        when={isAdmin()}
        fallback={
          <section class="card reveal" style="margin-top:0;padding:40px 24px;text-align:center">
            <h3 style="color:var(--grass-4);margin:0 0 6px">Admins only</h3>
            <p style="color:var(--ink-mute);margin:0">
              The activity log and trash bin are visible to house admins.
            </p>
          </section>
        }
      >
        {/* ---- Trash bin ---- */}
        <section class="card reveal d1" style="margin-top:0;padding:8px 22px 18px">
          <div class="card-hd"><h3>Trash</h3></div>
          <Show
            when={!trash.loading}
            fallback={<p style="padding:18px;color:var(--ink-mute)">Loading…</p>}
          >
            <Show
              when={(trash() ?? []).length > 0}
              fallback={<p style="padding:18px;color:var(--ink-mute)">Nothing in the trash.</p>}
            >
              <For each={trash()!}>
                {(item) => (
                  <div style="padding:12px 0;border-bottom:1px solid var(--line);display:flex;align-items:center;gap:12px">
                    <span
                      style="font-size:11px;text-transform:uppercase;letter-spacing:.04em;color:var(--ink-mute);min-width:78px"
                    >
                      {item.resourceType}
                    </span>
                    <div style="flex:1;min-width:0">
                      <div style="color:var(--ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
                        {item.title || item.resourceId}
                      </div>
                      <div style="font-size:12px;color:var(--ink-mute)">
                        deleted {when(item.deletedAt)}
                      </div>
                    </div>
                    <button
                      class="btn"
                      disabled={busy() === item.resourceId}
                      onClick={() => restore(item)}
                    >
                      Restore
                    </button>
                    <button
                      class="btn"
                      style="color:var(--rust)"
                      disabled={busy() === item.resourceId}
                      onClick={() => purge(item)}
                    >
                      Purge
                    </button>
                  </div>
                )}
              </For>
            </Show>
          </Show>
        </section>

        {/* ---- Audit log ---- */}
        <section class="card reveal d2" style="padding:8px 22px 18px">
          <div class="card-hd"><h3>Activity log</h3></div>
          <Show
            when={!audit.loading}
            fallback={<p style="padding:18px;color:var(--ink-mute)">Loading…</p>}
          >
            <Show
              when={(audit() ?? []).length > 0}
              fallback={<p style="padding:18px;color:var(--ink-mute)">No activity recorded yet.</p>}
            >
              <For each={audit()!}>{(e) => <AuditRow e={e} />}</For>
            </Show>
          </Show>
        </section>
      </Show>
    </AuthGate>
  );
};

const AuditRow = (props: { e: AuditEntry }) => {
  const e = props.e;
  const who = e.actorUserId ? `${e.actorUserId}@${e.actorDomain}` : "—";
  const target = e.resourceType ? `${e.resourceType}${e.resourceId ? " " + e.resourceId : ""}` : "";
  return (
    <div style="padding:11px 0;border-bottom:1px solid var(--line);display:flex;align-items:baseline;gap:12px">
      <span
        style={`font-size:11px;text-transform:uppercase;letter-spacing:.04em;font-weight:600;min-width:92px;color:${actionTone(e.action)}`}
      >
        {e.action}
      </span>
      <div style="flex:1;min-width:0">
        <div style="color:var(--ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
          {who}
          <Show when={target}>
            <span style="color:var(--ink-mute)"> · {target}</span>
          </Show>
          <Show when={e.outcome && e.outcome !== "ok"}>
            <span style="color:var(--rust)"> · {e.outcome}</span>
          </Show>
        </div>
        <div style="font-size:12px;color:var(--ink-mute)">
          {e.serviceName}.{e.method}
        </div>
      </div>
      <span style="font-size:12px;color:var(--ink-mute);white-space:nowrap">{when(e.createdAt)}</span>
    </div>
  );
};
