import { createSignal, createRoot, createEffect } from "solid-js";
import type { EffectiveSettings } from "@longhouse/client";
import { settingsClient } from "~/data/clients";
import { useCurrentHouseId, useSession } from "~/stores/auth";

/**
 * Settings store — the merged effective settings for the current house.
 * Reloaded when the session or the current house changes. Components read
 * specific keys via the small helpers at the bottom; until the first load
 * lands, every key reads as undefined and the helpers fall back to the
 * documented defaults (e.g. bug reports off).
 */

const [settings, setSettings] = createSignal<EffectiveSettings | null>(null);
const [loading, setLoading] = createSignal(false);

export const useSettings = () => settings;

/** Force a fresh load for the given house id. Safe to call concurrently —
 *  the most recent successful response wins. */
export async function reloadSettings(houseId: string): Promise<void> {
  if (!houseId) {
    setSettings(null);
    return;
  }
  setLoading(true);
  try {
    const next = await settingsClient.getSettings(houseId);
    setSettings(next);
  } catch {
    // Don't blow away the previous value on a transient failure — the
    // UI keeps rendering whatever it last had. Caller-initiated updates
    // surface their own error state.
  } finally {
    setLoading(false);
  }
}

/** Apply a partial update via SettingsService.UpdateSettings, then refresh
 *  the local store with the server's merged result. Returns the new state. */
export async function updateSettings(
  houseId: string,
  patch: EffectiveSettings,
): Promise<EffectiveSettings> {
  const next = await settingsClient.updateSettings({ houseId, settings: patch });
  setSettings(next);
  return next;
}

createRoot(() => {
  const session = useSession();
  const houseId = useCurrentHouseId();
  // Reload whenever the (session, houseId) pair changes. The effect is
  // app-lifetime intentionally — settings outlive any single page.
  createEffect(() => {
    const s = session();
    const hid = houseId();
    if (!s || !hid) {
      setSettings(null);
      return;
    }
    void reloadSettings(hid);
  });
});

/** Bug reports flag — defaults to false until loaded. */
export const bugReportsEnabled = (): boolean => settings()?.bugReportsEnabled === true;

export const isSettingsLoading = () => loading;
