import { For, Show, createEffect, createResource, createSignal } from "solid-js";
import { AuthGate } from "~/components/AuthGate";
import { authClient, houseClient, trustedDomainClient } from "~/data/clients";
import { hasRole, useCurrentHouse, useCurrentHouseId } from "~/stores/auth";
import { finishLogin, loadHouses } from "~/lib/session";
import { updateSettings, useSettings } from "~/stores/settings";

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

  // ---- Feature settings (admin only) ---------------------------------
  const settings = useSettings();
  const [bugEnabledDraft, setBugEnabledDraft] = createSignal(false);
  const [bugProjectIdDraft, setBugProjectIdDraft] = createSignal("");
  const [settingsBusy, setSettingsBusy] = createSignal(false);
  const [settingsErr, setSettingsErr] = createSignal<string | null>(null);
  const [settingsMsg, setSettingsMsg] = createSignal<string | null>(null);

  // Keep the draft inputs in sync with the loaded settings — the store
  // refreshes on house switch, and we want the form to reflect that.
  createEffect(() => {
    const s = settings();
    setBugEnabledDraft(s?.bugReportsEnabled === true);
    setBugProjectIdDraft(s?.bugReportsProjectId ?? "");
  });

  // ---- Trusted domains (admin only) -----------------------------------
  // Members listed here auto-join the current house on first sign-in.
  // Backed by TrustedDomainService; the initial admin's linkkeys domain
  // is seeded at boot from LONGHOUSE_INITIAL_ADMIN_DOMAIN, after which
  // this is the only place to add/remove rows.
  const [trustedDomains, { refetch: refetchTrusted }] = createResource(
    () => houseId(),
    async (h) => (h ? trustedDomainClient.listTrustedDomains(h) : []),
  );
  const [newDomain, setNewDomain] = createSignal("");
  const [tdBusy, setTdBusy] = createSignal(false);
  const [tdErr, setTdErr] = createSignal<string | null>(null);

  const addDomain = async (e: SubmitEvent) => {
    e.preventDefault();
    const id = houseId();
    const d = newDomain().trim().toLowerCase();
    if (!id || !d || tdBusy()) return;
    setTdBusy(true);
    setTdErr(null);
    try {
      await trustedDomainClient.addTrustedDomain({
        trustedDomainId: "",
        houseId: id,
        domain: d,
        createdAt: "",
      } as any);
      setNewDomain("");
      await refetchTrusted();
    } catch (e2) {
      setTdErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setTdBusy(false);
    }
  };

  const removeDomain = async (id: string) => {
    setTdBusy(true);
    setTdErr(null);
    try {
      await trustedDomainClient.removeTrustedDomain(id);
      await refetchTrusted();
    } catch (e2) {
      setTdErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setTdBusy(false);
    }
  };

  const submitFeatureSettings = async (e: SubmitEvent) => {
    e.preventDefault();
    const id = houseId();
    if (!id || settingsBusy()) return;
    setSettingsBusy(true);
    setSettingsErr(null);
    setSettingsMsg(null);
    try {
      // Only send fields that actually changed — partial update semantics.
      const cur = settings();
      const patch: { bugReportsEnabled?: boolean; bugReportsProjectId?: string } = {};
      if ((cur?.bugReportsEnabled === true) !== bugEnabledDraft()) {
        patch.bugReportsEnabled = bugEnabledDraft();
      }
      const trimmedPid = bugProjectIdDraft().trim();
      if ((cur?.bugReportsProjectId ?? "") !== trimmedPid) {
        // Empty string means "clear" — but the API treats absence as "leave
        // alone". Sending an empty pointer would persist as "". The server
        // recreate-on-stale path handles a missing project just fine, so
        // we send the empty string when the admin actually cleared it.
        patch.bugReportsProjectId = trimmedPid;
      }
      if (Object.keys(patch).length === 0) {
        setSettingsMsg("Nothing to update.");
        return;
      }
      await updateSettings(id, patch);
      setSettingsMsg("Saved.");
    } catch (e2) {
      setSettingsErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setSettingsBusy(false);
    }
  };

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

      <Show when={isAdmin()}>
        <section style="margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
          <h3 style="margin:0 0 6px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
            Trusted domains
          </h3>
          <p style="margin:0 0 14px;color:var(--ink-mute);font-size:14px">
            Members signing in from these domains auto-join this house on
            their first sign-in. Remove a domain to stop new auto-joins;
            existing memberships stay put.
          </p>
          <ul style="list-style:none;margin:0 0 14px;padding:0;display:flex;flex-direction:column;gap:6px">
            <For
              each={trustedDomains() ?? []}
              fallback={
                <li style="color:var(--ink-mute);font-style:italic;font-size:13px">
                  No trusted domains yet — sign-ins from any domain need an explicit invite.
                </li>
              }
            >
              {(td) => (
                <li style="display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);font-size:14px">
                  <span style="font-family:var(--mono,monospace)">{td.domain}</span>
                  <button
                    type="button"
                    class="btn btn-ghost"
                    onClick={() => removeDomain(td.trustedDomainId)}
                    disabled={tdBusy()}
                    style="padding:4px 10px;color:var(--rust);font-size:13px"
                  >
                    Remove
                  </button>
                </li>
              )}
            </For>
          </ul>
          <form onSubmit={addDomain} style="display:flex;gap:10px;align-items:end">
            <label style="display:flex;flex-direction:column;gap:4px;flex:1 1 240px">
              <span style="font-size:12px;color:var(--ink-mute)">Add a domain</span>
              <input
                type="text"
                value={newDomain()}
                onInput={(e) => setNewDomain(e.currentTarget.value)}
                placeholder="example.com"
                style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;font-family:var(--mono,monospace)"
              />
            </label>
            <button class="btn btn-primary" type="submit" disabled={tdBusy() || !newDomain().trim()}>
              {tdBusy() ? "Saving…" : "Add"}
            </button>
            <Show when={tdErr()}>
              {(m) => <span style="color:var(--rust);font-size:13px;flex:1 1 100%">{m()}</span>}
            </Show>
          </form>
        </section>

        <section style="margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
          <h3 style="margin:0 0 6px;font-family:var(--display);font-size:20px;color:var(--grass-4)">
            Feature settings
          </h3>
          <p style="margin:0 0 14px;color:var(--ink-mute);font-size:14px">
            Toggles that apply to everyone in this house.
          </p>
          <form onSubmit={submitFeatureSettings} style="display:flex;flex-direction:column;gap:14px">
            <label style="display:flex;gap:10px;align-items:flex-start">
              <input
                type="checkbox"
                checked={bugEnabledDraft()}
                onChange={(e) => setBugEnabledDraft(e.currentTarget.checked)}
                style="margin-top:3px"
              />
              <span>
                <div>In-app bug reports</div>
                <div style="font-size:12px;color:var(--ink-mute)">
                  Shows a bug icon in the header for every member of this
                  house. Submissions land as tasks in the bug-fixes project.
                </div>
              </span>
            </label>
            <label style="display:flex;flex-direction:column;gap:4px">
              <span style="font-size:12px;color:var(--ink-mute)">
                Bug-reports project id (leave blank to let the server create one on the next report)
              </span>
              <input
                type="text"
                value={bugProjectIdDraft()}
                onInput={(e) => setBugProjectIdDraft(e.currentTarget.value)}
                placeholder="(server-managed)"
                style="padding:8px 12px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:14px;font-family:var(--mono,monospace)"
              />
            </label>
            <div style="display:flex;gap:10px;align-items:center">
              <button class="btn btn-primary" type="submit" disabled={settingsBusy()}>
                {settingsBusy() ? "Saving…" : "Save settings"}
              </button>
              <Show when={settingsErr()}>
                {(m) => <span style="color:var(--rust);font-size:13px">{m()}</span>}
              </Show>
              <Show when={settingsMsg()}>
                {(m) => <span style="color:var(--grass-4);font-size:13px">{m()}</span>}
              </Show>
            </div>
          </form>
        </section>
      </Show>
    </AuthGate>
  );
};
