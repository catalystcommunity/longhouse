import { Show, createResource, createSignal } from "solid-js";
import { useSearchParams } from "@solidjs/router";
import { authClient } from "~/data/clients";
import { rememberAuthReturnTo } from "~/lib/session";
import { useSession } from "~/stores/auth";

const normalizeCode = (value: string) => value.trim().toUpperCase().replaceAll("-", "");
const formatCode = (value: string) => {
  const code = normalizeCode(value);
  return code.length === 8 ? `${code.slice(0, 4)}-${code.slice(4)}` : value;
};

export const CliAuthorizePage = () => {
  const session = useSession();
  const [params, setParams] = useSearchParams();
  const [codeInput, setCodeInput] = createSignal(typeof params.code === "string" ? params.code : "");
  const [submittedCode, setSubmittedCode] = createSignal(normalizeCode(codeInput()));
  const [result, setResult] = createSignal<"approved" | "denied" | null>(null);
  const [busy, setBusy] = createSignal(false);
  const [actionError, setActionError] = createSignal<string | null>(null);

  const [request] = createResource(
    () => session() && submittedCode().length === 8 ? submittedCode() : null,
    async (code) => authClient.inspectCliLogin({ userCode: code }),
  );

  const submitCode = (event: Event) => {
    event.preventDefault();
    const code = normalizeCode(codeInput());
    setSubmittedCode(code);
    setParams(code.length ? { code: formatCode(code) } : {});
    setResult(null);
    setActionError(null);
  };

  const signIn = () => {
    rememberAuthReturnTo(`${window.location.pathname}${window.location.search}`);
  };

  const decide = async (approved: boolean) => {
    setBusy(true);
    setActionError(null);
    try {
      const req = { userCode: submittedCode() };
      if (approved) await authClient.approveCliLogin(req);
      else await authClient.denyCliLogin(req);
      setResult(approved ? "approved" : "denied");
    } catch (error) {
      setActionError(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section class="reveal d1" style="max-width:620px;margin:48px auto 0">
      <div style="padding:28px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)">
        <h1 style="margin-top:0;font-family:var(--display);color:var(--grass-4)">Authorize a CLI</h1>

        <Show when={!session()}>
          <p style="color:var(--ink-mute)">Sign in before you authorize the command-line client.</p>
          <a
            href={import.meta.env.DEV ? "/dev-login" : "/api/v1/auth/start"}
            rel={import.meta.env.DEV ? undefined : "external"}
            class="btn btn-primary"
            onClick={signIn}
          >
            Sign in
          </a>
        </Show>

        <Show when={session() && !result()}>
          <form onSubmit={submitCode} style="display:flex;gap:10px;align-items:end;margin:18px 0">
            <label style="display:flex;flex-direction:column;gap:6px;flex:1;color:var(--ink-mute);font-size:13px">
              Code from the CLI
              <input
                value={codeInput()}
                onInput={(event) => setCodeInput(event.currentTarget.value)}
                autocomplete="one-time-code"
                maxlength="9"
                placeholder="ABCD-EFGH"
                style="font-family:var(--mono,monospace);font-size:18px;letter-spacing:.08em;text-transform:uppercase"
              />
            </label>
            <button class="btn btn-ghost" type="submit">Check code</button>
          </form>

          <Show when={request.loading}><p style="color:var(--ink-mute)">Checking the request…</p></Show>
          <Show when={request.error}>
            <p class="err">{request.error instanceof Error ? request.error.message : String(request.error)}</p>
          </Show>
          <Show when={request()}>
            {(info) => (
              <div style="padding:18px;border:1px solid var(--line);border-radius:var(--r-md)">
                <p style="margin:0 0 8px">Authorize <strong>{info().clientName}</strong> to use your Longhouse account?</p>
                <p style="margin:0 0 18px;color:var(--ink-mute);font-size:13px">
                  Code {info().userCode}. This request expires on {new Date(info().expiresAt).toLocaleString()}.
                </p>
                <div style="display:flex;gap:10px">
                  <button class="btn btn-primary" type="button" disabled={busy()} onClick={() => void decide(true)}>
                    Authorize
                  </button>
                  <button class="btn btn-ghost" type="button" disabled={busy()} onClick={() => void decide(false)}>
                    Deny
                  </button>
                </div>
              </div>
            )}
          </Show>
          <Show when={actionError()}>{(message) => <p class="err">{message()}</p>}</Show>
        </Show>

        <Show when={result() === "approved"}>
          <h2>CLI authorized</h2>
          <p style="color:var(--ink-mute)">Return to the terminal. You can close this page.</p>
        </Show>
        <Show when={result() === "denied"}>
          <h2>Request denied</h2>
          <p style="color:var(--ink-mute)">The CLI did not receive a session. You can close this page.</p>
        </Show>
      </div>
    </section>
  );
};
