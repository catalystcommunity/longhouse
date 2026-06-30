//! Generated types from CSIL specification

/// Returned by a generated `validate` method when a field violates one of its
/// CSIL constraints. `field` names the offending field; `message` explains.
#[derive(Debug, Clone, PartialEq)]
pub struct ValidationError {
    pub field: String,
    pub message: String,
}

impl std::fmt::Display for ValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "validation failed for `{}`: {}", self.field, self.message)
    }
}

impl std::error::Error for ValidationError {}

pub type HouseID = String;

pub type MemberID = String;

pub type RoleID = String;

pub type SkillID = String;

pub type GroupID = String;

pub type ProjectID = String;

pub type EventID = String;

pub type TaskID = String;

pub type CommentID = String;

pub type ShareID = String;

pub type TrustedDomainID = String;

pub type MemberAuditID = String;

pub type MilestoneID = String;

pub type NotificationID = String;

pub type NotificationEventID = String;

pub type Timestamp = String;

/// TaskStatus variants
#[derive(Debug, Clone, PartialEq)]
pub enum TaskStatus {
    Open,
    InProgress,
    Done,
    Cancelled,
}

/// TargetType variants
#[derive(Debug, Clone, PartialEq)]
pub enum TargetType {
    Event,
    Task,
    Project,
}

/// AccessLevel variants
#[derive(Debug, Clone, PartialEq)]
pub enum AccessLevel {
    None,
    Read,
    Edit,
    Full,
}

/// GranteeType variants
#[derive(Debug, Clone, PartialEq)]
pub enum GranteeType {
    Member,
    Group,
}

/// ResourceType variants
#[derive(Debug, Clone, PartialEq)]
pub enum ResourceType {
    Event,
    Task,
    House,
}

/// ProjectStatus variants
#[derive(Debug, Clone, PartialEq)]
pub enum ProjectStatus {
    Active,
    Archived,
}

/// DependencyNodeType variants
#[derive(Debug, Clone, PartialEq)]
pub enum DependencyNodeType {
    Task,
    Project,
}

/// RecurrenceFreq variants
#[derive(Debug, Clone, PartialEq)]
pub enum RecurrenceFreq {
    Hourly,
    Daily,
    Weekly,
    Monthly,
    Quarterly,
    Yearly,
}

/// MilestoneState variants
#[derive(Debug, Clone, PartialEq)]
pub enum MilestoneState {
    Done,
    Current,
    Future,
}

