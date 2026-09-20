//! Generated transport-agnostic service clients from CSIL specification

#![allow(async_fn_in_trait)]

use super::client::ClientError;
use super::codec::*;
use super::types::*;

/// The caller-supplied byte carrier: it performs the call named by `(service, op)`
/// with the already-encoded request bytes and returns the response bytes, or an
/// error. The generated client owns (de)serialization via the codec; the carrier
/// only moves bytes, so it can be HTTP, a queue, or an in-process loop.
pub trait AsyncTransport {
    async fn call(&self, service: &str, op: &str, req: &[u8]) -> Result<Vec<u8>, ClientError>;
}

/// Typed client for the AuthService service.
pub struct AuthAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> AuthAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// login (request/response).
    pub async fn login(&self, req: LoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "login", &encode_login_request(&req))
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// complete (request/response).
    pub async fn complete(&self, req: CompleteRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "complete", &encode_complete_request(&req))
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// refresh (request/response).
    pub async fn refresh(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "refresh", &encode_empty_request(&req))
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// logout (request/response).
    pub async fn logout(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "logout", &encode_empty_request(&req))
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// me (request/response).
    pub async fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "me", &encode_empty_request(&req))
            .await?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// begin-cli-login (request/response).
    pub async fn begin_cli_login(
        &self,
        req: BeginCliLoginRequest,
    ) -> Result<BeginCliLoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "begin-cli-login",
                &encode_begin_cli_login_request(&req),
            )
            .await?;
        decode_begin_cli_login_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// inspect-cli-login (request/response).
    pub async fn inspect_cli_login(
        &self,
        req: ApproveCliLoginRequest,
    ) -> Result<CliLoginRequestInfo, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "inspect-cli-login",
                &encode_approve_cli_login_request(&req),
            )
            .await?;
        decode_cli_login_request_info(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// approve-cli-login (request/response).
    pub async fn approve_cli_login(
        &self,
        req: ApproveCliLoginRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "approve-cli-login",
                &encode_approve_cli_login_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// deny-cli-login (request/response).
    pub async fn deny_cli_login(
        &self,
        req: DenyCliLoginRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "deny-cli-login",
                &encode_deny_cli_login_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// exchange-cli-login (request/response).
    pub async fn exchange_cli_login(
        &self,
        req: ExchangeCliLoginRequest,
    ) -> Result<ExchangeCliLoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "exchange-cli-login",
                &encode_exchange_cli_login_request(&req),
            )
            .await?;
        decode_exchange_cli_login_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// refresh-session (request/response).
    pub async fn refresh_session(
        &self,
        req: RefreshSessionRequest,
    ) -> Result<CliTokenResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "refresh-session",
                &encode_refresh_session_request(&req),
            )
            .await?;
        decode_cli_token_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-sessions (request/response).
    pub async fn list_sessions(
        &self,
        req: EmptyRequest,
    ) -> Result<CliSessionsResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("AuthService", "list-sessions", &encode_empty_request(&req))
            .await?;
        decode_cli_sessions_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// revoke-session (request/response).
    pub async fn revoke_session(
        &self,
        req: RevokeSessionRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "AuthService",
                "revoke-session",
                &encode_revoke_session_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DevAuthService service.
pub struct DevAuthAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> DevAuthAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-dev-users (request/response).
    pub async fn list_dev_users(&self, req: EmptyRequest) -> Result<DevUsersResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DevAuthService",
                "list-dev-users",
                &encode_empty_request(&req),
            )
            .await?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// dev-login (request/response).
    pub async fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DevAuthService",
                "dev-login",
                &encode_dev_login_request(&req),
            )
            .await?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the HouseService service.
