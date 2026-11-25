//! Generated service traits from CSIL specification

use super::types::*;

/// AuthService service trait
pub trait AuthService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// begin-browser-login (request/response).
    fn begin_browser_login(
        &self,
        ctx: &Self::Context,
        input: BeginBrowserLoginRequest,
    ) -> Result<BeginBrowserLoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-act-as-scope-set (request/response).
    fn get_act_as_scope_set(
        &self,
        ctx: &Self::Context,
        input: ActAsScopeSetRequest,
    ) -> Result<ActAsScopeSetResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// exchange-act-as-credential (request/response).
    fn exchange_act_as_credential(
        &self,
        ctx: &Self::Context,
        input: ExchangeActAsCredentialRequest,
    ) -> Result<ActAsTokenResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// login (request/response).
    fn login(
        &self,
        ctx: &Self::Context,
        input: LoginRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// complete (request/response).
    fn complete(
        &self,
        ctx: &Self::Context,
        input: CompleteRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// refresh (request/response).
    fn refresh(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// logout (request/response).
    fn logout(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// resume-session (request/response).
    fn resume_session(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<LoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// end-session (request/response).
    fn end_session(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// me (request/response).
    fn me(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<MeResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// begin-cli-login (request/response).
    fn begin_cli_login(
        &self,
        ctx: &Self::Context,
        input: BeginCliLoginRequest,
    ) -> Result<BeginCliLoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// inspect-cli-login (request/response).
    fn inspect_cli_login(
        &self,
        ctx: &Self::Context,
        input: ApproveCliLoginRequest,
    ) -> Result<CliLoginRequestInfo, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// approve-cli-login (request/response).
    fn approve_cli_login(
        &self,
        ctx: &Self::Context,
        input: ApproveCliLoginRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// deny-cli-login (request/response).
    fn deny_cli_login(
        &self,
        ctx: &Self::Context,
        input: DenyCliLoginRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// exchange-cli-login (request/response).
    fn exchange_cli_login(
        &self,
        ctx: &Self::Context,
        input: ExchangeCliLoginRequest,
    ) -> Result<ExchangeCliLoginResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// refresh-session (request/response).
    fn refresh_session(
        &self,
        ctx: &Self::Context,
        input: RefreshSessionRequest,
    ) -> Result<CliTokenResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-sessions (request/response).
    fn list_sessions(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<CliSessionsResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// revoke-session (request/response).
    fn revoke_session(
        &self,
        ctx: &Self::Context,
        input: RevokeSessionRequest,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// DevAuthService service trait
pub trait DevAuthService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-dev-users (request/response).
    fn list_dev_users(
        &self,
        ctx: &Self::Context,
        input: EmptyRequest,
    ) -> Result<DevUsersResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// dev-login (request/response).
    fn dev_login(
        &self,
        ctx: &Self::Context,
        input: DevLoginRequest,
    ) -> Result<LoginResponse, ServiceError>;
}

/// HouseService service trait
pub trait HouseService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-house (request/response).
    fn create_house(&self, ctx: &Self::Context, input: House) -> Result<House, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-house (request/response).
    fn get_house(&self, ctx: &Self::Context, input: HouseID) -> Result<House, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-house (request/response).
    fn update_house(&self, ctx: &Self::Context, input: House) -> Result<House, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-house (request/response).
    fn delete_house(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-houses (request/response).
    fn list_houses(
        &self,
        ctx: &Self::Context,
        input: HouseListRequest,
    ) -> Result<HouseItems, ServiceError>;
}

/// MemberService service trait
pub trait MemberService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-member (request/response).
    fn create_member(&self, ctx: &Self::Context, input: Member) -> Result<Member, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-member (request/response).
    fn get_member(&self, ctx: &Self::Context, input: MemberID) -> Result<Member, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-member-by-identity (request/response).
    fn get_member_by_identity(
        &self,
        ctx: &Self::Context,
        input: Member,
    ) -> Result<Member, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-member (request/response).
    fn update_member(&self, ctx: &Self::Context, input: Member) -> Result<Member, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// deactivate-member (request/response).
    fn deactivate_member(
        &self,
        ctx: &Self::Context,
        input: MemberID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// reactivate-member (request/response).
    fn reactivate_member(
        &self,
        ctx: &Self::Context,
        input: MemberID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-members (request/response).
    fn list_members(
        &self,
        ctx: &Self::Context,
        input: HouseScopedListRequest,
    ) -> Result<MemberItems, ServiceError>;
}

/// TrustedDomainService service trait
pub trait TrustedDomainService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// add-trusted-domain (request/response).
    fn add_trusted_domain(
        &self,
        ctx: &Self::Context,
        input: TrustedDomain,
    ) -> Result<TrustedDomain, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-trusted-domain (request/response).
    fn remove_trusted_domain(
        &self,
        ctx: &Self::Context,
        input: TrustedDomainID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-trusted-domains (request/response).
    fn list_trusted_domains(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<TrustedDomainItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// is-domain-trusted (request/response).
    fn is_domain_trusted(
        &self,
        ctx: &Self::Context,
        input: TrustedDomain,
    ) -> Result<BoolResponse, ServiceError>;
}

/// RoleService service trait
pub trait RoleService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-role (request/response).
    fn create_role(&self, ctx: &Self::Context, input: Role) -> Result<Role, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-role (request/response).
    fn update_role(&self, ctx: &Self::Context, input: Role) -> Result<Role, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-role (request/response).
    fn delete_role(
        &self,
        ctx: &Self::Context,
        input: RoleID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-roles (request/response).
    fn list_roles(
        &self,
        ctx: &Self::Context,
        input: HouseScopedListRequest,
    ) -> Result<RoleItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// grant-role (request/response).
    fn grant_role(
        &self,
        ctx: &Self::Context,
        input: MemberRoleRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// revoke-role (request/response).
    fn revoke_role(
        &self,
        ctx: &Self::Context,
        input: MemberRoleRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-member-roles (request/response).
    fn list_member_roles(
        &self,
        ctx: &Self::Context,
        input: MemberScopedListRequest,
    ) -> Result<RoleItems, ServiceError>;
}

/// SkillService service trait
pub trait SkillService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-skill (request/response).
    fn create_skill(&self, ctx: &Self::Context, input: Skill) -> Result<Skill, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-skill (request/response).
    fn update_skill(&self, ctx: &Self::Context, input: Skill) -> Result<Skill, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-skill (request/response).
    fn delete_skill(
        &self,
        ctx: &Self::Context,
        input: SkillID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-skills (request/response).
    fn list_skills(
        &self,
        ctx: &Self::Context,
        input: HouseScopedListRequest,
    ) -> Result<SkillItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// add-member-skill (request/response).
    fn add_member_skill(
        &self,
        ctx: &Self::Context,
        input: MemberSkillRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-member-skill (request/response).
    fn remove_member_skill(
        &self,
        ctx: &Self::Context,
        input: MemberSkillRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-member-skills (request/response).
    fn list_member_skills(
        &self,
        ctx: &Self::Context,
        input: MemberScopedListRequest,
    ) -> Result<SkillItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// add-group-skill (request/response).
    fn add_group_skill(
        &self,
        ctx: &Self::Context,
        input: GroupSkillRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-group-skill (request/response).
    fn remove_group_skill(
        &self,
        ctx: &Self::Context,
        input: GroupSkillRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-group-skills (request/response).
    fn list_group_skills(
        &self,
        ctx: &Self::Context,
        input: GroupID,
    ) -> Result<SkillItems, ServiceError>;
}

/// GroupService service trait
pub trait GroupService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-group (request/response).
    fn create_group(&self, ctx: &Self::Context, input: Group) -> Result<Group, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-group (request/response).
    fn update_group(&self, ctx: &Self::Context, input: Group) -> Result<Group, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-group (request/response).
    fn delete_group(
        &self,
        ctx: &Self::Context,
        input: GroupID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-groups (request/response).
    fn list_groups(
        &self,
        ctx: &Self::Context,
        input: HouseScopedListRequest,
    ) -> Result<GroupItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// add-group-member (request/response).
    fn add_group_member(
        &self,
        ctx: &Self::Context,
        input: GroupMemberRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-group-member (request/response).
    fn remove_group_member(
        &self,
        ctx: &Self::Context,
        input: GroupMemberRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-group-members (request/response).
    fn list_group_members(
        &self,
        ctx: &Self::Context,
        input: MemberScopedListRequest,
    ) -> Result<MemberItems, ServiceError>;
}

/// ProjectService service trait
pub trait ProjectService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-project (request/response).
    fn create_project(&self, ctx: &Self::Context, input: Project) -> Result<Project, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-project (request/response).
    fn get_project(&self, ctx: &Self::Context, input: ProjectID) -> Result<Project, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-project (request/response).
    fn update_project(&self, ctx: &Self::Context, input: Project) -> Result<Project, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-project (request/response).
    fn delete_project(
        &self,
        ctx: &Self::Context,
        input: ProjectID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-projects (request/response).
    fn list_projects(
        &self,
        ctx: &Self::Context,
        input: HouseScopedListRequest,
    ) -> Result<ProjectList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-project-tasks (request/response).
    fn list_project_tasks(
        &self,
        ctx: &Self::Context,
        input: ProjectScopedListRequest,
    ) -> Result<TaskList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// add-project-task (request/response).
    fn add_project_task(
        &self,
        ctx: &Self::Context,
        input: ProjectTaskOrderRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-project-task (request/response).
    fn remove_project_task(
        &self,
        ctx: &Self::Context,
        input: ProjectTaskRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// set-project-task-position (request/response).
    fn set_project_task_position(
        &self,
        ctx: &Self::Context,
        input: ProjectTaskOrderRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-project-members (request/response).
    fn list_project_members(
        &self,
        ctx: &Self::Context,
        input: ProjectID,
    ) -> Result<MemberItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// add-project-member (request/response).
    fn add_project_member(
        &self,
        ctx: &Self::Context,
        input: ProjectMemberRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-project-member (request/response).
    fn remove_project_member(
        &self,
        ctx: &Self::Context,
        input: ProjectMemberRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-project-owners (request/response).
    fn list_project_owners(
        &self,
        ctx: &Self::Context,
        input: ProjectID,
    ) -> Result<MemberItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// add-project-owner (request/response).
    fn add_project_owner(
        &self,
        ctx: &Self::Context,
        input: ProjectOwnerRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-project-owner (request/response).
    fn remove_project_owner(
        &self,
        ctx: &Self::Context,
        input: ProjectOwnerRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-milestones (request/response).
    fn list_milestones(
        &self,
        ctx: &Self::Context,
        input: ProjectID,
    ) -> Result<MilestoneItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-milestone (request/response).
    fn create_milestone(
        &self,
        ctx: &Self::Context,
        input: Milestone,
    ) -> Result<Milestone, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-milestone (request/response).
    fn update_milestone(
        &self,
        ctx: &Self::Context,
        input: Milestone,
    ) -> Result<Milestone, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-milestone (request/response).
    fn delete_milestone(
        &self,
        ctx: &Self::Context,
        input: MilestoneID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// set-project-visibility (request/response).
    fn set_project_visibility(
        &self,
        ctx: &Self::Context,
        input: SetProjectVisibilityRequest,
    ) -> Result<Project, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-project-grants (request/response).
    fn list_project_grants(
        &self,
        ctx: &Self::Context,
        input: ProjectID,
    ) -> Result<GrantItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// put-project-grant (request/response).
    fn put_project_grant(
        &self,
        ctx: &Self::Context,
        input: PutProjectGrantRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-project-grant (request/response).
    fn delete_project_grant(
        &self,
        ctx: &Self::Context,
        input: ProjectGrantRef,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// EventService service trait
pub trait EventService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-event (request/response).
    fn create_event(&self, ctx: &Self::Context, input: Event) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-event (request/response).
    fn get_event(&self, ctx: &Self::Context, input: EventID) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-event (request/response).
    fn update_event(&self, ctx: &Self::Context, input: Event) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-event (request/response).
    fn delete_event(
        &self,
        ctx: &Self::Context,
        input: EventID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-event-and-future (request/response).
    fn delete_event_and_future(
        &self,
        ctx: &Self::Context,
        input: EventID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-events (request/response).
    fn list_events(
        &self,
        ctx: &Self::Context,
        input: EventListRequest,
    ) -> Result<EventItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-calendar-view (request/response).
    fn get_calendar_view(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<CalendarView, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// set-calendar-view (request/response).
    fn set_calendar_view(
        &self,
        ctx: &Self::Context,
        input: CalendarView,
    ) -> Result<CalendarView, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// accept-event (request/response).
    fn accept_event(
        &self,
        ctx: &Self::Context,
        input: EventDecision,
    ) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// recover-edit (request/response).
    fn recover_edit(&self, ctx: &Self::Context, input: EventID) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// decline-event (request/response).
    fn decline_event(
        &self,
        ctx: &Self::Context,
        input: EventDecision,
    ) -> Result<Event, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-local-events (request/response).
    fn list_local_events(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<EventItems, ServiceError>;
}

/// CalendarService service trait
pub trait CalendarService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-project-deadlines (request/response).
    fn list_project_deadlines(
        &self,
        ctx: &Self::Context,
        input: ProjectDeadlineRequest,
    ) -> Result<ProjectDeadlineList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-status (request/response).
    fn get_status(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<CalendarStatus, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// configure-calendar (request/response).
    fn configure_calendar(
        &self,
        ctx: &Self::Context,
        input: ConfigureCalendarRequest,
    ) -> Result<CalendarStatus, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// begin-connect (request/response).
    fn begin_connect(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<CalendarConnectResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// complete-connect (request/response).
    fn complete_connect(
        &self,
        ctx: &Self::Context,
        input: CompleteCalendarConnectRequest,
    ) -> Result<CalendarStatus, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// disconnect (request/response).
    fn disconnect(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// preview-migration (request/response).
    fn preview_migration(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<CalendarMigrationRowItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-trash (request/response).
    fn list_trash(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<CalendarTrashEntryItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// restore-event (request/response).
    fn restore_event(
        &self,
        ctx: &Self::Context,
        input: RestoreCalendarEventRequest,
    ) -> Result<Event, ServiceError>;
}

/// TaskService service trait
pub trait TaskService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-task (request/response).
    fn create_task(&self, ctx: &Self::Context, input: Task) -> Result<Task, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-task (request/response).
    fn get_task(&self, ctx: &Self::Context, input: TaskID) -> Result<Task, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-task (request/response).
    fn update_task(&self, ctx: &Self::Context, input: Task) -> Result<Task, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-task (request/response).
    fn delete_task(
        &self,
        ctx: &Self::Context,
        input: TaskID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-tasks (request/response).
    fn list_tasks(
        &self,
        ctx: &Self::Context,
        input: TaskListRequest,
    ) -> Result<TaskList, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// create-personal-task (request/response).
    fn create_personal_task(&self, ctx: &Self::Context, input: Task) -> Result<Task, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// set-task-visibility (request/response).
    fn set_task_visibility(
        &self,
        ctx: &Self::Context,
        input: SetTaskVisibilityRequest,
    ) -> Result<Task, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-task-grants (request/response).
    fn list_task_grants(
        &self,
        ctx: &Self::Context,
        input: TaskID,
    ) -> Result<GrantItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// put-task-grant (request/response).
    fn put_task_grant(
        &self,
        ctx: &Self::Context,
        input: PutTaskGrantRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-task-grant (request/response).
    fn delete_task_grant(
        &self,
        ctx: &Self::Context,
        input: TaskGrantRef,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// DependencyService service trait
pub trait DependencyService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// add-dependency (request/response).
    fn add_dependency(
        &self,
        ctx: &Self::Context,
        input: DependencyRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// remove-dependency (request/response).
    fn remove_dependency(
        &self,
        ctx: &Self::Context,
        input: DependencyRef,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-dependencies (request/response).
    fn get_dependencies(
        &self,
        ctx: &Self::Context,
        input: DependencyTarget,
    ) -> Result<DependencyGraph, ServiceError>;
}

/// CommentService service trait
pub trait CommentService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-comment (request/response).
    fn create_comment(&self, ctx: &Self::Context, input: Comment) -> Result<Comment, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// get-comment (request/response).
    fn get_comment(&self, ctx: &Self::Context, input: CommentID) -> Result<Comment, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-comment (request/response).
    fn update_comment(&self, ctx: &Self::Context, input: Comment) -> Result<Comment, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-comment (request/response).
    fn delete_comment(
        &self,
        ctx: &Self::Context,
        input: CommentID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-comments (request/response).
    fn list_comments(
        &self,
        ctx: &Self::Context,
        input: CommentListRequest,
    ) -> Result<CommentItems, ServiceError>;
}

/// NotificationService service trait
pub trait NotificationService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-notifications (request/response).
    fn list_notifications(
        &self,
        ctx: &Self::Context,
        input: NotificationListRequest,
    ) -> Result<NotificationItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// unread-count (request/response).
    fn unread_count(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<NotificationUnreadCount, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// mark-read (request/response).
    fn mark_read(
        &self,
        ctx: &Self::Context,
        input: NotificationID,
    ) -> Result<Notification, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// mark-all-read (request/response).
    fn mark_all_read(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<EmptyResponse, ServiceError>;
}

/// ShareService service trait
pub trait ShareService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// create-share (request/response).
    fn create_share(&self, ctx: &Self::Context, input: Share) -> Result<Share, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// delete-share (request/response).
    fn delete_share(
        &self,
        ctx: &Self::Context,
        input: ShareID,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// list-shares-by-resource (request/response).
    fn list_shares_by_resource(
        &self,
        ctx: &Self::Context,
        input: ResourceRef,
    ) -> Result<ShareItems, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// check-access (request/response).
    fn check_access(
        &self,
        ctx: &Self::Context,
        input: ShareAccessRequest,
    ) -> Result<Share, ServiceError>;
}

/// MemberAuditService service trait
pub trait MemberAuditService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-audits-for-member (request/response).
    fn list_audits_for_member(
        &self,
        ctx: &Self::Context,
        input: MemberScopedListRequest,
    ) -> Result<MemberAuditItems, ServiceError>;
}

/// SettingsService service trait
pub trait SettingsService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// get-settings (request/response).
    fn get_settings(
        &self,
        ctx: &Self::Context,
        input: HouseID,
    ) -> Result<EffectiveSettings, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// update-settings (request/response).
    fn update_settings(
        &self,
        ctx: &Self::Context,
        input: UpdateSettingsRequest,
    ) -> Result<EffectiveSettings, ServiceError>;
}

/// BugService service trait
pub trait BugService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// report-bug (request/response).
    fn report_bug(
        &self,
        ctx: &Self::Context,
        input: BugReportRequest,
    ) -> Result<Task, ServiceError>;
}

/// AuditService service trait
pub trait AuditService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// query-audit (request/response).
    fn query_audit(
        &self,
        ctx: &Self::Context,
        input: AuditQuery,
    ) -> Result<AuditPage, ServiceError>;
}

/// TrashService service trait
pub trait TrashService {
    type Context;
    #[allow(clippy::result_large_err)]
    /// list-trash (request/response).
    fn list_trash(
        &self,
        ctx: &Self::Context,
        input: HouseScopedListRequest,
    ) -> Result<TrashPage, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// restore (request/response).
    fn restore(
        &self,
        ctx: &Self::Context,
        input: RestoreRequest,
    ) -> Result<EmptyResponse, ServiceError>;
    #[allow(clippy::result_large_err)]
    /// purge (request/response).
    fn purge(
        &self,
        ctx: &Self::Context,
        input: PurgeRequest,
    ) -> Result<EmptyResponse, ServiceError>;
}
