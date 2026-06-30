//! Generated service traits from CSIL specification

use super::types::*;

/// AuthService service trait
pub trait AuthService {
    type Context;
    /// login (request/response).
    fn login(&self, ctx: &Self::Context, input: LoginRequest) -> Result<LoginResponse, ServiceError>;
    /// complete (request/response).
    fn complete(&self, ctx: &Self::Context, input: CompleteRequest) -> Result<LoginResponse, ServiceError>;
    /// refresh (request/response).
    fn refresh(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<LoginResponse, ServiceError>;
    /// logout (request/response).
    fn logout(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<EmptyResponse, ServiceError>;
    /// me (request/response).
    fn me(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<MeResponse, ServiceError>;
}

/// DevAuthService service trait
pub trait DevAuthService {
    type Context;
    /// list-dev-users (request/response).
    fn list_dev_users(&self, ctx: &Self::Context, input: EmptyRequest) -> Result<DevUsersResponse, ServiceError>;
    /// dev-login (request/response).
    fn dev_login(&self, ctx: &Self::Context, input: DevLoginRequest) -> Result<LoginResponse, ServiceError>;
}

/// HouseService service trait
pub trait HouseService {
    type Context;
    /// create-house (request/response).
    fn create_house(&self, ctx: &Self::Context, input: House) -> Result<House, ServiceError>;
    /// get-house (request/response).
    fn get_house(&self, ctx: &Self::Context, input: HouseID) -> Result<House, ServiceError>;
    /// update-house (request/response).
    fn update_house(&self, ctx: &Self::Context, input: House) -> Result<House, ServiceError>;
    /// delete-house (request/response).
    fn delete_house(&self, ctx: &Self::Context, input: HouseID) -> Result<EmptyResponse, ServiceError>;
    /// list-houses (request/response).
    fn list_houses(&self, ctx: &Self::Context, input: HouseListRequest) -> Result<Vec<House>, ServiceError>;
}

/// MemberService service trait
pub trait MemberService {
    type Context;
    /// create-member (request/response).
    fn create_member(&self, ctx: &Self::Context, input: Member) -> Result<Member, ServiceError>;
    /// get-member (request/response).
    fn get_member(&self, ctx: &Self::Context, input: MemberID) -> Result<Member, ServiceError>;
    /// get-member-by-identity (request/response).
    fn get_member_by_identity(&self, ctx: &Self::Context, input: Member) -> Result<Member, ServiceError>;
    /// update-member (request/response).
    fn update_member(&self, ctx: &Self::Context, input: Member) -> Result<Member, ServiceError>;
    /// deactivate-member (request/response).
    fn deactivate_member(&self, ctx: &Self::Context, input: MemberID) -> Result<EmptyResponse, ServiceError>;
    /// reactivate-member (request/response).
    fn reactivate_member(&self, ctx: &Self::Context, input: MemberID) -> Result<EmptyResponse, ServiceError>;
    /// list-members (request/response).
    fn list_members(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<Vec<Member>, ServiceError>;
}

/// TrustedDomainService service trait
pub trait TrustedDomainService {
    type Context;
    /// add-trusted-domain (request/response).
    fn add_trusted_domain(&self, ctx: &Self::Context, input: TrustedDomain) -> Result<TrustedDomain, ServiceError>;
    /// remove-trusted-domain (request/response).
    fn remove_trusted_domain(&self, ctx: &Self::Context, input: TrustedDomainID) -> Result<EmptyResponse, ServiceError>;
    /// list-trusted-domains (request/response).
    fn list_trusted_domains(&self, ctx: &Self::Context, input: HouseID) -> Result<Vec<TrustedDomain>, ServiceError>;
    /// is-domain-trusted (request/response).
    fn is_domain_trusted(&self, ctx: &Self::Context, input: TrustedDomain) -> Result<BoolResponse, ServiceError>;
}

/// RoleService service trait
pub trait RoleService {
    type Context;
    /// create-role (request/response).
    fn create_role(&self, ctx: &Self::Context, input: Role) -> Result<Role, ServiceError>;
    /// update-role (request/response).
    fn update_role(&self, ctx: &Self::Context, input: Role) -> Result<Role, ServiceError>;
    /// delete-role (request/response).
    fn delete_role(&self, ctx: &Self::Context, input: RoleID) -> Result<EmptyResponse, ServiceError>;
    /// list-roles (request/response).
    fn list_roles(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<Vec<Role>, ServiceError>;
    /// grant-role (request/response).
    fn grant_role(&self, ctx: &Self::Context, input: MemberRoleRef) -> Result<EmptyResponse, ServiceError>;
    /// revoke-role (request/response).
    fn revoke_role(&self, ctx: &Self::Context, input: MemberRoleRef) -> Result<EmptyResponse, ServiceError>;
    /// list-member-roles (request/response).
    fn list_member_roles(&self, ctx: &Self::Context, input: MemberScopedListRequest) -> Result<Vec<Role>, ServiceError>;
}

/// SkillService service trait
pub trait SkillService {
    type Context;
    /// create-skill (request/response).
    fn create_skill(&self, ctx: &Self::Context, input: Skill) -> Result<Skill, ServiceError>;
    /// update-skill (request/response).
    fn update_skill(&self, ctx: &Self::Context, input: Skill) -> Result<Skill, ServiceError>;
    /// delete-skill (request/response).
    fn delete_skill(&self, ctx: &Self::Context, input: SkillID) -> Result<EmptyResponse, ServiceError>;
    /// list-skills (request/response).
    fn list_skills(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<Vec<Skill>, ServiceError>;
    /// add-member-skill (request/response).
    fn add_member_skill(&self, ctx: &Self::Context, input: MemberSkillRef) -> Result<EmptyResponse, ServiceError>;
    /// remove-member-skill (request/response).
    fn remove_member_skill(&self, ctx: &Self::Context, input: MemberSkillRef) -> Result<EmptyResponse, ServiceError>;
    /// list-member-skills (request/response).
    fn list_member_skills(&self, ctx: &Self::Context, input: MemberScopedListRequest) -> Result<Vec<Skill>, ServiceError>;
    /// add-group-skill (request/response).
    fn add_group_skill(&self, ctx: &Self::Context, input: GroupSkillRef) -> Result<EmptyResponse, ServiceError>;
    /// remove-group-skill (request/response).
    fn remove_group_skill(&self, ctx: &Self::Context, input: GroupSkillRef) -> Result<EmptyResponse, ServiceError>;
    /// list-group-skills (request/response).
    fn list_group_skills(&self, ctx: &Self::Context, input: GroupID) -> Result<Vec<Skill>, ServiceError>;
}

/// GroupService service trait
pub trait GroupService {
    type Context;
    /// create-group (request/response).
    fn create_group(&self, ctx: &Self::Context, input: Group) -> Result<Group, ServiceError>;
    /// update-group (request/response).
    fn update_group(&self, ctx: &Self::Context, input: Group) -> Result<Group, ServiceError>;
    /// delete-group (request/response).
    fn delete_group(&self, ctx: &Self::Context, input: GroupID) -> Result<EmptyResponse, ServiceError>;
    /// list-groups (request/response).
    fn list_groups(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<Vec<Group>, ServiceError>;
    /// add-group-member (request/response).
    fn add_group_member(&self, ctx: &Self::Context, input: GroupMemberRef) -> Result<EmptyResponse, ServiceError>;
    /// remove-group-member (request/response).
    fn remove_group_member(&self, ctx: &Self::Context, input: GroupMemberRef) -> Result<EmptyResponse, ServiceError>;
    /// list-group-members (request/response).
    fn list_group_members(&self, ctx: &Self::Context, input: MemberScopedListRequest) -> Result<Vec<Member>, ServiceError>;
}

/// ProjectService service trait
pub trait ProjectService {
    type Context;
    /// create-project (request/response).
    fn create_project(&self, ctx: &Self::Context, input: Project) -> Result<Project, ServiceError>;
    /// get-project (request/response).
    fn get_project(&self, ctx: &Self::Context, input: ProjectID) -> Result<Project, ServiceError>;
    /// update-project (request/response).
    fn update_project(&self, ctx: &Self::Context, input: Project) -> Result<Project, ServiceError>;
    /// delete-project (request/response).
    fn delete_project(&self, ctx: &Self::Context, input: ProjectID) -> Result<EmptyResponse, ServiceError>;
    /// list-projects (request/response).
    fn list_projects(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<ProjectList, ServiceError>;
    /// list-project-tasks (request/response).
    fn list_project_tasks(&self, ctx: &Self::Context, input: ProjectScopedListRequest) -> Result<TaskList, ServiceError>;
    /// add-project-task (request/response).
    fn add_project_task(&self, ctx: &Self::Context, input: ProjectTaskOrderRequest) -> Result<EmptyResponse, ServiceError>;
    /// remove-project-task (request/response).
    fn remove_project_task(&self, ctx: &Self::Context, input: ProjectTaskRef) -> Result<EmptyResponse, ServiceError>;
    /// set-project-task-position (request/response).
    fn set_project_task_position(&self, ctx: &Self::Context, input: ProjectTaskOrderRequest) -> Result<EmptyResponse, ServiceError>;
    /// list-project-members (request/response).
    fn list_project_members(&self, ctx: &Self::Context, input: ProjectID) -> Result<Vec<Member>, ServiceError>;
    /// add-project-member (request/response).
    fn add_project_member(&self, ctx: &Self::Context, input: ProjectMemberRef) -> Result<EmptyResponse, ServiceError>;
    /// remove-project-member (request/response).
    fn remove_project_member(&self, ctx: &Self::Context, input: ProjectMemberRef) -> Result<EmptyResponse, ServiceError>;
    /// list-project-owners (request/response).
    fn list_project_owners(&self, ctx: &Self::Context, input: ProjectID) -> Result<Vec<Member>, ServiceError>;
    /// add-project-owner (request/response).
    fn add_project_owner(&self, ctx: &Self::Context, input: ProjectOwnerRef) -> Result<EmptyResponse, ServiceError>;
    /// remove-project-owner (request/response).
    fn remove_project_owner(&self, ctx: &Self::Context, input: ProjectOwnerRef) -> Result<EmptyResponse, ServiceError>;
    /// list-milestones (request/response).
    fn list_milestones(&self, ctx: &Self::Context, input: ProjectID) -> Result<Vec<Milestone>, ServiceError>;
    /// create-milestone (request/response).
    fn create_milestone(&self, ctx: &Self::Context, input: Milestone) -> Result<Milestone, ServiceError>;
    /// update-milestone (request/response).
    fn update_milestone(&self, ctx: &Self::Context, input: Milestone) -> Result<Milestone, ServiceError>;
    /// delete-milestone (request/response).
    fn delete_milestone(&self, ctx: &Self::Context, input: MilestoneID) -> Result<EmptyResponse, ServiceError>;
    /// set-project-visibility (request/response).
    fn set_project_visibility(&self, ctx: &Self::Context, input: SetProjectVisibilityRequest) -> Result<Project, ServiceError>;
    /// list-project-grants (request/response).
    fn list_project_grants(&self, ctx: &Self::Context, input: ProjectID) -> Result<Vec<Grant>, ServiceError>;
    /// put-project-grant (request/response).
    fn put_project_grant(&self, ctx: &Self::Context, input: PutProjectGrantRequest) -> Result<EmptyResponse, ServiceError>;
    /// delete-project-grant (request/response).
    fn delete_project_grant(&self, ctx: &Self::Context, input: ProjectGrantRef) -> Result<EmptyResponse, ServiceError>;
}

/// EventService service trait
pub trait EventService {
    type Context;
    /// create-event (request/response).
    fn create_event(&self, ctx: &Self::Context, input: Event) -> Result<Event, ServiceError>;
    /// get-event (request/response).
    fn get_event(&self, ctx: &Self::Context, input: EventID) -> Result<Event, ServiceError>;
    /// update-event (request/response).
    fn update_event(&self, ctx: &Self::Context, input: Event) -> Result<Event, ServiceError>;
    /// delete-event (request/response).
    fn delete_event(&self, ctx: &Self::Context, input: EventID) -> Result<EmptyResponse, ServiceError>;
    /// delete-event-and-future (request/response).
    fn delete_event_and_future(&self, ctx: &Self::Context, input: EventID) -> Result<EmptyResponse, ServiceError>;
    /// list-events (request/response).
    fn list_events(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<Vec<Event>, ServiceError>;
}

/// TaskService service trait
pub trait TaskService {
    type Context;
    /// create-task (request/response).
    fn create_task(&self, ctx: &Self::Context, input: Task) -> Result<Task, ServiceError>;
    /// get-task (request/response).
    fn get_task(&self, ctx: &Self::Context, input: TaskID) -> Result<Task, ServiceError>;
    /// update-task (request/response).
    fn update_task(&self, ctx: &Self::Context, input: Task) -> Result<Task, ServiceError>;
    /// delete-task (request/response).
    fn delete_task(&self, ctx: &Self::Context, input: TaskID) -> Result<EmptyResponse, ServiceError>;
    /// list-tasks (request/response).
    fn list_tasks(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<TaskList, ServiceError>;
    /// set-task-visibility (request/response).
    fn set_task_visibility(&self, ctx: &Self::Context, input: SetTaskVisibilityRequest) -> Result<Task, ServiceError>;
    /// list-task-grants (request/response).
    fn list_task_grants(&self, ctx: &Self::Context, input: TaskID) -> Result<Vec<Grant>, ServiceError>;
    /// put-task-grant (request/response).
    fn put_task_grant(&self, ctx: &Self::Context, input: PutTaskGrantRequest) -> Result<EmptyResponse, ServiceError>;
    /// delete-task-grant (request/response).
    fn delete_task_grant(&self, ctx: &Self::Context, input: TaskGrantRef) -> Result<EmptyResponse, ServiceError>;
}

/// DependencyService service trait
pub trait DependencyService {
    type Context;
    /// add-dependency (request/response).
    fn add_dependency(&self, ctx: &Self::Context, input: DependencyRef) -> Result<EmptyResponse, ServiceError>;
    /// remove-dependency (request/response).
    fn remove_dependency(&self, ctx: &Self::Context, input: DependencyRef) -> Result<EmptyResponse, ServiceError>;
    /// get-dependencies (request/response).
    fn get_dependencies(&self, ctx: &Self::Context, input: DependencyTarget) -> Result<DependencyGraph, ServiceError>;
}

/// CommentService service trait
pub trait CommentService {
    type Context;
    /// create-comment (request/response).
    fn create_comment(&self, ctx: &Self::Context, input: Comment) -> Result<Comment, ServiceError>;
    /// get-comment (request/response).
    fn get_comment(&self, ctx: &Self::Context, input: CommentID) -> Result<Comment, ServiceError>;
    /// update-comment (request/response).
    fn update_comment(&self, ctx: &Self::Context, input: Comment) -> Result<Comment, ServiceError>;
    /// delete-comment (request/response).
    fn delete_comment(&self, ctx: &Self::Context, input: CommentID) -> Result<EmptyResponse, ServiceError>;
    /// list-comments (request/response).
    fn list_comments(&self, ctx: &Self::Context, input: CommentListRequest) -> Result<Vec<Comment>, ServiceError>;
}

/// NotificationService service trait
pub trait NotificationService {
    type Context;
    /// list-notifications (request/response).
    fn list_notifications(&self, ctx: &Self::Context, input: NotificationListRequest) -> Result<Vec<Notification>, ServiceError>;
    /// unread-count (request/response).
    fn unread_count(&self, ctx: &Self::Context, input: HouseID) -> Result<NotificationUnreadCount, ServiceError>;
    /// mark-read (request/response).
    fn mark_read(&self, ctx: &Self::Context, input: NotificationID) -> Result<Notification, ServiceError>;
    /// mark-all-read (request/response).
    fn mark_all_read(&self, ctx: &Self::Context, input: HouseID) -> Result<EmptyResponse, ServiceError>;
}

/// ShareService service trait
pub trait ShareService {
    type Context;
    /// create-share (request/response).
    fn create_share(&self, ctx: &Self::Context, input: Share) -> Result<Share, ServiceError>;
    /// delete-share (request/response).
    fn delete_share(&self, ctx: &Self::Context, input: ShareID) -> Result<EmptyResponse, ServiceError>;
    /// list-shares-by-resource (request/response).
    fn list_shares_by_resource(&self, ctx: &Self::Context, input: ResourceRef) -> Result<Vec<Share>, ServiceError>;
    /// check-access (request/response).
    fn check_access(&self, ctx: &Self::Context, input: ShareAccessRequest) -> Result<Share, ServiceError>;
}

/// MemberAuditService service trait
pub trait MemberAuditService {
    type Context;
    /// list-audits-for-member (request/response).
    fn list_audits_for_member(&self, ctx: &Self::Context, input: MemberScopedListRequest) -> Result<Vec<MemberAudit>, ServiceError>;
}

/// SettingsService service trait
pub trait SettingsService {
    type Context;
    /// get-settings (request/response).
    fn get_settings(&self, ctx: &Self::Context, input: HouseID) -> Result<EffectiveSettings, ServiceError>;
    /// update-settings (request/response).
    fn update_settings(&self, ctx: &Self::Context, input: UpdateSettingsRequest) -> Result<EffectiveSettings, ServiceError>;
}

/// BugService service trait
pub trait BugService {
    type Context;
    /// report-bug (request/response).
    fn report_bug(&self, ctx: &Self::Context, input: BugReportRequest) -> Result<Task, ServiceError>;
}

/// AuditService service trait
pub trait AuditService {
    type Context;
    /// query-audit (request/response).
    fn query_audit(&self, ctx: &Self::Context, input: AuditQuery) -> Result<AuditPage, ServiceError>;
}

/// TrashService service trait
pub trait TrashService {
    type Context;
    /// list-trash (request/response).
    fn list_trash(&self, ctx: &Self::Context, input: HouseScopedListRequest) -> Result<TrashPage, ServiceError>;
    /// restore (request/response).
    fn restore(&self, ctx: &Self::Context, input: RestoreRequest) -> Result<EmptyResponse, ServiceError>;
    /// purge (request/response).
    fn purge(&self, ctx: &Self::Context, input: PurgeRequest) -> Result<EmptyResponse, ServiceError>;
}

