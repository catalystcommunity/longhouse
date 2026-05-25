import { Show, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { authClient, houseClient } from "~/data/clients";
import { hasRole, useCurrentHouse, useCurrentHouseId } from "~/stores/auth";
import { finishLogin, loadHouses } from "~/lib/session";

export const SettingsPage = () => {
  const houseId = useCurrentHouseId();
  const currentHouse = useCurrentHouse();
  const isAdmin = () => hasRole("admin");

  const [renameValue, setRenameValue] = createSignal("");
  const [renameBusy, setRenameBusy] = createSignal(false);
  const [renameErr, setRenameErr] = createSignal<string | null>(null);

  const [newHouseName, setNewHouseName] = createSignal("");
  const [newHouseDesc, setNewHouseDesc] = createSignal("");
  const [createBusy, setCreateBusy] = createSignal(false);
  const [createErr, setCreateErr] = createSignal<string | null>(null);
  const [createMsg, setCreateMsg] = createSignal<string | null>(null);

  const submitRename = async (e: SubmitEvent) => {
    e.preventDefault();
    const id = houseId();
    if (!id || !renameValue().trim() || renameBusy()) return;
    setRenameBusy(true);
    setRenameErr(null);
    try {
      await houseClient.updateHouse({
        houseId: id,
        name: renameValue().trim(),
        createdAt: "",
        updatedAt: "",
      } as any);
      await loadHouses();
      setRenameValue("");
    } catch (e2) {
      setRenameErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setRenameBusy(false);
    }
  };

  const submitCreate = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!newHouseName().trim() || createBusy()) return;
    setCreateBusy(true);
    setCreateErr(null);
    setCreateMsg(null);
    try {
      const created = await houseClient.createHouse({
        houseId: "",
        name: newHouseName().trim(),
        description: newHouseDesc().trim() || undefined,
        createdAt: "",
        updatedAt: "",
      } as any);
      // The new house's admin role for the caller exists server-side, but
      // the current bearer was minted before we were a member. Refresh
      // the bearer so the new house surfaces in /me + the switcher.
      const refreshed = await authClient.refresh({});
      await finishLogin(refreshed);
      setCreateMsg(`Created "${created.name}". It's now in your house switcher.`);
      setNewHouseName("");
      setNewHouseDesc("");
    } catch (e2) {
      setCreateErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setCreateBusy(false);
    }
  };

  return (
    <AuthGate>
      <div class="section-hd reveal">
        <h2>Settings <em>the house and you</em></h2>
        <p class="lead">Rename the current house, or create a new one.</p>
      </div>

      <section style="margin-top:16px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h3 style="margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          Current house
        </h3>
        <Show when={currentHouse()}>
          {(h) => (
            <p style="margin:0 0 14px;color:var(--ink-mute)">
              You're in <b>{h().name}</b> as {h().roles?.join(", ") || "a member"}.
            </p>
          )}
        </Show>
        <Show
          when={isAdmin()}
          fallback={<p style="color:var(--ink-mute);font-size:13px;font-style:italic">Only house admins can rename.</p>}
        >
          <form onSubmit={submitRename} style="display:flex;gap:10px;flex-wrap:wrap;align-items:end">
            <label style="display:flex;flex-direction:column;gap:4px;flex:1 1 240px">
              <span style="font-size:12px;color:var(--ink-mute)">Rename to</span>
              <input
                type="text"
                value={renameValue()}
                onInput={(e) => setRenameValue(e.currentTarget.value)}
                placeholder={currentHouse()?.name ?? ""}
                style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
              />
            </label>
            <button class="btn btn-primary" type="submit" disabled={renameBusy() || !renameValue().trim()}>
              {renameBusy() ? "Saving…" : "Rename"}
            </button>
            <Show when={renameErr()}>
              {(m) => <span style="color:var(--rust);font-size:13px;flex:1 1 100%">{m()}</span>}
            </Show>
          </form>
        </Show>
      </section>

      <section style="margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h3 style="margin:0 0 6px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
          New house
        </h3>
        <p style="margin:0 0 14px;color:var(--ink-mute);font-size:14px">
          Houses are independent — you'll be the founding admin of the new one. Your bearer is refreshed so the house appears in your switcher immediately.
        </p>
        <form onSubmit={submitCreate} style="display:grid;grid-template-columns:1fr 2fr auto;gap:10px;align-items:end">
          <label style="display:flex;flex-direction:column;gap:4px">
            <span style="font-size:12px;color:var(--ink-mute)">Name</span>
            <input
              type="text"
              value={newHouseName()}
              onInput={(e) => setNewHouseName(e.currentTarget.value)}
              required
              style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
            />
          </label>
          <label style="display:flex;flex-direction:column;gap:4px">
            <span style="font-size:12px;color:var(--ink-mute)">Description (optional)</span>
            <input
              type="text"
              value={newHouseDesc()}
              onInput={(e) => setNewHouseDesc(e.currentTarget.value)}
              style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px"
            />
          </label>
          <button class="btn btn-primary" type="submit" disabled={createBusy() || !newHouseName().trim()}>
            {createBusy() ? "Creating…" : "Create house"}
          </button>
          <Show when={createErr()}>
            {(m) => <span style="color:var(--rust);font-size:13px;grid-column:1/-1">{m()}</span>}
          </Show>
          <Show when={createMsg()}>
            {(m) => <span style="color:var(--grass-4);font-size:13px;grid-column:1/-1">{m()}</span>}
          </Show>
        </form>
      </section>
    </AuthGate>
  );
};
