import { query, cell } from "~/ui/state";
import { When, el, live } from "~/ui/dom";
import { searchState } from "~/ui/router";
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
    const [params, setParams] = searchState();
    const [codeInput, setCodeInput] = cell(typeof params.code === "string" ? params.code : "");
    const [submittedCode, setSubmittedCode] = cell(normalizeCode(codeInput()));
    const [result, setResult] = cell<"approved" | "denied" | null>(null);
    const [busy, setBusy] = cell(false);
    const [actionError, setActionError] = cell<string | null>(null);
    const [request] = query(() => session() && submittedCode().length === 8 ? submittedCode() : null, async (code) => authClient.inspectCliLogin({ userCode: code }));
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
            if (approved)
                await authClient.approveCliLogin(req);
            else
                await authClient.denyCliLogin(req);
            setResult(approved ? "approved" : "denied");
        }
        catch (error) {
            setActionError(error instanceof Error ? error.message : String(error));
        }
        finally {
            setBusy(false);
        }
    };
    return (el("section", {
        "class": "reveal d1",
        "style": "max-width:620px;margin:48px auto 0",
        get children() {
            return el("div", {
                "style": "padding:28px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
                get children() {
                    return [
                        el("h1", {
                            "style": "margin-top:0;font-family:var(--display);color:var(--grass-4)",
                            get children() {
                                return "Authorize a CLI";
                            }
                        }),
                        When({
                            get "when"() {
                                return !session();
                            },
                            get children() {
                                return [
                                    el("p", {
                                        "style": "color:var(--ink-mute)",
                                        get children() {
                                            return "Sign in before you authorize the command-line client.";
                                        }
                                    }),
                                    el("a", {
                                        get "href"() {
                                            return import.meta.env.DEV ? "/dev-login" : "/api/v1/auth/start";
                                        },
                                        get "rel"() {
                                            return import.meta.env.DEV ? undefined : "external";
                                        },
                                        "class": "btn btn-primary",
                                        get "onClick"() {
                                            return signIn;
                                        },
                                        get children() {
                                            return "Sign in";
                                        }
                                    })
                                ];
                            }
                        }),
                        When({
                            get "when"() {
                                return session() && !result();
                            },
                            get children() {
                                return [
                                    el("form", {
                                        get "onSubmit"() {
                                            return submitCode;
                                        },
                                        "style": "display:flex;gap:10px;align-items:end;margin:18px 0",
                                        get children() {
                                            return [
                                                el("label", {
                                                    "style": "display:flex;flex-direction:column;gap:6px;flex:1;color:var(--ink-mute);font-size:13px",
                                                    get children() {
                                                        return [
                                                            "Code from the CLI",
                                                            el("input", {
                                                                get "value"() {
                                                                    return codeInput();
                                                                },
                                                                "onInput": (event) => setCodeInput(event.currentTarget.value),
                                                                "autocomplete": "one-time-code",
                                                                "maxlength": "9",
                                                                "placeholder": "ABCD-EFGH",
                                                                "style": "font-family:var(--mono,monospace);font-size:18px;letter-spacing:.08em;text-transform:uppercase"
                                                            })
                                                        ];
                                                    }
                                                }),
                                                el("button", {
                                                    "class": "btn btn-ghost",
                                                    "type": "submit",
                                                    get children() {
                                                        return "Check code";
                                                    }
                                                })
                                            ];
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return request.loading;
                                        },
                                        get children() {
                                            return el("p", {
                                                "style": "color:var(--ink-mute)",
                                                get children() {
                                                    return "Checking the request\u2026";
                                                }
                                            });
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return request.error;
                                        },
                                        get children() {
                                            return el("p", {
                                                "class": "err",
                                                get children() {
                                                    return live(() => request.error instanceof Error ? request.error.message : String(request.error));
                                                }
                                            });
                                        }
                                    }),
                                    When({
                                        get "when"() {
                                            return request();
                                        },
                                        children: (info) => (el("div", {
                                            "style": "padding:18px;border:1px solid var(--line);border-radius:var(--r-md)",
                                            get children() {
                                                return [
                                                    el("p", {
                                                        "style": "margin:0 0 8px",
                                                        get children() {
                                                            return [
                                                                "Authorize ",
                                                                el("strong", {
                                                                    get children() {
                                                                        return live(() => info().clientName);
                                                                    }
                                                                }),
                                                                " to use your Longhouse account?"
                                                            ];
                                                        }
                                                    }),
                                                    el("p", {
                                                        "style": "margin:0 0 18px;color:var(--ink-mute);font-size:13px",
                                                        get children() {
                                                            return [
                                                                "Code ",
                                                                live(() => info().userCode),
                                                                ". This request expires on ",
                                                                live(() => new Date(info().expiresAt).toLocaleString()),
                                                                "."
                                                            ];
                                                        }
                                                    }),
                                                    el("div", {
                                                        "style": "display:flex;gap:10px",
                                                        get children() {
                                                            return [
                                                                el("button", {
                                                                    "class": "btn btn-primary",
                                                                    "type": "button",
                                                                    get "disabled"() {
                                                                        return busy();
                                                                    },
                                                                    "onClick": () => void decide(true),
                                                                    get children() {
                                                                        return "Authorize";
                                                                    }
                                                                }),
                                                                el("button", {
                                                                    "class": "btn btn-ghost",
                                                                    "type": "button",
                                                                    get "disabled"() {
                                                                        return busy();
                                                                    },
                                                                    "onClick": () => void decide(false),
                                                                    get children() {
                                                                        return "Deny";
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    })
                                                ];
                                            }
                                        }))
                                    }),
                                    When({
                                        get "when"() {
                                            return actionError();
                                        },
                                        children: (message) => el("p", {
                                            "class": "err",
                                            get children() {
                                                return live(() => message());
                                            }
                                        })
                                    })
                                ];
                            }
                        }),
                        When({
                            get "when"() {
                                return result() === "approved";
                            },
                            get children() {
                                return [
                                    el("h2", {
                                        get children() {
                                            return "CLI authorized";
                                        }
                                    }),
                                    el("p", {
                                        "style": "color:var(--ink-mute)",
                                        get children() {
                                            return "Return to the terminal. You can close this page.";
                                        }
                                    })
                                ];
                            }
                        }),
                        When({
                            get "when"() {
                                return result() === "denied";
                            },
                            get children() {
                                return [
                                    el("h2", {
                                        get children() {
                                            return "Request denied";
                                        }
                                    }),
                                    el("p", {
                                        "style": "color:var(--ink-mute)",
                                        get children() {
                                            return "The CLI did not receive a session. You can close this page.";
                                        }
                                    })
                                ];
                            }
                        })
                    ];
                }
            });
        }
    }));
};
