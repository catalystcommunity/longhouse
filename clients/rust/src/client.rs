//! Generated transport-agnostic service clients from CSIL specification

use super::types::*;
use super::codec::*;

/// Error from a generated client call: a structured error the service returned,
/// or a transport-level failure. The caller-supplied `Transport` decides how an
/// error response maps onto `Service`.
#[derive(Debug, Clone)]
pub enum ClientError {
    Service { code: i64, message: String },
    Transport(String),
}

impl std::fmt::Display for ClientError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ClientError::Service { code, message } => write!(f, "service error {code}: {message}"),
            ClientError::Transport(msg) => write!(f, "transport error: {msg}"),
        }
    }
}

impl std::error::Error for ClientError {}

/// The caller-supplied byte carrier: it performs the call named by `(service, op)`
/// with the already-encoded request bytes and returns the response bytes, or an
/// error. The generated client owns (de)serialization via the codec; the carrier
/// only moves bytes, so it can be HTTP, a queue, or an in-process loop.
pub trait Transport {
    fn call(&self, service: &str, op: &str, req: &[u8]) -> Result<Vec<u8>, ClientError>;
}

/// Typed client for the AuthService service.
pub struct AuthClient<T: Transport> {
    transport: T,
}

impl<T: Transport> AuthClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// login (request/response).
    pub fn login(&self, req: LoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call("auth", "Login", &encode_login_request(&req))?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// complete (request/response).
    pub fn complete(&self, req: CompleteRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call("auth", "Complete", &encode_complete_request(&req))?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// refresh (request/response).
    pub fn refresh(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call("auth", "Refresh", &encode_empty_request(&req))?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// logout (request/response).
    pub fn logout(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("auth", "Logout", &encode_empty_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// me (request/response).
    pub fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_resp = self.transport.call("auth", "Me", &encode_empty_request(&req))?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DevAuthService service.
pub struct DevAuthClient<T: Transport> {
    transport: T,
}

impl<T: Transport> DevAuthClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-dev-users (request/response).
    pub fn list_dev_users(&self, req: EmptyRequest) -> Result<DevUsersResponse, ClientError> {
        let csil_resp = self.transport.call("devauth", "ListDevUsers", &encode_empty_request(&req))?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// dev-login (request/response).
    pub fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self.transport.call("devauth", "DevLogin", &encode_dev_login_request(&req))?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the HouseService service.
pub struct HouseClient<T: Transport> {
    transport: T,
}

impl<T: Transport> HouseClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-house (request/response).
    pub fn create_house(&self, req: House) -> Result<House, ClientError> {
        let csil_resp = self.transport.call("house", "CreateHouse", &encode_house(&req))?;
        decode_house(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-house (request/response).
    pub fn get_house(&self, req: HouseID) -> Result<House, ClientError> {
        let csil_resp = self.transport.call("house", "GetHouse", &encode_house_get_house_request(&req))?;
        decode_house(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-house (request/response).
    pub fn update_house(&self, req: House) -> Result<House, ClientError> {
        let csil_resp = self.transport.call("house", "UpdateHouse", &encode_house(&req))?;
        decode_house(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-house (request/response).
    pub fn delete_house(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("house", "DeleteHouse", &encode_house_delete_house_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-houses (request/response).
    pub fn list_houses(&self, req: HouseListRequest) -> Result<Vec<House>, ClientError> {
        let csil_resp = self.transport.call("house", "ListHouses", &encode_house_list_request(&req))?;
        decode_house_list_houses_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the MemberService service.
pub struct MemberClient<T: Transport> {
    transport: T,
}

impl<T: Transport> MemberClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-member (request/response).
    pub fn create_member(&self, req: Member) -> Result<Member, ClientError> {
        let csil_resp = self.transport.call("member", "CreateMember", &encode_member(&req))?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-member (request/response).
    pub fn get_member(&self, req: MemberID) -> Result<Member, ClientError> {
        let csil_resp = self.transport.call("member", "GetMember", &encode_member_get_member_request(&req))?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-member-by-identity (request/response).
    pub fn get_member_by_identity(&self, req: Member) -> Result<Member, ClientError> {
        let csil_resp = self.transport.call("member", "GetMemberByIdentity", &encode_member(&req))?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-member (request/response).
    pub fn update_member(&self, req: Member) -> Result<Member, ClientError> {
        let csil_resp = self.transport.call("member", "UpdateMember", &encode_member(&req))?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// deactivate-member (request/response).
    pub fn deactivate_member(&self, req: MemberID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("member", "DeactivateMember", &encode_member_deactivate_member_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// reactivate-member (request/response).
    pub fn reactivate_member(&self, req: MemberID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("member", "ReactivateMember", &encode_member_reactivate_member_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-members (request/response).
    pub fn list_members(&self, req: HouseScopedListRequest) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self.transport.call("member", "ListMembers", &encode_house_scoped_list_request(&req))?;
        decode_member_list_members_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the TrustedDomainService service.
pub struct TrustedDomainClient<T: Transport> {
    transport: T,
}

impl<T: Transport> TrustedDomainClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// add-trusted-domain (request/response).
    pub fn add_trusted_domain(&self, req: TrustedDomain) -> Result<TrustedDomain, ClientError> {
        let csil_resp = self.transport.call("trusteddomain", "AddTrustedDomain", &encode_trusted_domain(&req))?;
        decode_trusted_domain(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-trusted-domain (request/response).
    pub fn remove_trusted_domain(&self, req: TrustedDomainID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("trusteddomain", "RemoveTrustedDomain", &encode_trusted_domain_remove_trusted_domain_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-trusted-domains (request/response).
    pub fn list_trusted_domains(&self, req: HouseID) -> Result<Vec<TrustedDomain>, ClientError> {
        let csil_resp = self.transport.call("trusteddomain", "ListTrustedDomains", &encode_trusted_domain_list_trusted_domains_request(&req))?;
        decode_trusted_domain_list_trusted_domains_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// is-domain-trusted (request/response).
    pub fn is_domain_trusted(&self, req: TrustedDomain) -> Result<BoolResponse, ClientError> {
        let csil_resp = self.transport.call("trusteddomain", "IsDomainTrusted", &encode_trusted_domain(&req))?;
        decode_bool_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the RoleService service.
pub struct RoleClient<T: Transport> {
    transport: T,
}

impl<T: Transport> RoleClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-role (request/response).
    pub fn create_role(&self, req: Role) -> Result<Role, ClientError> {
        let csil_resp = self.transport.call("role", "CreateRole", &encode_role(&req))?;
        decode_role(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-role (request/response).
    pub fn update_role(&self, req: Role) -> Result<Role, ClientError> {
        let csil_resp = self.transport.call("role", "UpdateRole", &encode_role(&req))?;
        decode_role(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-role (request/response).
    pub fn delete_role(&self, req: RoleID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("role", "DeleteRole", &encode_role_delete_role_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-roles (request/response).
    pub fn list_roles(&self, req: HouseScopedListRequest) -> Result<Vec<Role>, ClientError> {
        let csil_resp = self.transport.call("role", "ListRoles", &encode_house_scoped_list_request(&req))?;
        decode_role_list_roles_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// grant-role (request/response).
    pub fn grant_role(&self, req: MemberRoleRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("role", "GrantRole", &encode_member_role_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// revoke-role (request/response).
    pub fn revoke_role(&self, req: MemberRoleRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("role", "RevokeRole", &encode_member_role_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-member-roles (request/response).
    pub fn list_member_roles(&self, req: MemberScopedListRequest) -> Result<Vec<Role>, ClientError> {
        let csil_resp = self.transport.call("role", "ListMemberRoles", &encode_member_scoped_list_request(&req))?;
        decode_role_list_member_roles_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the SkillService service.
pub struct SkillClient<T: Transport> {
    transport: T,
}

impl<T: Transport> SkillClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-skill (request/response).
    pub fn create_skill(&self, req: Skill) -> Result<Skill, ClientError> {
        let csil_resp = self.transport.call("skill", "CreateSkill", &encode_skill(&req))?;
        decode_skill(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-skill (request/response).
    pub fn update_skill(&self, req: Skill) -> Result<Skill, ClientError> {
        let csil_resp = self.transport.call("skill", "UpdateSkill", &encode_skill(&req))?;
        decode_skill(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-skill (request/response).
    pub fn delete_skill(&self, req: SkillID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("skill", "DeleteSkill", &encode_skill_delete_skill_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-skills (request/response).
    pub fn list_skills(&self, req: HouseScopedListRequest) -> Result<Vec<Skill>, ClientError> {
        let csil_resp = self.transport.call("skill", "ListSkills", &encode_house_scoped_list_request(&req))?;
        decode_skill_list_skills_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-member-skill (request/response).
    pub fn add_member_skill(&self, req: MemberSkillRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("skill", "AddMemberSkill", &encode_member_skill_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-member-skill (request/response).
    pub fn remove_member_skill(&self, req: MemberSkillRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("skill", "RemoveMemberSkill", &encode_member_skill_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-member-skills (request/response).
    pub fn list_member_skills(&self, req: MemberScopedListRequest) -> Result<Vec<Skill>, ClientError> {
        let csil_resp = self.transport.call("skill", "ListMemberSkills", &encode_member_scoped_list_request(&req))?;
        decode_skill_list_member_skills_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-group-skill (request/response).
    pub fn add_group_skill(&self, req: GroupSkillRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("skill", "AddGroupSkill", &encode_group_skill_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-group-skill (request/response).
    pub fn remove_group_skill(&self, req: GroupSkillRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("skill", "RemoveGroupSkill", &encode_group_skill_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-group-skills (request/response).
    pub fn list_group_skills(&self, req: GroupID) -> Result<Vec<Skill>, ClientError> {
        let csil_resp = self.transport.call("skill", "ListGroupSkills", &encode_skill_list_group_skills_request(&req))?;
        decode_skill_list_group_skills_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the GroupService service.
pub struct GroupClient<T: Transport> {
    transport: T,
}

impl<T: Transport> GroupClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-group (request/response).
    pub fn create_group(&self, req: Group) -> Result<Group, ClientError> {
        let csil_resp = self.transport.call("group", "CreateGroup", &encode_group(&req))?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-group (request/response).
    pub fn update_group(&self, req: Group) -> Result<Group, ClientError> {
        let csil_resp = self.transport.call("group", "UpdateGroup", &encode_group(&req))?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-group (request/response).
    pub fn delete_group(&self, req: GroupID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("group", "DeleteGroup", &encode_group_delete_group_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-groups (request/response).
    pub fn list_groups(&self, req: HouseScopedListRequest) -> Result<Vec<Group>, ClientError> {
        let csil_resp = self.transport.call("group", "ListGroups", &encode_house_scoped_list_request(&req))?;
        decode_group_list_groups_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-group-member (request/response).
    pub fn add_group_member(&self, req: GroupMemberRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("group", "AddGroupMember", &encode_group_member_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-group-member (request/response).
    pub fn remove_group_member(&self, req: GroupMemberRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("group", "RemoveGroupMember", &encode_group_member_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-group-members (request/response).
    pub fn list_group_members(&self, req: MemberScopedListRequest) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self.transport.call("group", "ListGroupMembers", &encode_member_scoped_list_request(&req))?;
        decode_group_list_group_members_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the ProjectService service.
pub struct ProjectClient<T: Transport> {
    transport: T,
}

impl<T: Transport> ProjectClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-project (request/response).
    pub fn create_project(&self, req: Project) -> Result<Project, ClientError> {
        let csil_resp = self.transport.call("project", "CreateProject", &encode_project(&req))?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-project (request/response).
    pub fn get_project(&self, req: ProjectID) -> Result<Project, ClientError> {
        let csil_resp = self.transport.call("project", "GetProject", &encode_project_get_project_request(&req))?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-project (request/response).
    pub fn update_project(&self, req: Project) -> Result<Project, ClientError> {
        let csil_resp = self.transport.call("project", "UpdateProject", &encode_project(&req))?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-project (request/response).
    pub fn delete_project(&self, req: ProjectID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "DeleteProject", &encode_project_delete_project_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-projects (request/response).
    pub fn list_projects(&self, req: HouseScopedListRequest) -> Result<ProjectList, ClientError> {
        let csil_resp = self.transport.call("project", "ListProjects", &encode_house_scoped_list_request(&req))?;
        decode_project_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-tasks (request/response).
    pub fn list_project_tasks(&self, req: ProjectScopedListRequest) -> Result<TaskList, ClientError> {
        let csil_resp = self.transport.call("project", "ListProjectTasks", &encode_project_scoped_list_request(&req))?;
        decode_task_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-project-task (request/response).
    pub fn add_project_task(&self, req: ProjectTaskOrderRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "AddProjectTask", &encode_project_task_order_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-project-task (request/response).
    pub fn remove_project_task(&self, req: ProjectTaskRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "RemoveProjectTask", &encode_project_task_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-project-task-position (request/response).
    pub fn set_project_task_position(&self, req: ProjectTaskOrderRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "SetProjectTaskPosition", &encode_project_task_order_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-members (request/response).
    pub fn list_project_members(&self, req: ProjectID) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self.transport.call("project", "ListProjectMembers", &encode_project_list_project_members_request(&req))?;
        decode_project_list_project_members_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-project-member (request/response).
    pub fn add_project_member(&self, req: ProjectMemberRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "AddProjectMember", &encode_project_member_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-project-member (request/response).
    pub fn remove_project_member(&self, req: ProjectMemberRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "RemoveProjectMember", &encode_project_member_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-owners (request/response).
    pub fn list_project_owners(&self, req: ProjectID) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self.transport.call("project", "ListProjectOwners", &encode_project_list_project_owners_request(&req))?;
        decode_project_list_project_owners_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-project-owner (request/response).
    pub fn add_project_owner(&self, req: ProjectOwnerRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "AddProjectOwner", &encode_project_owner_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-project-owner (request/response).
    pub fn remove_project_owner(&self, req: ProjectOwnerRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "RemoveProjectOwner", &encode_project_owner_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-milestones (request/response).
    pub fn list_milestones(&self, req: ProjectID) -> Result<Vec<Milestone>, ClientError> {
        let csil_resp = self.transport.call("project", "ListMilestones", &encode_project_list_milestones_request(&req))?;
        decode_project_list_milestones_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-milestone (request/response).
    pub fn create_milestone(&self, req: Milestone) -> Result<Milestone, ClientError> {
        let csil_resp = self.transport.call("project", "CreateMilestone", &encode_milestone(&req))?;
        decode_milestone(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-milestone (request/response).
    pub fn update_milestone(&self, req: Milestone) -> Result<Milestone, ClientError> {
        let csil_resp = self.transport.call("project", "UpdateMilestone", &encode_milestone(&req))?;
        decode_milestone(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-milestone (request/response).
    pub fn delete_milestone(&self, req: MilestoneID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "DeleteMilestone", &encode_project_delete_milestone_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-project-visibility (request/response).
    pub fn set_project_visibility(&self, req: SetProjectVisibilityRequest) -> Result<Project, ClientError> {
        let csil_resp = self.transport.call("project", "SetProjectVisibility", &encode_set_project_visibility_request(&req))?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-grants (request/response).
    pub fn list_project_grants(&self, req: ProjectID) -> Result<Vec<Grant>, ClientError> {
        let csil_resp = self.transport.call("project", "ListProjectGrants", &encode_project_list_project_grants_request(&req))?;
        decode_project_list_project_grants_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// put-project-grant (request/response).
    pub fn put_project_grant(&self, req: PutProjectGrantRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "PutProjectGrant", &encode_put_project_grant_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-project-grant (request/response).
    pub fn delete_project_grant(&self, req: ProjectGrantRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("project", "DeleteProjectGrant", &encode_project_grant_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the EventService service.
pub struct EventClient<T: Transport> {
    transport: T,
}

impl<T: Transport> EventClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-event (request/response).
    pub fn create_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call("event", "CreateEvent", &encode_event(&req))?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-event (request/response).
    pub fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call("event", "GetEvent", &encode_event_get_event_request(&req))?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-event (request/response).
    pub fn update_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_resp = self.transport.call("event", "UpdateEvent", &encode_event(&req))?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-event (request/response).
    pub fn delete_event(&self, req: EventID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("event", "DeleteEvent", &encode_event_delete_event_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-event-and-future (request/response).
    pub fn delete_event_and_future(&self, req: EventID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("event", "DeleteEventAndFuture", &encode_event_delete_event_and_future_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-events (request/response).
    pub fn list_events(&self, req: HouseScopedListRequest) -> Result<Vec<Event>, ClientError> {
        let csil_resp = self.transport.call("event", "ListEvents", &encode_house_scoped_list_request(&req))?;
        decode_event_list_events_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the TaskService service.
pub struct TaskClient<T: Transport> {
    transport: T,
}

impl<T: Transport> TaskClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-task (request/response).
    pub fn create_task(&self, req: Task) -> Result<Task, ClientError> {
        let csil_resp = self.transport.call("task", "CreateTask", &encode_task(&req))?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-task (request/response).
    pub fn get_task(&self, req: TaskID) -> Result<Task, ClientError> {
        let csil_resp = self.transport.call("task", "GetTask", &encode_task_get_task_request(&req))?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-task (request/response).
    pub fn update_task(&self, req: Task) -> Result<Task, ClientError> {
        let csil_resp = self.transport.call("task", "UpdateTask", &encode_task(&req))?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-task (request/response).
    pub fn delete_task(&self, req: TaskID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("task", "DeleteTask", &encode_task_delete_task_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-tasks (request/response).
    pub fn list_tasks(&self, req: HouseScopedListRequest) -> Result<TaskList, ClientError> {
        let csil_resp = self.transport.call("task", "ListTasks", &encode_house_scoped_list_request(&req))?;
        decode_task_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-task-visibility (request/response).
    pub fn set_task_visibility(&self, req: SetTaskVisibilityRequest) -> Result<Task, ClientError> {
        let csil_resp = self.transport.call("task", "SetTaskVisibility", &encode_set_task_visibility_request(&req))?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-task-grants (request/response).
    pub fn list_task_grants(&self, req: TaskID) -> Result<Vec<Grant>, ClientError> {
        let csil_resp = self.transport.call("task", "ListTaskGrants", &encode_task_list_task_grants_request(&req))?;
        decode_task_list_task_grants_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// put-task-grant (request/response).
    pub fn put_task_grant(&self, req: PutTaskGrantRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("task", "PutTaskGrant", &encode_put_task_grant_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-task-grant (request/response).
    pub fn delete_task_grant(&self, req: TaskGrantRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("task", "DeleteTaskGrant", &encode_task_grant_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DependencyService service.
pub struct DependencyClient<T: Transport> {
    transport: T,
}

impl<T: Transport> DependencyClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// add-dependency (request/response).
    pub fn add_dependency(&self, req: DependencyRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("dependency", "AddDependency", &encode_dependency_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-dependency (request/response).
    pub fn remove_dependency(&self, req: DependencyRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("dependency", "RemoveDependency", &encode_dependency_ref(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-dependencies (request/response).
    pub fn get_dependencies(&self, req: DependencyTarget) -> Result<DependencyGraph, ClientError> {
        let csil_resp = self.transport.call("dependency", "GetDependencies", &encode_dependency_target(&req))?;
        decode_dependency_graph(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the CommentService service.
pub struct CommentClient<T: Transport> {
    transport: T,
}

impl<T: Transport> CommentClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-comment (request/response).
    pub fn create_comment(&self, req: Comment) -> Result<Comment, ClientError> {
        let csil_resp = self.transport.call("comment", "CreateComment", &encode_comment(&req))?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-comment (request/response).
    pub fn get_comment(&self, req: CommentID) -> Result<Comment, ClientError> {
        let csil_resp = self.transport.call("comment", "GetComment", &encode_comment_get_comment_request(&req))?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-comment (request/response).
    pub fn update_comment(&self, req: Comment) -> Result<Comment, ClientError> {
        let csil_resp = self.transport.call("comment", "UpdateComment", &encode_comment(&req))?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-comment (request/response).
    pub fn delete_comment(&self, req: CommentID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("comment", "DeleteComment", &encode_comment_delete_comment_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-comments (request/response).
    pub fn list_comments(&self, req: CommentListRequest) -> Result<Vec<Comment>, ClientError> {
        let csil_resp = self.transport.call("comment", "ListComments", &encode_comment_list_request(&req))?;
        decode_comment_list_comments_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the NotificationService service.
pub struct NotificationClient<T: Transport> {
    transport: T,
}

impl<T: Transport> NotificationClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-notifications (request/response).
    pub fn list_notifications(&self, req: NotificationListRequest) -> Result<Vec<Notification>, ClientError> {
        let csil_resp = self.transport.call("notification", "ListNotifications", &encode_notification_list_request(&req))?;
        decode_notification_list_notifications_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// unread-count (request/response).
    pub fn unread_count(&self, req: HouseID) -> Result<NotificationUnreadCount, ClientError> {
        let csil_resp = self.transport.call("notification", "UnreadCount", &encode_notification_unread_count_request(&req))?;
        decode_notification_unread_count(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// mark-read (request/response).
    pub fn mark_read(&self, req: NotificationID) -> Result<Notification, ClientError> {
        let csil_resp = self.transport.call("notification", "MarkRead", &encode_notification_mark_read_request(&req))?;
        decode_notification(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// mark-all-read (request/response).
    pub fn mark_all_read(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("notification", "MarkAllRead", &encode_notification_mark_all_read_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the ShareService service.
pub struct ShareClient<T: Transport> {
    transport: T,
}

impl<T: Transport> ShareClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-share (request/response).
    pub fn create_share(&self, req: Share) -> Result<Share, ClientError> {
        let csil_resp = self.transport.call("share", "CreateShare", &encode_share(&req))?;
        decode_share(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-share (request/response).
    pub fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("share", "DeleteShare", &encode_share_delete_share_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-shares-by-resource (request/response).
    pub fn list_shares_by_resource(&self, req: ResourceRef) -> Result<Vec<Share>, ClientError> {
        let csil_resp = self.transport.call("share", "ListSharesByResource", &encode_resource_ref(&req))?;
        decode_share_list_shares_by_resource_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// check-access (request/response).
    pub fn check_access(&self, req: ShareAccessRequest) -> Result<Share, ClientError> {
        let csil_resp = self.transport.call("share", "CheckAccess", &encode_share_access_request(&req))?;
        decode_share(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the MemberAuditService service.
pub struct MemberAuditClient<T: Transport> {
    transport: T,
}

impl<T: Transport> MemberAuditClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-audits-for-member (request/response).
    pub fn list_audits_for_member(&self, req: MemberScopedListRequest) -> Result<Vec<MemberAudit>, ClientError> {
        let csil_resp = self.transport.call("memberaudit", "ListAuditsForMember", &encode_member_scoped_list_request(&req))?;
        decode_member_audit_list_audits_for_member_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the SettingsService service.
pub struct SettingsClient<T: Transport> {
    transport: T,
}

impl<T: Transport> SettingsClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// get-settings (request/response).
    pub fn get_settings(&self, req: HouseID) -> Result<EffectiveSettings, ClientError> {
        let csil_resp = self.transport.call("settings", "GetSettings", &encode_settings_get_settings_request(&req))?;
        decode_effective_settings(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-settings (request/response).
    pub fn update_settings(&self, req: UpdateSettingsRequest) -> Result<EffectiveSettings, ClientError> {
        let csil_resp = self.transport.call("settings", "UpdateSettings", &encode_update_settings_request(&req))?;
        decode_effective_settings(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the BugService service.
pub struct BugClient<T: Transport> {
    transport: T,
}

impl<T: Transport> BugClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// report-bug (request/response).
    pub fn report_bug(&self, req: BugReportRequest) -> Result<Task, ClientError> {
        let csil_resp = self.transport.call("bug", "ReportBug", &encode_bug_report_request(&req))?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the AuditService service.
pub struct AuditClient<T: Transport> {
    transport: T,
}

impl<T: Transport> AuditClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// query-audit (request/response).
    pub fn query_audit(&self, req: AuditQuery) -> Result<AuditPage, ClientError> {
        let csil_resp = self.transport.call("audit", "QueryAudit", &encode_audit_query(&req))?;
        decode_audit_page(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the TrashService service.
pub struct TrashClient<T: Transport> {
    transport: T,
}

impl<T: Transport> TrashClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-trash (request/response).
    pub fn list_trash(&self, req: HouseScopedListRequest) -> Result<TrashPage, ClientError> {
        let csil_resp = self.transport.call("trash", "ListTrash", &encode_house_scoped_list_request(&req))?;
        decode_trash_page(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// restore (request/response).
    pub fn restore(&self, req: RestoreRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("trash", "Restore", &encode_restore_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// purge (request/response).
    pub fn purge(&self, req: PurgeRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self.transport.call("trash", "Purge", &encode_purge_request(&req))?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