pub struct HouseAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> HouseAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-house (request/response).
    pub async fn create_house(&self, req: House) -> Result<House, ClientError> {
        let csil_resp = self
            .transport
            .call("HouseService", "create-house", &encode_house(&req))
            .await?;
        decode_house(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-house (request/response).
    pub async fn get_house(&self, req: HouseID) -> Result<House, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "HouseService",
                "get-house",
                &encode_house_get_house_request(&req),
            )
            .await?;
        decode_house(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-house (request/response).
    pub async fn update_house(&self, req: House) -> Result<House, ClientError> {
        let csil_resp = self
            .transport
            .call("HouseService", "update-house", &encode_house(&req))
            .await?;
        decode_house(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-house (request/response).
    pub async fn delete_house(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "HouseService",
                "delete-house",
                &encode_house_delete_house_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-houses (request/response).
    pub async fn list_houses(&self, req: HouseListRequest) -> Result<Vec<House>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "HouseService",
                "list-houses",
                &encode_house_list_request(&req),
            )
            .await?;
        decode_house_list_houses_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the MemberService service.
pub struct MemberAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> MemberAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-member (request/response).
    pub async fn create_member(&self, req: Member) -> Result<Member, ClientError> {
        let csil_resp = self
            .transport
            .call("MemberService", "create-member", &encode_member(&req))
            .await?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-member (request/response).
    pub async fn get_member(&self, req: MemberID) -> Result<Member, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "MemberService",
                "get-member",
                &encode_member_get_member_request(&req),
            )
            .await?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-member-by-identity (request/response).
    pub async fn get_member_by_identity(&self, req: Member) -> Result<Member, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "MemberService",
                "get-member-by-identity",
                &encode_member(&req),
            )
            .await?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-member (request/response).
    pub async fn update_member(&self, req: Member) -> Result<Member, ClientError> {
        let csil_resp = self
            .transport
            .call("MemberService", "update-member", &encode_member(&req))
            .await?;
        decode_member(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// deactivate-member (request/response).
    pub async fn deactivate_member(&self, req: MemberID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "MemberService",
                "deactivate-member",
                &encode_member_deactivate_member_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// reactivate-member (request/response).
    pub async fn reactivate_member(&self, req: MemberID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "MemberService",
                "reactivate-member",
                &encode_member_reactivate_member_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-members (request/response).
    pub async fn list_members(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "MemberService",
                "list-members",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_member_list_members_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the TrustedDomainService service.
pub struct TrustedDomainAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> TrustedDomainAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// add-trusted-domain (request/response).
    pub async fn add_trusted_domain(
        &self,
        req: TrustedDomain,
    ) -> Result<TrustedDomain, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TrustedDomainService",
                "add-trusted-domain",
                &encode_trusted_domain(&req),
            )
            .await?;
        decode_trusted_domain(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-trusted-domain (request/response).
    pub async fn remove_trusted_domain(
        &self,
        req: TrustedDomainID,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TrustedDomainService",
                "remove-trusted-domain",
                &encode_trusted_domain_remove_trusted_domain_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-trusted-domains (request/response).
    pub async fn list_trusted_domains(
        &self,
        req: HouseID,
    ) -> Result<Vec<TrustedDomain>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TrustedDomainService",
                "list-trusted-domains",
                &encode_trusted_domain_list_trusted_domains_request(&req),
            )
            .await?;
        decode_trusted_domain_list_trusted_domains_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// is-domain-trusted (request/response).
    pub async fn is_domain_trusted(&self, req: TrustedDomain) -> Result<BoolResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TrustedDomainService",
                "is-domain-trusted",
                &encode_trusted_domain(&req),
            )
            .await?;
        decode_bool_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the RoleService service.
pub struct RoleAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> RoleAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-role (request/response).
    pub async fn create_role(&self, req: Role) -> Result<Role, ClientError> {
        let csil_resp = self
            .transport
            .call("RoleService", "create-role", &encode_role(&req))
            .await?;
        decode_role(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-role (request/response).
    pub async fn update_role(&self, req: Role) -> Result<Role, ClientError> {
        let csil_resp = self
            .transport
            .call("RoleService", "update-role", &encode_role(&req))
            .await?;
        decode_role(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-role (request/response).
    pub async fn delete_role(&self, req: RoleID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "RoleService",
                "delete-role",
                &encode_role_delete_role_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-roles (request/response).
    pub async fn list_roles(&self, req: HouseScopedListRequest) -> Result<Vec<Role>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "RoleService",
                "list-roles",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_role_list_roles_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// grant-role (request/response).
    pub async fn grant_role(&self, req: MemberRoleRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("RoleService", "grant-role", &encode_member_role_ref(&req))
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// revoke-role (request/response).
    pub async fn revoke_role(&self, req: MemberRoleRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("RoleService", "revoke-role", &encode_member_role_ref(&req))
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-member-roles (request/response).
    pub async fn list_member_roles(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<Vec<Role>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "RoleService",
                "list-member-roles",
                &encode_member_scoped_list_request(&req),
            )
            .await?;
        decode_role_list_member_roles_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the SkillService service.
pub struct SkillAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> SkillAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-skill (request/response).
    pub async fn create_skill(&self, req: Skill) -> Result<Skill, ClientError> {
        let csil_resp = self
            .transport
            .call("SkillService", "create-skill", &encode_skill(&req))
            .await?;
        decode_skill(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-skill (request/response).
    pub async fn update_skill(&self, req: Skill) -> Result<Skill, ClientError> {
        let csil_resp = self
            .transport
            .call("SkillService", "update-skill", &encode_skill(&req))
            .await?;
        decode_skill(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-skill (request/response).
    pub async fn delete_skill(&self, req: SkillID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "delete-skill",
                &encode_skill_delete_skill_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-skills (request/response).
    pub async fn list_skills(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<Vec<Skill>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "list-skills",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_skill_list_skills_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-member-skill (request/response).
    pub async fn add_member_skill(
        &self,
        req: MemberSkillRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "add-member-skill",
                &encode_member_skill_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-member-skill (request/response).
    pub async fn remove_member_skill(
        &self,
        req: MemberSkillRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "remove-member-skill",
                &encode_member_skill_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-member-skills (request/response).
    pub async fn list_member_skills(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<Vec<Skill>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "list-member-skills",
                &encode_member_scoped_list_request(&req),
            )
            .await?;
        decode_skill_list_member_skills_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-group-skill (request/response).
    pub async fn add_group_skill(&self, req: GroupSkillRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "add-group-skill",
                &encode_group_skill_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-group-skill (request/response).
    pub async fn remove_group_skill(
        &self,
        req: GroupSkillRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "remove-group-skill",
                &encode_group_skill_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-group-skills (request/response).
    pub async fn list_group_skills(&self, req: GroupID) -> Result<Vec<Skill>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SkillService",
                "list-group-skills",
                &encode_skill_list_group_skills_request(&req),
            )
            .await?;
        decode_skill_list_group_skills_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the GroupService service.
pub struct GroupAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> GroupAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-group (request/response).
    pub async fn create_group(&self, req: Group) -> Result<Group, ClientError> {
        let csil_resp = self
            .transport
            .call("GroupService", "create-group", &encode_group(&req))
            .await?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-group (request/response).
    pub async fn update_group(&self, req: Group) -> Result<Group, ClientError> {
        let csil_resp = self
            .transport
            .call("GroupService", "update-group", &encode_group(&req))
            .await?;
        decode_group(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-group (request/response).
    pub async fn delete_group(&self, req: GroupID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "GroupService",
                "delete-group",
                &encode_group_delete_group_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-groups (request/response).
    pub async fn list_groups(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<Vec<Group>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "GroupService",
                "list-groups",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_group_list_groups_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-group-member (request/response).
    pub async fn add_group_member(
        &self,
        req: GroupMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "GroupService",
                "add-group-member",
                &encode_group_member_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-group-member (request/response).
    pub async fn remove_group_member(
        &self,
        req: GroupMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "GroupService",
                "remove-group-member",
                &encode_group_member_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-group-members (request/response).
    pub async fn list_group_members(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "GroupService",
                "list-group-members",
                &encode_member_scoped_list_request(&req),
            )
            .await?;
        decode_group_list_group_members_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the ProjectService service.
pub struct ProjectAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> ProjectAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-project (request/response).
    pub async fn create_project(&self, req: Project) -> Result<Project, ClientError> {
        let csil_resp = self
            .transport
            .call("ProjectService", "create-project", &encode_project(&req))
            .await?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-project (request/response).
    pub async fn get_project(&self, req: ProjectID) -> Result<Project, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "get-project",
                &encode_project_get_project_request(&req),
            )
            .await?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-project (request/response).
    pub async fn update_project(&self, req: Project) -> Result<Project, ClientError> {
        let csil_resp = self
            .transport
            .call("ProjectService", "update-project", &encode_project(&req))
            .await?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-project (request/response).
    pub async fn delete_project(&self, req: ProjectID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "delete-project",
                &encode_project_delete_project_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-projects (request/response).
    pub async fn list_projects(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<ProjectList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "list-projects",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_project_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-tasks (request/response).
    pub async fn list_project_tasks(
        &self,
        req: ProjectScopedListRequest,
    ) -> Result<TaskList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "list-project-tasks",
                &encode_project_scoped_list_request(&req),
            )
            .await?;
        decode_task_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-project-task (request/response).
    pub async fn add_project_task(
        &self,
        req: ProjectTaskOrderRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "add-project-task",
                &encode_project_task_order_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-project-task (request/response).
    pub async fn remove_project_task(
        &self,
        req: ProjectTaskRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "remove-project-task",
                &encode_project_task_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-project-task-position (request/response).
    pub async fn set_project_task_position(
        &self,
        req: ProjectTaskOrderRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "set-project-task-position",
                &encode_project_task_order_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-members (request/response).
    pub async fn list_project_members(&self, req: ProjectID) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "list-project-members",
                &encode_project_list_project_members_request(&req),
            )
            .await?;
        decode_project_list_project_members_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-project-member (request/response).
    pub async fn add_project_member(
        &self,
        req: ProjectMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "add-project-member",
                &encode_project_member_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-project-member (request/response).
    pub async fn remove_project_member(
        &self,
        req: ProjectMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "remove-project-member",
                &encode_project_member_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-owners (request/response).
    pub async fn list_project_owners(&self, req: ProjectID) -> Result<Vec<Member>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "list-project-owners",
                &encode_project_list_project_owners_request(&req),
            )
            .await?;
        decode_project_list_project_owners_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// add-project-owner (request/response).
    pub async fn add_project_owner(
        &self,
        req: ProjectOwnerRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "add-project-owner",
                &encode_project_owner_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-project-owner (request/response).
    pub async fn remove_project_owner(
        &self,
        req: ProjectOwnerRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "remove-project-owner",
                &encode_project_owner_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-milestones (request/response).
    pub async fn list_milestones(&self, req: ProjectID) -> Result<Vec<Milestone>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "list-milestones",
                &encode_project_list_milestones_request(&req),
            )
            .await?;
        decode_project_list_milestones_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// create-milestone (request/response).
    pub async fn create_milestone(&self, req: Milestone) -> Result<Milestone, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "create-milestone",
                &encode_milestone(&req),
            )
            .await?;
        decode_milestone(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-milestone (request/response).
    pub async fn update_milestone(&self, req: Milestone) -> Result<Milestone, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "update-milestone",
                &encode_milestone(&req),
            )
            .await?;
        decode_milestone(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-milestone (request/response).
    pub async fn delete_milestone(&self, req: MilestoneID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "delete-milestone",
                &encode_project_delete_milestone_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-project-visibility (request/response).
    pub async fn set_project_visibility(
        &self,
        req: SetProjectVisibilityRequest,
    ) -> Result<Project, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "set-project-visibility",
                &encode_set_project_visibility_request(&req),
            )
            .await?;
        decode_project(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-project-grants (request/response).
    pub async fn list_project_grants(&self, req: ProjectID) -> Result<Vec<Grant>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "list-project-grants",
                &encode_project_list_project_grants_request(&req),
            )
            .await?;
        decode_project_list_project_grants_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// put-project-grant (request/response).
    pub async fn put_project_grant(
        &self,
        req: PutProjectGrantRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "put-project-grant",
                &encode_put_project_grant_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-project-grant (request/response).
    pub async fn delete_project_grant(
        &self,
        req: ProjectGrantRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ProjectService",
                "delete-project-grant",
                &encode_project_grant_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the EventService service.
pub struct EventAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> EventAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-event (request/response).
    pub async fn create_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call("EventService", "create-event", &encode_event(&req))
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-event (request/response).
    pub async fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "get-event",
                &encode_event_get_event_request(&req),
            )
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-event (request/response).
    pub async fn update_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_resp = self
            .transport
            .call("EventService", "update-event", &encode_event(&req))
            .await?;
        decode_event(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-event (request/response).
    pub async fn delete_event(&self, req: EventID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "delete-event",
                &encode_event_delete_event_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-event-and-future (request/response).
    pub async fn delete_event_and_future(
        &self,
        req: EventID,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "delete-event-and-future",
                &encode_event_delete_event_and_future_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-events (request/response).
    pub async fn list_events(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<Vec<Event>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "list-events",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_event_list_events_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-calendar-view (request/response).
    pub async fn get_calendar_view(&self, req: HouseID) -> Result<CalendarView, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "get-calendar-view",
                &encode_event_get_calendar_view_request(&req),
            )
            .await?;
        decode_calendar_view(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-calendar-view (request/response).
    pub async fn set_calendar_view(&self, req: CalendarView) -> Result<CalendarView, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "EventService",
                "set-calendar-view",
                &encode_calendar_view(&req),
            )
            .await?;
        decode_calendar_view(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the TaskService service.
pub struct TaskAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> TaskAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-task (request/response).
    pub async fn create_task(&self, req: Task) -> Result<Task, ClientError> {
        let csil_resp = self
            .transport
            .call("TaskService", "create-task", &encode_task(&req))
            .await?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-task (request/response).
    pub async fn get_task(&self, req: TaskID) -> Result<Task, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "get-task",
                &encode_task_get_task_request(&req),
            )
            .await?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-task (request/response).
    pub async fn update_task(&self, req: Task) -> Result<Task, ClientError> {
        let csil_resp = self
            .transport
            .call("TaskService", "update-task", &encode_task(&req))
            .await?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-task (request/response).
    pub async fn delete_task(&self, req: TaskID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "delete-task",
                &encode_task_delete_task_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-tasks (request/response).
    pub async fn list_tasks(&self, req: HouseScopedListRequest) -> Result<TaskList, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "list-tasks",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_task_list(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// set-task-visibility (request/response).
    pub async fn set_task_visibility(
        &self,
        req: SetTaskVisibilityRequest,
    ) -> Result<Task, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "set-task-visibility",
                &encode_set_task_visibility_request(&req),
            )
            .await?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-task-grants (request/response).
    pub async fn list_task_grants(&self, req: TaskID) -> Result<Vec<Grant>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "list-task-grants",
                &encode_task_list_task_grants_request(&req),
            )
            .await?;
        decode_task_list_task_grants_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// put-task-grant (request/response).
    pub async fn put_task_grant(
        &self,
        req: PutTaskGrantRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "put-task-grant",
                &encode_put_task_grant_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-task-grant (request/response).
    pub async fn delete_task_grant(&self, req: TaskGrantRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TaskService",
                "delete-task-grant",
                &encode_task_grant_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the DependencyService service.
pub struct DependencyAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> DependencyAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// add-dependency (request/response).
    pub async fn add_dependency(&self, req: DependencyRef) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DependencyService",
                "add-dependency",
                &encode_dependency_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// remove-dependency (request/response).
    pub async fn remove_dependency(
        &self,
        req: DependencyRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DependencyService",
                "remove-dependency",
                &encode_dependency_ref(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-dependencies (request/response).
    pub async fn get_dependencies(
        &self,
        req: DependencyTarget,
    ) -> Result<DependencyGraph, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "DependencyService",
                "get-dependencies",
                &encode_dependency_target(&req),
            )
            .await?;
        decode_dependency_graph(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the CommentService service.
pub struct CommentAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> CommentAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-comment (request/response).
    pub async fn create_comment(&self, req: Comment) -> Result<Comment, ClientError> {
        let csil_resp = self
            .transport
            .call("CommentService", "create-comment", &encode_comment(&req))
            .await?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// get-comment (request/response).
    pub async fn get_comment(&self, req: CommentID) -> Result<Comment, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CommentService",
                "get-comment",
                &encode_comment_get_comment_request(&req),
            )
            .await?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-comment (request/response).
    pub async fn update_comment(&self, req: Comment) -> Result<Comment, ClientError> {
        let csil_resp = self
            .transport
            .call("CommentService", "update-comment", &encode_comment(&req))
            .await?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-comment (request/response).
    pub async fn delete_comment(&self, req: CommentID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CommentService",
                "delete-comment",
                &encode_comment_delete_comment_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-comments (request/response).
    pub async fn list_comments(
        &self,
        req: CommentListRequest,
    ) -> Result<Vec<Comment>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "CommentService",
                "list-comments",
                &encode_comment_list_request(&req),
            )
            .await?;
        decode_comment_list_comments_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the NotificationService service.
pub struct NotificationAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> NotificationAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-notifications (request/response).
    pub async fn list_notifications(
        &self,
        req: NotificationListRequest,
    ) -> Result<Vec<Notification>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "NotificationService",
                "list-notifications",
                &encode_notification_list_request(&req),
            )
            .await?;
        decode_notification_list_notifications_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// unread-count (request/response).
    pub async fn unread_count(&self, req: HouseID) -> Result<NotificationUnreadCount, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "NotificationService",
                "unread-count",
                &encode_notification_unread_count_request(&req),
            )
            .await?;
        decode_notification_unread_count(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// mark-read (request/response).
    pub async fn mark_read(&self, req: NotificationID) -> Result<Notification, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "NotificationService",
                "mark-read",
                &encode_notification_mark_read_request(&req),
            )
            .await?;
        decode_notification(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// mark-all-read (request/response).
    pub async fn mark_all_read(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "NotificationService",
                "mark-all-read",
                &encode_notification_mark_all_read_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the ShareService service.
pub struct ShareAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> ShareAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// create-share (request/response).
    pub async fn create_share(&self, req: Share) -> Result<Share, ClientError> {
        let csil_resp = self
            .transport
            .call("ShareService", "create-share", &encode_share(&req))
            .await?;
        decode_share(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// delete-share (request/response).
    pub async fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ShareService",
                "delete-share",
                &encode_share_delete_share_request(&req),
            )
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// list-shares-by-resource (request/response).
    pub async fn list_shares_by_resource(
        &self,
        req: ResourceRef,
    ) -> Result<Vec<Share>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ShareService",
                "list-shares-by-resource",
                &encode_resource_ref(&req),
            )
            .await?;
        decode_share_list_shares_by_resource_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// check-access (request/response).
    pub async fn check_access(&self, req: ShareAccessRequest) -> Result<Share, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "ShareService",
                "check-access",
                &encode_share_access_request(&req),
            )
            .await?;
        decode_share(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the MemberAuditService service.
pub struct MemberAuditAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> MemberAuditAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-audits-for-member (request/response).
    pub async fn list_audits_for_member(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<Vec<MemberAudit>, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "MemberAuditService",
                "list-audits-for-member",
                &encode_member_scoped_list_request(&req),
            )
            .await?;
        decode_member_audit_list_audits_for_member_response(&csil_resp)
            .map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the SettingsService service.
pub struct SettingsAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> SettingsAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// get-settings (request/response).
    pub async fn get_settings(&self, req: HouseID) -> Result<EffectiveSettings, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SettingsService",
                "get-settings",
                &encode_settings_get_settings_request(&req),
            )
            .await?;
        decode_effective_settings(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// update-settings (request/response).
    pub async fn update_settings(
        &self,
        req: UpdateSettingsRequest,
    ) -> Result<EffectiveSettings, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "SettingsService",
                "update-settings",
                &encode_update_settings_request(&req),
            )
            .await?;
        decode_effective_settings(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the BugService service.
pub struct BugAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> BugAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// report-bug (request/response).
    pub async fn report_bug(&self, req: BugReportRequest) -> Result<Task, ClientError> {
        let csil_resp = self
            .transport
            .call("BugService", "report-bug", &encode_bug_report_request(&req))
            .await?;
        decode_task(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the AuditService service.
pub struct AuditAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> AuditAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// query-audit (request/response).
    pub async fn query_audit(&self, req: AuditQuery) -> Result<AuditPage, ClientError> {
        let csil_resp = self
            .transport
            .call("AuditService", "query-audit", &encode_audit_query(&req))
            .await?;
        decode_audit_page(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}

/// Typed client for the TrashService service.
pub struct TrashAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> TrashAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-trash (request/response).
    pub async fn list_trash(&self, req: HouseScopedListRequest) -> Result<TrashPage, ClientError> {
        let csil_resp = self
            .transport
            .call(
                "TrashService",
                "list-trash",
                &encode_house_scoped_list_request(&req),
            )
            .await?;
        decode_trash_page(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// restore (request/response).
    pub async fn restore(&self, req: RestoreRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("TrashService", "restore", &encode_restore_request(&req))
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }

    /// purge (request/response).
    pub async fn purge(&self, req: PurgeRequest) -> Result<EmptyResponse, ClientError> {
        let csil_resp = self
            .transport
            .call("TrashService", "purge", &encode_purge_request(&req))
            .await?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Transport(e.to_string()))
    }
}
