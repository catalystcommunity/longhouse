import { query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { navigation } from "~/ui/router";
import { AuthGate } from "~/components/AuthGate";
import { useHouses, useSession } from "~/stores/auth";
import { endSession } from "~/lib/session";
import { authClient } from "~/data/clients";
/**
 * Per-user account surface. Today this is a read-only view of the signed-in
 * identity, the houses the bearer can act in (with the roles in each), and
 * a sign-out button. As user-scoped settings show up in CSIL, they land
 * here.
 */
export const AccountPage = () => {
    const navigate = navigation();
    const session = useSession();
    const houses = useHouses();
    const [revoking, setRevoking] = cell<string | null>(null);
    const [sessionError, setSessionError] = cell<string | null>(null);
    const [cliSessions, { refetch: refetchCliSessions }] = query(() => session()?.token, async () => ((await authClient.listSessions({})).sessions ?? []).filter(
    // Every browser sign-in makes a session, so ended ones would pile
    // up here. Keep only live browser sessions; CLI rows stay visible.
    (s) => s.kind !== "browser" || (!s.revokedAt && Date.parse(s.expiresAt) > Date.now())));
    const onSignOut = async () => {
        await endSession();
        navigate(import.meta.env.DEV ? "/dev-login" : "/");
    };
    const revokeCliSession = async (sessionId: string, current: boolean) => {
        setRevoking(sessionId);
        setSessionError(null);
        try {
            await authClient.revokeSession({ sessionId });
            // Revoking this browser's own session is a sign-out.
            if (current) {
                await onSignOut();
                return;
            }
            await refetchCliSessions();
        }
        catch (error) {
            setSessionError(error instanceof Error ? error.message : String(error));
        }
        finally {
            setRevoking(null);
        }
    };
    return (AuthGate({
        get children() {
            return [
                el("div", {
                    "class": "section-hd reveal",
                    get children() {
                        return [
                            el("h2", {
                                get children() {
                                    return [
                                        "Account ",
                                        el("em", {
                                            get children() {
                                                return "you, and your houses";
                                            }
                                        })
                                    ];
                                }
                            }),
                            el("p", {
                                "class": "lead",
                                get children() {
                                    return "Your identity comes from linkkeys; per-house roles come from the bearer.";
                                }
                            })
                        ];
                    }
                }),
                el("section", {
                    "style": "margin-top:16px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return "Identity";
                                }
                            }),
                            When({
                                get "when"() {
                                    return session();
                                },
                                get "fallback"() {
                                    return el("p", {
                                        "style": "color:var(--ink-mute)",
                                        get children() {
                                            return "Not signed in.";
                                        }
                                    });
                                },
                                children: (s) => (el("dl", {
                                    "style": "display:grid;grid-template-columns:max-content 1fr;gap:8px 18px;margin:0;font-size:14px",
                                    get children() {
                                        return [
                                            el("dt", {
                                                "style": "color:var(--ink-mute)",
                                                get children() {
                                                    return "Display name";
                                                }
                                            }),
                                            el("dd", {
                                                "style": "margin:0",
                                                get children() {
                                                    return live(() => s().displayName || "—");
                                                }
                                            }),
                                            el("dt", {
                                                "style": "color:var(--ink-mute)",
                                                get children() {
                                                    return "User id";
                                                }
                                            }),
                                            el("dd", {
                                                "style": "margin:0;font-family:var(--mono,monospace);font-size:13px",
                                                get children() {
                                                    return live(() => s().userId);
                                                }
                                            }),
                                            el("dt", {
                                                "style": "color:var(--ink-mute)",
                                                get children() {
                                                    return "Domain";
                                                }
                                            }),
                                            el("dd", {
                                                "style": "margin:0;font-family:var(--mono,monospace);font-size:13px",
                                                get children() {
                                                    return live(() => s().domain);
                                                }
                                            }),
                                            el("dt", {
                                                "style": "color:var(--ink-mute)",
                                                get children() {
                                                    return "Token expires";
                                                }
                                            }),
                                            el("dd", {
                                                "style": "margin:0",
                                                get children() {
                                                    return live(() => new Date(s().expiresAt).toLocaleString());
                                                }
                                            })
                                        ];
                                    }
                                }))
                            })
                        ];
                    }
                }),
                el("section", {
                    "style": "margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return "Sessions";
                                }
                            }),
                            el("p", {
                                "style": "margin:0 0 14px;color:var(--ink-mute);font-size:14px",
                                get children() {
                                    return "Browsers and command-line tools that are signed in as you. Revoke one that you no longer use, such as a lost phone. Its current access bearer can work until it expires.";
                                }
                            }),
                            When({
                                get "when"() {
                                    return sessionError();
                                },
                                children: (message) => el("p", {
                                    "class": "err",
                                    get children() {
                                        return live(() => message());
                                    }
                                })
                            }),
                            When({
                                get "when"() {
                                    return !cliSessions.loading;
                                },
                                get "fallback"() {
                                    return el("p", {
                                        "style": "color:var(--ink-mute)",
                                        get children() {
                                            return "Loading sessions\u2026";
                                        }
                                    });
                                },
                                get children() {
                                    return When({
                                        get "when"() {
                                            return (cliSessions() ?? []).length > 0;
                                        },
                                        get "fallback"() {
                                            return el("p", {
                                                "style": "color:var(--ink-mute)",
                                                get children() {
                                                    return "No sessions.";
                                                }
                                            });
                                        },
                                        get children() {
                                            return el("ul", {
                                                "style": "list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:8px",
                                                get children() {
                                                    return Repeat({
                                                        get "each"() {
                                                            return cliSessions();
                                                        },
                                                        children: (cliSession) => {
                                                            const inactive = () => Boolean(cliSession.revokedAt) || Date.parse(cliSession.expiresAt) <= Date.now();
                                                            const kindLabel = cliSession.kind === "browser" ? "Browser" : "Command line";
                                                            return (el("li", {
                                                                "style": "display:flex;justify-content:space-between;align-items:center;gap:12px;padding:10px 12px;border:1px solid var(--line);border-radius:var(--r-md)",
                                                                get children() {
                                                                    return [
                                                                        el("span", {
                                                                            get children() {
                                                                                return [
                                                                                    el("strong", {
                                                                                        get children() {
                                                                                            return live(() => cliSession.clientName);
                                                                                        }
                                                                                    }),
                                                                                    When({
                                                                                        get "when"() {
                                                                                            return cliSession.current;
                                                                                        },
                                                                                        get children() {
                                                                                            return el("span", {
                                                                                                "style": "margin-left:8px;font-size:12px;color:var(--grass-4)",
                                                                                                get children() {
                                                                                                    return "This device";
                                                                                                }
                                                                                            });
                                                                                        }
                                                                                    }),
                                                                                    el("span", {
                                                                                        "style": "display:block;color:var(--ink-mute);font-size:12px",
                                                                                        get children() {
                                                                                            return [
                                                                                                live(() => kindLabel),
                                                                                                " \u00B7 ",
                                                                                                live(() => inactive() ? "Inactive" : `Last used ${new Date(cliSession.lastUsedAt).toLocaleString()}`)
                                                                                            ];
                                                                                        }
                                                                                    })
                                                                                ];
                                                                            }
                                                                        }),
                                                                        el("button", {
                                                                            "class": "btn btn-ghost",
                                                                            "type": "button",
                                                                            get "disabled"() {
                                                                                return inactive() || revoking() !== null;
                                                                            },
                                                                            "onClick": () => void revokeCliSession(cliSession.sessionId, cliSession.current === true),
                                                                            get children() {
                                                                                return live(() => revoking() === cliSession.sessionId ? "Revoking…" : "Revoke");
                                                                            }
                                                                        })
                                                                    ];
                                                                }
                                                            }));
                                                        }
                                                    });
                                                }
                                            });
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                el("section", {
                    "style": "margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return "Houses";
                                }
                            }),
                            When({
                                get "when"() {
                                    return houses().length > 0;
                                },
                                get "fallback"() {
                                    return el("p", {
                                        "style": "color:var(--ink-mute)",
                                        get children() {
                                            return "No houses yet.";
                                        }
                                    });
                                },
                                get children() {
                                    return el("ul", {
                                        "style": "list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:8px",
                                        get children() {
                                            return Repeat({
                                                get "each"() {
                                                    return houses();
                                                },
                                                children: (h) => (el("li", {
                                                    "style": "display:flex;justify-content:space-between;gap:12px;padding:10px 12px;border:1px solid var(--line);border-radius:var(--r-md)",
                                                    get children() {
                                                        return [
                                                            el("span", {
                                                                "style": "font-weight:500",
                                                                get children() {
                                                                    return live(() => h.name || h.id);
                                                                }
                                                            }),
                                                            el("span", {
                                                                "style": "color:var(--ink-mute);font-size:13px",
                                                                get children() {
                                                                    return live(() => h.roles?.length ? h.roles.join(", ") : "member");
                                                                }
                                                            })
                                                        ];
                                                    }
                                                }))
                                            });
                                        }
                                    });
                                }
                            })
                        ];
                    }
                }),
                el("section", {
                    "style": "margin-top:20px;padding:20px 22px;background:var(--paper);border:1px solid var(--line);border-radius:var(--r-lg);box-shadow:var(--shadow-low)",
                    get children() {
                        return [
                            el("h3", {
                                "style": "margin:0 0 10px;font-family:var(--display);font-size:20px;color:var(--grass-4)",
                                get children() {
                                    return "Sign out";
                                }
                            }),
                            el("p", {
                                "style": "margin:0 0 14px;color:var(--ink-mute);font-size:14px",
                                get children() {
                                    return "Ends this browser's session. The next sign-in goes through linkkeys (or the dev picker, in dev builds).";
                                }
                            }),
                            el("button", {
                                "class": "btn btn-primary",
                                "type": "button",
                                get "onClick"() {
                                    return onSignOut;
                                },
                                get children() {
                                    return "Sign out";
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }));
};
