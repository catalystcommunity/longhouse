import { Route, Router } from "@solidjs/router";
import { lazy } from "solid-js";
import { AppShell } from "~/layouts/AppShell";
import { Dashboard } from "~/pages/Dashboard";
import { CalendarPage } from "~/pages/Calendar";
import { ProjectsPage } from "~/pages/Projects";
import { ProjectDetail } from "~/pages/ProjectDetail";
import { TasksPage } from "~/pages/Tasks";
import { MembersPage } from "~/pages/Members";
import { AuthCallback } from "~/pages/AuthCallback";
import { Stub } from "~/pages/Stub";

// /dev-login exists only in dev builds. The conditional below makes the
// import statically dead-code in prod, so Vite tree-shakes the page module
// out of the production bundle entirely.
const DevLogin = import.meta.env.DEV
  ? lazy(() => import("~/pages/DevLogin").then((m) => ({ default: m.DevLogin })))
  : null;

export const App = () => (
  <Router root={AppShell}>
    <Route path="/" component={Dashboard} />
    <Route path="/tasks" component={TasksPage} />
    <Route path="/calendar" component={CalendarPage} />
    <Route path="/events" component={CalendarPage} />
    <Route path="/projects" component={ProjectsPage} />
    <Route path="/projects/:slug" component={ProjectDetail} />
    <Route path="/members" component={MembersPage} />
    <Route path="/auth/callback" component={AuthCallback} />
    <Route path="/shares" component={Stub} />
    <Route path="/more" component={Stub} />
    {DevLogin && <Route path="/dev-login" component={DevLogin} />}
    <Route path="*" component={Stub} />
  </Router>
);
