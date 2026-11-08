import { AppShell } from '~/layouts/AppShell';
import { Dashboard } from '~/pages/Dashboard';
import { CalendarPage } from '~/pages/Calendar';
import { ProjectsPage } from '~/pages/Projects';
import { ProjectDetail } from '~/pages/ProjectDetail';
import { TasksPage } from '~/pages/Tasks';
import { MembersPage } from '~/pages/Members';
import { GroupsPage } from '~/pages/Groups';
import { SkillsPage } from '~/pages/Skills';
import { SettingsPage } from '~/pages/Settings';
import { AccountPage } from '~/pages/Account';
import { MorePage } from '~/pages/More';
import { ActivityTrashPage } from '~/pages/ActivityTrash';
import { AuthCallback } from '~/pages/AuthCallback';
import { Stub } from '~/pages/Stub';
import { CliAuthorizePage } from '~/pages/CliAuthorize';
import { outlet, locationState, type PageRoute } from '~/ui/router';
import { live, When, type Content } from '~/ui/dom';
import { cell, cleanup, currentLifetime, inLifetime, sample } from '~/ui/state';

export const routes: PageRoute[] = [
  { path: '/', page: Dashboard }, { path: '/tasks', page: TasksPage },
  { path: '/calendar', page: CalendarPage }, { path: '/events', page: CalendarPage },
  { path: '/projects', page: ProjectsPage },
  { path: '/members', page: MembersPage }, { path: '/groups', page: GroupsPage },
  { path: '/skills', page: SkillsPage }, { path: '/settings', page: SettingsPage },
  { path: '/admin/activity', page: ActivityTrashPage },
  { path: '/auth/callback', page: AuthCallback }, { path: '/cli/authorize', page: CliAuthorizePage },
  { path: '/shares', page: Stub }, { path: '/more', page: MorePage }, { path: '/account', page: AccountPage },
];
// Prefix matching lets the project page select the detail view without a second router.
const projectRoute = routes.find(route => route.path === '/projects')!;
projectRoute.page = () => When({
  get when() { return locationState().pathname.split('/').filter(Boolean).length > 1; },
  get children() { return ProjectDetail(); },
  get fallback() { return sample(ProjectsPage); },
});
if (import.meta.env.DEV) routes.push({ path: '/dev-login', page: () => {
  const [page, setPage] = cell<Content>(null);
  const owner = currentLifetime();
  let active = true; cleanup(() => { active = false; });
  void import('~/pages/DevLogin').then(module => { if (active) inLifetime(owner, () => setPage(module.DevLogin())); });
  return live(page);
} });
export const App = () => AppShell({ get children() { return outlet(routes, Stub); } });
