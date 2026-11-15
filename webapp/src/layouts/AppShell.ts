import { type ParentView, live, el } from "~/ui/dom";
import { Header } from "~/components/Header";
import { Tabbar } from "~/components/Tabbar";
// Vite uses the release image version from CI, or VERSION.txt for local builds.
declare const __APP_VERSION__: string;
export const AppShell: ParentView = (props) => ([
    el("div", {
        "class": "shell",
        get children() {
            return [
                Header(),
                live(() => props.children),
                el("footer", {
                    "class": "foot",
                    get children() {
                        return [
                            el("span", {
                                get children() {
                                    return [
                                        "Longhouse \u00B7 v",
                                        live(() => __APP_VERSION__)
                                    ];
                                }
                            }),
                            el("a", {
                                "href": "https://github.com/catalystcommunity/longhouse",
                                "target": "_blank",
                                "rel": "noopener noreferrer",
                                get children() {
                                    return "source";
                                }
                            })
                        ];
                    }
                })
            ];
        }
    }),
    Tabbar()
]);
