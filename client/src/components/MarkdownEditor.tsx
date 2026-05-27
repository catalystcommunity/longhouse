import { createSignal, createMemo, Show } from "solid-js";
import { marked } from "marked";
import DOMPurify, { type Config } from "dompurify";

/**
 * Click-to-edit markdown field. Default view is rendered HTML; clicking
 * anywhere in the rendered body swaps to a raw-markdown <textarea>.
 * Blur (focusout) collapses back to rendered. The parent owns the value
 * via `value` + `onChange` — this component is stateless except for the
 * focus/edit toggle.
 *
 * Security model:
 *   * Raw HTML typed by the user is NOT rendered. We override marked's
 *     `tag` and `html` tokenizers to return false, so any `<...>` in the
 *     input falls through to the text tokenizer (which HTML-escapes the
 *     brackets). A user-typed `<a href="evil">click</a>` shows up as
 *     literal text.
 *   * `<a>` tags ARE allowed in the sanitized OUTPUT — but only the ones
 *     marked itself emits from markdown syntax (`[text](url)` and
 *     autolinks), because those are the only `<a>` tags marked can
 *     produce now that inline HTML is disabled.
 *   * DOMPurify is still the last line of defense — it strips anything
 *     outside the structural-markdown allowlist + the curated `<a>` form.
 *   * Anchors that point outside the current origin get `target="_blank"`
 *     + `rel="noopener noreferrer"` so user-supplied links can't hijack
 *     the SPA tab via `window.opener`. Same-origin links stay in-place.
 */

marked.setOptions({ gfm: true, breaks: true });

// Disable raw inline/block HTML by overriding the renderer hook. The
// tokenizer still consumes raw `<…>` as an html token, but instead of
// emitting it verbatim we HTML-escape and emit as text. Result: any
// user-typed `<a href="evil">click</a>` shows up as literal characters,
// while marked's own `<a>` output (from `[text](url)` and autolinks)
// is unaffected.
const escapeHTML = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

marked.use({
  renderer: {
    html(token: any) {
      const src = typeof token === "string" ? token : (token.text ?? token.raw ?? "");
      return escapeHTML(src);
    },
  },
});

const sanitizeOptions: Config = {
  // Tight allowlist: structural markdown plus anchors (markdown-only —
  // raw `<a>` is already stripped by marked's tokenizer override above).
  // Drops script/style/iframe/form/object/embed/etc. by omission.
  ALLOWED_TAGS: [
    "p", "br", "hr",
    "strong", "em", "del", "code", "pre",
    "ul", "ol", "li",
    "blockquote",
    "a",
    "h1", "h2", "h3", "h4", "h5", "h6",
    "table", "thead", "tbody", "tr", "th", "td",
  ],
  ALLOWED_ATTR: ["href", "title", "align", "target", "rel"],
  ALLOWED_URI_REGEXP: /^(?:https?|mailto):/i,
};

// External anchors open in a new tab with noopener/noreferrer; same-origin
// anchors stay in-place so internal nav (when we eventually emit it) feels
// native. We re-add the hook every render to make sure it's installed
// even after HMR.
DOMPurify.removeAllHooks();
DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName !== "A") return;
  const href = node.getAttribute("href") ?? "";
  let external = true;
  try {
    const u = new URL(href, window.location.href);
    external = u.origin !== window.location.origin;
  } catch {
    external = false;
  }
  if (external) {
    node.setAttribute("target", "_blank");
    node.setAttribute("rel", "noopener noreferrer");
  } else {
    node.removeAttribute("target");
    node.removeAttribute("rel");
  }
});

interface Props {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  rows?: number;
  /** Extra style applied to both the rendered and editing surfaces, so the
   *  swap doesn't jitter the layout. */
  style?: string;
}

// URL-ish single token: starts with http(s):// (or mailto:), no internal
// whitespace. Used for the smart-paste handler.
const URL_RE = /^(?:https?:\/\/|mailto:)\S+$/;

export const MarkdownEditor = (props: Props) => {
  const [editing, setEditing] = createSignal(false);
  let textareaRef: HTMLTextAreaElement | undefined;

  const rendered = createMemo(() => {
    const v = props.value.trim();
    if (!v) return "";
    const html = marked.parse(v, { async: false }) as string;
    // DOMPurify.sanitize returns TrustedHTML when Trusted Types are on;
    // we always treat it as a plain string for innerHTML.
    return String(DOMPurify.sanitize(html, sanitizeOptions));
  });

  const enterEdit = () => {
    setEditing(true);
    // Focus the textarea on the next tick so the click event finishes
    // first; otherwise the click-outside handlers elsewhere may steal
    // focus back.
    queueMicrotask(() => textareaRef?.focus());
  };

  const exitEdit = () => setEditing(false);

  // Smart paste: if the user has text highlighted and pastes a bare URL,
  // wrap the selection in `[selection](url)` instead of letting the URL
  // overwrite the selection. Matches the pattern from Slack/Notion/etc.
  // No-op for pastes without a clear URL or with no selection.
  const onPaste = (e: ClipboardEvent) => {
    const ta = textareaRef;
    if (!ta) return;
    const pasted = e.clipboardData?.getData("text/plain") ?? "";
    if (!URL_RE.test(pasted.trim())) return;
    const start = ta.selectionStart;
    const end = ta.selectionEnd;
    if (start === end) return; // no selection — fall through to default paste
    e.preventDefault();
    const before = ta.value.slice(0, start);
    const selected = ta.value.slice(start, end);
    const after = ta.value.slice(end);
    const link = `[${selected}](${pasted.trim()})`;
    const next = before + link + after;
    props.onChange(next);
    // Restore cursor at the end of the inserted link on the next frame
    // (after Solid has reconciled the controlled textarea value).
    queueMicrotask(() => {
      const pos = before.length + link.length;
      ta.selectionStart = ta.selectionEnd = pos;
      ta.focus();
    });
  };

  return (
    <Show
      when={editing()}
      fallback={
        <div
          role="textbox"
          tabIndex={0}
          aria-label="Description (click to edit)"
          onClick={enterEdit}
          onFocus={enterEdit}
          style={`min-height:36px;padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;cursor:text;line-height:1.55;${props.style ?? ""}`}
        >
          <Show
            when={props.value.trim().length > 0}
            fallback={
              <span style="color:var(--ink-mute);font-style:italic">
                {props.placeholder ?? "Click to add a description (markdown supported)…"}
              </span>
            }
          >
            <div class="md-body" innerHTML={rendered()} />
          </Show>
        </div>
      }
    >
      <textarea
        ref={textareaRef}
        rows={props.rows ?? 4}
        value={props.value}
        placeholder={props.placeholder}
        onInput={(e) => props.onChange(e.currentTarget.value)}
        onPaste={onPaste}
        onBlur={exitEdit}
        style={`padding:7px 10px;border:1px solid var(--line);border-radius:var(--r-md);background:var(--paper);font-size:13px;resize:vertical;font-family:var(--mono,inherit);line-height:1.5;${props.style ?? ""}`}
      />
    </Show>
  );
};