#[derive(Debug, Clone, PartialEq)]
pub struct House {
    pub house_id: HouseID,
    /// constraint: size in 1..=256
    pub name: String,
    pub description: Option<String>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl House {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.len() < 1usize || v.len() > 256usize {
                return Err(ValidationError { field: "name".to_string(), message: "length must be in 1..=256".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Member {
    pub member_id: MemberID,
    pub house_id: HouseID,
    pub linkkeys_domain: String,
    pub linkkeys_user_id: String,
    pub display_name: Option<String>,
    pub email: Option<String>,
    pub avatar_url: Option<String>,
    pub cached_public_key: Option<Vec<u8>>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
    pub last_seen_at: Option<Timestamp>,
    pub deactivated_at: Option<Timestamp>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TrustedDomain {
    pub trusted_domain_id: TrustedDomainID,
    pub house_id: HouseID,
    pub domain: String,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Role {
    pub role_id: RoleID,
    pub house_id: HouseID,
    /// constraint: size in 1..=128
    pub name: String,
    pub description: Option<String>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Role {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.len() < 1usize || v.len() > 128usize {
                return Err(ValidationError { field: "name".to_string(), message: "length must be in 1..=128".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct MemberRole {
    pub member_id: MemberID,
    pub role_id: RoleID,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MemberAudit {
    pub audit_id: MemberAuditID,
    pub house_id: HouseID,
    pub subject_member_id: MemberID,
    pub actor_member_id: Option<MemberID>,
    pub action: String,
    pub target_type: Option<String>,
    pub target_id: Option<String>,
    pub detail: Option<String>,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Skill {
    pub skill_id: SkillID,
    pub house_id: HouseID,
    /// constraint: size in 1..=256
    pub name: String,
    pub description: Option<String>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Skill {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.len() < 1usize || v.len() > 256usize {
                return Err(ValidationError { field: "name".to_string(), message: "length must be in 1..=256".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct MemberSkill {
    pub member_id: MemberID,
    pub skill_id: SkillID,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct GroupSkill {
    pub group_id: GroupID,
    pub skill_id: SkillID,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Group {
    pub group_id: GroupID,
    pub house_id: HouseID,
    /// constraint: size in 1..=256
    pub name: String,
    pub description: Option<String>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Group {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.len() < 1usize || v.len() > 256usize {
                return Err(ValidationError { field: "name".to_string(), message: "length must be in 1..=256".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct GroupMember {
    pub group_id: GroupID,
    pub member_id: MemberID,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Project {
    pub project_id: ProjectID,
    pub house_id: HouseID,
    /// constraint: size in 1..=256
    pub name: String,
    pub description: Option<String>,
    pub category: Option<String>,
    /// default: "active"
    pub status: Option<ProjectStatus>,
    /// default: "read"
    pub visibility: Option<AccessLevel>,
    pub created_by_member_id: Option<MemberID>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Project {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.name;
            if v.len() < 1usize || v.len() > 256usize {
                return Err(ValidationError { field: "name".to_string(), message: "length must be in 1..=256".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectTask {
    pub project_id: ProjectID,
    pub task_id: TaskID,
    pub position: i64,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectMember {
    pub project_id: ProjectID,
    pub member_id: MemberID,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectOwner {
    pub project_id: ProjectID,
    pub member_id: MemberID,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Milestone {
    pub milestone_id: MilestoneID,
    pub project_id: ProjectID,
    /// constraint: size in 1..=256
    pub label: String,
    pub when_label: String,
    pub state: MilestoneState,
    pub position: i64,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Milestone {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.label;
            if v.len() < 1usize || v.len() > 256usize {
                return Err(ValidationError { field: "label".to_string(), message: "length must be in 1..=256".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Event {
    pub event_id: EventID,
    pub house_id: HouseID,
    pub owner_member_id: MemberID,
    /// constraint: size in 1..=512
    pub title: String,
    pub description: Option<String>,
    pub location: Option<String>,
    pub starts_at: Option<Timestamp>,
    pub ends_at: Option<Timestamp>,
    /// default: false
    pub all_day: Option<bool>,
    pub recurrence_freq: Option<RecurrenceFreq>,
    /// default: 1
    pub recurrence_interval: Option<i64>,
    pub recurrence_by_weekday: Option<Vec<i64>>,
    pub recurrence_by_setpos: Option<i64>,
    pub next_recurrence_at: Option<Timestamp>,
    pub recurrence_root_event_id: Option<EventID>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Event {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.title;
            if v.len() < 1usize || v.len() > 512usize {
                return Err(ValidationError { field: "title".to_string(), message: "length must be in 1..=512".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Task {
    pub task_id: TaskID,
    pub house_id: HouseID,
    pub owner_member_id: MemberID,
    pub assignees: Option<Vec<MemberID>>,
    pub assigned_to_skill_id: Option<SkillID>,
    pub parent_task_id: Option<TaskID>,
    /// default: "read"
    pub visibility: Option<AccessLevel>,
    /// constraint: size in 1..=512
    pub title: String,
    pub description: Option<String>,
    /// default: "open"
    pub status: Option<TaskStatus>,
    pub due_at: Option<Timestamp>,
    pub tag: Option<String>,
    pub estimate_minutes: Option<u64>,
    pub recurrence_freq: Option<RecurrenceFreq>,
    /// default: 1
    pub recurrence_interval: Option<i64>,
    pub recurrence_by_weekday: Option<Vec<i64>>,
    pub recurrence_by_setpos: Option<i64>,
    pub next_recurrence_at: Option<Timestamp>,
    pub recurrence_root_task_id: Option<TaskID>,
    pub deleted_at: Option<Timestamp>,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Task {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.title;
            if v.len() < 1usize || v.len() > 512usize {
                return Err(ValidationError { field: "title".to_string(), message: "length must be in 1..=512".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Comment {
    pub comment_id: CommentID,
    pub house_id: HouseID,
    pub member_id: MemberID,
    pub target_type: TargetType,
    pub target_id: String,
    /// constraint: size in 1..=10000
    pub body: String,
    pub created_at: Timestamp,
    pub updated_at: Timestamp,
}

impl Comment {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.body;
            if v.len() < 1usize || v.len() > 10000usize {
                return Err(ValidationError { field: "body".to_string(), message: "length must be in 1..=10000".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Share {
    pub share_id: ShareID,
    pub house_id: HouseID,
    pub shared_by: MemberID,
    pub linkkeys_domain: String,
    pub linkkeys_user_id: String,
    pub resource_type: ResourceType,
    pub resource_id: String,
    /// default: "read"
    pub access_level: Option<AccessLevel>,
    pub created_at: Timestamp,
    pub expires_at: Option<Timestamp>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HouseSummary {
    pub house_id: HouseID,
    pub name: String,
    pub member_id: MemberID,
    pub roles: Vec<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HouseRoles {
    pub house: HouseID,
    pub member: MemberID,
    pub roles: Vec<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Identity {
    pub domain: String,
    pub user_id: String,
    pub display_name: Option<String>,
    pub houses: Vec<HouseRoles>,
    pub iat: i64,
    pub exp: i64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct LoginRequest {
    pub signed_assertion: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CompleteRequest {
    pub encrypted_token: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct LoginResponse {
    pub token: String,
    pub domain: String,
    pub user_id: String,
    pub display_name: Option<String>,
    pub expires_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DevUserEntry {
    pub member_id: MemberID,
    pub house_id: HouseID,
    pub house_name: String,
    pub display_name: Option<String>,
    pub linkkeys_domain: Option<String>,
    pub linkkeys_user_id: Option<String>,
    pub roles: Vec<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DevUsersResponse {
    pub users: Vec<DevUserEntry>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DevLoginRequest {
    pub member_id: MemberID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MeResponse {
    pub domain: String,
    pub user_id: String,
    pub display_name: Option<String>,
    pub expires_at: Timestamp,
    pub houses: Vec<HouseSummary>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct EmptyRequest {
}

#[derive(Debug, Clone, PartialEq)]
pub struct EmptyResponse {
}

#[derive(Debug, Clone, PartialEq)]
pub struct BoolResponse {
    pub value: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HouseListRequest {
    /// default: 50
    pub limit: Option<u64>,
    /// default: 0
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct HouseScopedListRequest {
    pub house_id: HouseID,
    /// default: 50
    pub limit: Option<u64>,
    /// default: 0
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TaskList {
    pub tasks: Vec<Task>,
    pub hidden_count: u64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectList {
    pub projects: Vec<Project>,
    pub hidden_count: u64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MemberScopedListRequest {
    pub house_id: HouseID,
    pub member_id: MemberID,
    /// default: 50
    pub limit: Option<u64>,
    /// default: 0
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectScopedListRequest {
    pub house_id: HouseID,
    pub project_id: ProjectID,
    /// default: 50
    pub limit: Option<u64>,
    /// default: 0
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CommentListRequest {
    pub target_type: TargetType,
    pub target_id: String,
    /// default: 50
    pub limit: Option<u64>,
    /// default: 0
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Notification {
    pub notification_id: NotificationID,
    pub house_id: HouseID,
    pub member_id: MemberID,
    pub kind: String,
    pub actor_member_id: Option<MemberID>,
    pub actor_name: String,
    pub target_type: Option<String>,
    pub target_id: Option<String>,
    pub target_title: String,
    pub body: String,
    pub read: bool,
    pub read_at: Option<Timestamp>,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct NotificationListRequest {
    pub house_id: HouseID,
    /// default: false
    pub unread_only: Option<bool>,
    /// default: 50
    pub limit: Option<u64>,
    /// default: 0
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct NotificationUnreadCount {
    pub count: u64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ShareAccessRequest {
    pub linkkeys_domain: String,
    pub linkkeys_user_id: String,
    pub resource_type: ResourceType,
    pub resource_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ResourceRef {
    pub resource_type: ResourceType,
    pub resource_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MemberRoleRef {
    pub member_id: MemberID,
    pub role_id: RoleID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct MemberSkillRef {
    pub member_id: MemberID,
    pub skill_id: SkillID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct GroupSkillRef {
    pub group_id: GroupID,
    pub skill_id: SkillID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct GroupMemberRef {
    pub group_id: GroupID,
    pub member_id: MemberID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectTaskRef {
    pub project_id: ProjectID,
    pub task_id: TaskID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectTaskOrderRequest {
    pub project_id: ProjectID,
    pub task_id: TaskID,
    pub position: i64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectMemberRef {
    pub project_id: ProjectID,
    pub member_id: MemberID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectOwnerRef {
    pub project_id: ProjectID,
    pub member_id: MemberID,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DependencyRef {
    pub dependent_type: DependencyNodeType,
    pub dependent_id: String,
    pub dependency_type: DependencyNodeType,
    pub dependency_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DependencyTarget {
    pub r#type: DependencyNodeType,
    pub id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DependencyNode {
    pub r#type: DependencyNodeType,
    pub id: String,
    pub title: String,
    pub status: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DependencyGraph {
    pub dependencies: Vec<DependencyNode>,
    pub dependents: Vec<DependencyNode>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Grant {
    pub grantee_type: GranteeType,
    pub grantee_id: String,
    pub access_level: AccessLevel,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TaskGrantRef {
    pub task_id: TaskID,
    pub grantee_type: GranteeType,
    pub grantee_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PutTaskGrantRequest {
    pub task_id: TaskID,
    pub grantee_type: GranteeType,
    pub grantee_id: String,
    pub access_level: AccessLevel,
}

#[derive(Debug, Clone, PartialEq)]
pub struct SetTaskVisibilityRequest {
    pub task_id: TaskID,
    pub visibility: AccessLevel,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ProjectGrantRef {
    pub project_id: ProjectID,
    pub grantee_type: GranteeType,
    pub grantee_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PutProjectGrantRequest {
    pub project_id: ProjectID,
    pub grantee_type: GranteeType,
    pub grantee_id: String,
    pub access_level: AccessLevel,
}

#[derive(Debug, Clone, PartialEq)]
pub struct SetProjectVisibilityRequest {
    pub project_id: ProjectID,
    pub visibility: AccessLevel,
}

#[derive(Debug, Clone, PartialEq)]
pub struct EffectiveSettings {
    /// default: false
    pub bug_reports_enabled: Option<bool>,
    pub bug_reports_project_id: Option<ProjectID>,
    /// default: "read"
    pub default_project_visibility: Option<AccessLevel>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct UpdateSettingsRequest {
    pub house_id: HouseID,
    pub settings: EffectiveSettings,
}

#[derive(Debug, Clone, PartialEq)]
pub struct BugReportRequest {
    pub house_id: HouseID,
    /// constraint: size in 1..=512
    pub title: String,
    pub description: Option<String>,
}

impl BugReportRequest {
    /// Validate this value against the constraints declared in the CSIL spec.
    pub fn validate(&self) -> Result<(), ValidationError> {
        {
            let v = &self.title;
            if v.len() < 1usize || v.len() > 512usize {
                return Err(ValidationError { field: "title".to_string(), message: "length must be in 1..=512".to_string() });
            }
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct ServiceError {
    pub code: u64,
    pub message: String,
}

pub type AuditID = String;

#[derive(Debug, Clone, PartialEq)]
pub struct AuditEntry {
    pub audit_id: AuditID,
    pub house_id: Option<HouseID>,
    pub actor_member_id: Option<MemberID>,
    pub actor_domain: String,
    pub actor_user_id: String,
    pub service_name: String,
    pub method: String,
    pub action: String,
    pub resource_type: Option<String>,
    pub resource_id: Option<String>,
    pub outcome: String,
    pub before: Option<String>,
    pub after: Option<String>,
    pub detail: Option<String>,
    pub created_at: Timestamp,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuditQuery {
    pub house_id: HouseID,
    pub actor_member_id: Option<MemberID>,
    pub resource_type: Option<String>,
    pub action: Option<String>,
    pub since: Option<Timestamp>,
    pub until: Option<Timestamp>,
    pub cursor: Option<String>,
    /// default: 100
    pub limit: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct AuditPage {
    pub entries: Vec<AuditEntry>,
    pub next_cursor: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TrashItem {
    pub resource_type: String,
    pub resource_id: String,
    pub house_id: HouseID,
    pub title: Option<String>,
    pub deleted_at: Timestamp,
    pub deleted_by_member_id: Option<MemberID>,
    pub deleted_op_id: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TrashPage {
    pub items: Vec<TrashItem>,
    pub next_cursor: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct RestoreRequest {
    pub house_id: HouseID,
    pub deleted_op_id: Option<String>,
    pub resource_type: Option<String>,
    pub resource_id: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PurgeRequest {
    pub house_id: HouseID,
    pub resource_type: String,
    pub resource_id: String,
}

