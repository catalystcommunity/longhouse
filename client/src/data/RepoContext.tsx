import { createContext, useContext, type ParentComponent } from "solid-js";
import type { Repo } from "./repo";
import { repo as defaultRepo } from "./repo";

/**
 * Repo injection. Production wraps the app in <RepoProvider> with the real
 * (currently mock) repo; tests wrap individual components with a fake repo
 * so they never touch the network. Falling back to the module singleton
 * means components rendered without a provider still work.
 */
const RepoContext = createContext<Repo>(defaultRepo);

export const RepoProvider: ParentComponent<{ repo: Repo }> = (props) => (
  <RepoContext.Provider value={props.repo}>{props.children}</RepoContext.Provider>
);

export const useRepo = (): Repo => useContext(RepoContext);
