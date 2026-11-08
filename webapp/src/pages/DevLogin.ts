import { query, cell } from "~/ui/state";
import { Repeat, When, el, live } from "~/ui/dom";
import { navigation } from "~/ui/router";
/**
 * Dev-only login page. Lists every (member, house) the local API exposes
 * via DevAuthService.ListDevUsers, click one to call DevAuthService.DevLogin
 * and stash the resulting JWT in the auth store.
 *
 * The whole module is dynamically imported (see App.ts) so prod builds
 * tree-shake it out — the dead-code branch on `import.meta.env.DEV`
 * eliminates the import.
 */
import { devAuthClient } from "~/data/clients";
import { finishLogin, takeAuthReturnTo } from "~/lib/session";
import type { DevUserEntry } from "@longhouse/client";
export const DevLogin = () => {
    const navigate = navigation();
    const [users] = query(async () => {
        const res = await devAuthClient.listDevUsers({});
        return res.users ?? [];
    });
    const [busy, setBusy] = cell<string | null>(null);
    const [err, setErr] = cell<string | null>(null);
    const choose = async (u: DevUserEntry) => {
        setErr(null);
        setBusy(u.memberId);
        try {
            const resp = await devAuthClient.devLogin({ memberId: u.memberId });
            await finishLogin(resp);
            navigate(takeAuthReturnTo(), { replace: true });
        }
        catch (e) {
            setErr(e instanceof Error ? e.message : String(e));
            setBusy(null);
        }
    };
    return (el("section", {
        "class": "dev-login reveal d1",
        get children() {
            return el("div", {
                "class": "dev-login-card",
                get children() {
                    return [
                        el("span", {
                            "class": "kicker plain",
                            "style": "color: var(--rust); border-color: color-mix(in oklab, var(--rust) 40%, transparent); background: color-mix(in oklab, var(--rust) 14%, var(--paper))",
                            get children() {
                                return "DEV MODE";
                            }
                        }),
                        el("h1", {
                            get children() {
                                return "Choose a user to sign in as";
                            }
                        }),
                        el("p", {
                            "class": "lede",
                            get children() {
                                return [
                                    "This page only exists in development. The list comes from the local API's",
                                    el("code", {
                                        get children() {
                                            return " devauth.ListDevUsers";
                                        }
                                    }),
                                    " method, which itself is only registered when ",
                                    el("code", {
                                        get children() {
                                            return "LONGHOUSE_DEV_AUTH_ENABLED=true";
                                        }
                                    }),
                                    " and",
                                    el("code", {
                                        get children() {
                                            return "LONGHOUSE_ENV";
                                        }
                                    }),
                                    " is ",
                                    el("code", {
                                        get children() {
                                            return "dev";
                                        }
                                    }),
                                    " or ",
                                    el("code", {
                                        get children() {
                                            return "nonprod";
                                        }
                                    }),
                                    "."
                                ];
                            }
                        }),
                        When({
                            get "when"() {
                                return err();
                            },
                            children: (message) => (el("div", {
                                "class": "dev-login-err",
                                get children() {
                                    return [
                                        el("b", {
                                            get children() {
                                                return "Couldn't sign in:";
                                            }
                                        }),
                                        " ",
                                        live(() => message()),
                                        el("p", {
                                            "style": "margin: 6px 0 0; font-size: 13px",
                                            get children() {
                                                return [
                                                    "Is the api running on port 6080? Is dev-auth actually enabled? Try",
                                                    live(() => " "),
                                                    el("code", {
                                                        get children() {
                                                            return "LONGHOUSE_DEV_AUTH_ENABLED=true LONGHOUSE_ENV=dev go run ./api serve";
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
                                return users() && users()!.length > 0;
                            },
                            get "fallback"() {
                                return el("div", {
                                    "class": "dev-login-empty",
                                    get children() {
                                        return [
                                            When({
                                                get "when"() {
                                                    return users.loading;
                                                },
                                                get children() {
                                                    return "Loading users\u2026";
                                                }
                                            }),
                                            When({
                                                get "when"() {
                                                    return users.error;
                                                },
                                                get children() {
                                                    return [
                                                        el("b", {
                                                            get children() {
                                                                return "Failed to fetch users.";
                                                            }
                                                        }),
                                                        " The api isn't reachable, or dev-auth isn't enabled."
                                                    ];
                                                }
                                            }),
                                            When({
                                                get "when"() {
                                                    return users() && users()!.length === 0;
                                                },
                                                get children() {
                                                    return "No members in the local DB. Sign in once with linkkeys (or seed the DB) so there's a member to impersonate.";
                                                }
                                            })
                                        ];
                                    }
                                });
                            },
                            get children() {
                                return el("ul", {
                                    "class": "dev-login-list",
                                    get children() {
                                        return Repeat({
                                            get "each"() {
                                                return users();
                                            },
                                            children: (u) => (el("li", {
                                                get children() {
                                                    return el("button", {
                                                        get "disabled"() {
                                                            return busy() !== null;
                                                        },
                                                        "onClick": () => choose(u),
                                                        get "class"() {
                                                            return busy() === u.memberId ? "busy" : "";
                                                        },
                                                        get children() {
                                                            return [
                                                                el("span", {
                                                                    "class": "who-name",
                                                                    get children() {
                                                                        return live(() => u.displayName ?? u.linkkeysUserId ?? u.memberId);
                                                                    }
                                                                }),
                                                                el("span", {
                                                                    "class": "meta",
                                                                    get children() {
                                                                        return [
                                                                            el("span", {
                                                                                "class": "house",
                                                                                get children() {
                                                                                    return live(() => u.houseName);
                                                                                }
                                                                            }),
                                                                            When({
                                                                                get "when"() {
                                                                                    return u.linkkeysDomain && u.linkkeysUserId;
                                                                                },
                                                                                get children() {
                                                                                    return [
                                                                                        el("span", {
                                                                                            "class": "dot"
                                                                                        }),
                                                                                        el("span", {
                                                                                            "class": "ident",
                                                                                            get children() {
                                                                                                return [
                                                                                                    live(() => u.linkkeysUserId),
                                                                                                    "@",
                                                                                                    live(() => u.linkkeysDomain)
                                                                                                ];
                                                                                            }
                                                                                        })
                                                                                    ];
                                                                                }
                                                                            }),
                                                                            When({
                                                                                get "when"() {
                                                                                    return u.roles.length > 0;
                                                                                },
                                                                                get children() {
                                                                                    return [
                                                                                        el("span", {
                                                                                            "class": "dot"
                                                                                        }),
                                                                                        el("span", {
                                                                                            "class": "roles",
                                                                                            get children() {
                                                                                                return live(() => u.roles.join(", "));
                                                                                            }
                                                                                        })
                                                                                    ];
                                                                                }
                                                                            })
                                                                        ];
                                                                    }
                                                                })
                                                            ];
                                                        }
                                                    });
                                                }
                                            }))
                                        });
                                    }
                                });
                            }
                        })
                    ];
                }
            });
        }
    }));
};
