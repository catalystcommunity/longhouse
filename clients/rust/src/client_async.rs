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

    /// Perform a call whose operation declares several reply arms and return
    /// the status-zero reply `variant` with its payload, so the client can
    /// return the success arm or the declared error arm.
    async fn call_reply(
        &self,
        service: &str,
        op: &str,
        req: &[u8],
    ) -> Result<super::client::WireReply, ClientError>;
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

    /// begin-browser-login (request/response).
    pub async fn begin_browser_login(
        &self,
        req: BeginBrowserLoginRequest,
    ) -> Result<BeginBrowserLoginResponse, ClientError> {
        let csil_req = match encode_begin_browser_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "begin-browser-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "begin-browser-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_begin_browser_login_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-act-as-scope-set (request/response).
    pub async fn get_act_as_scope_set(
        &self,
        req: ActAsScopeSetRequest,
    ) -> Result<ActAsScopeSetResponse, ClientError> {
        let csil_req = match encode_act_as_scope_set_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "get-act-as-scope-set", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "get-act-as-scope-set",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_act_as_scope_set_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// exchange-act-as-credential (request/response).
    pub async fn exchange_act_as_credential(
        &self,
        req: ExchangeActAsCredentialRequest,
    ) -> Result<ActAsTokenResponse, ClientError> {
        let csil_req = match encode_exchange_act_as_credential_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "exchange-act-as-credential", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "exchange-act-as-credential",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_act_as_token_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// login (request/response).
    pub async fn login(&self, req: LoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// complete (request/response).
    pub async fn complete(&self, req: CompleteRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_complete_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "complete", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "complete",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// refresh (request/response).
    pub async fn refresh(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "refresh", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "refresh",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// logout (request/response).
    pub async fn logout(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "logout", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "logout",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// resume-session (request/response).
    pub async fn resume_session(&self, req: EmptyRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "resume-session", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "resume-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// end-session (request/response).
    pub async fn end_session(&self, req: EmptyRequest) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "end-session", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "end-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// me (request/response).
    pub async fn me(&self, req: EmptyRequest) -> Result<MeResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "me", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "me",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_me_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// begin-cli-login (request/response).
    pub async fn begin_cli_login(
        &self,
        req: BeginCliLoginRequest,
    ) -> Result<BeginCliLoginResponse, ClientError> {
        let csil_req = match encode_begin_cli_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "begin-cli-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "begin-cli-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_begin_cli_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// inspect-cli-login (request/response).
    pub async fn inspect_cli_login(
        &self,
        req: ApproveCliLoginRequest,
    ) -> Result<CliLoginRequestInfo, ClientError> {
        let csil_req = match encode_approve_cli_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "inspect-cli-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "inspect-cli-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_cli_login_request_info(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// approve-cli-login (request/response).
    pub async fn approve_cli_login(
        &self,
        req: ApproveCliLoginRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_approve_cli_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "approve-cli-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "approve-cli-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// deny-cli-login (request/response).
    pub async fn deny_cli_login(
        &self,
        req: DenyCliLoginRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_deny_cli_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "deny-cli-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "deny-cli-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// exchange-cli-login (request/response).
    pub async fn exchange_cli_login(
        &self,
        req: ExchangeCliLoginRequest,
    ) -> Result<ExchangeCliLoginResponse, ClientError> {
        let csil_req = match encode_exchange_cli_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "exchange-cli-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "exchange-cli-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_exchange_cli_login_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// refresh-session (request/response).
    pub async fn refresh_session(
        &self,
        req: RefreshSessionRequest,
    ) -> Result<CliTokenResponse, ClientError> {
        let csil_req = match encode_refresh_session_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "refresh-session", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "refresh-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_cli_token_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-sessions (request/response).
    pub async fn list_sessions(
        &self,
        req: EmptyRequest,
    ) -> Result<CliSessionsResponse, ClientError> {
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "list-sessions", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "list-sessions",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_cli_sessions_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// revoke-session (request/response).
    pub async fn revoke_session(
        &self,
        req: RevokeSessionRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_revoke_session_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuthService", "revoke-session", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuthService",
            "revoke-session",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_empty_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DevAuthService", "list-dev-users", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DevAuthService",
            "list-dev-users",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_dev_users_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// dev-login (request/response).
    pub async fn dev_login(&self, req: DevLoginRequest) -> Result<LoginResponse, ClientError> {
        let csil_req = match encode_dev_login_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DevAuthService", "dev-login", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DevAuthService",
            "dev-login",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_login_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_house(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("HouseService", "create-house", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "HouseService",
            "create-house",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_house(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-house (request/response).
    pub async fn get_house(&self, req: HouseID) -> Result<House, ClientError> {
        let csil_req = match encode_house_get_house_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("HouseService", "get-house", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "HouseService",
            "get-house",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_house(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-house (request/response).
    pub async fn update_house(&self, req: House) -> Result<House, ClientError> {
        let csil_req = match encode_house(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("HouseService", "update-house", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "HouseService",
            "update-house",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_house(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-house (request/response).
    pub async fn delete_house(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_house_delete_house_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("HouseService", "delete-house", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "HouseService",
            "delete-house",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-houses (request/response).
    pub async fn list_houses(&self, req: HouseListRequest) -> Result<HouseItems, ClientError> {
        let csil_req = match encode_house_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("HouseService", "list-houses", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "HouseService",
            "list-houses",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_house_list_houses_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_member(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "create-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "create-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_member(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-member (request/response).
    pub async fn get_member(&self, req: MemberID) -> Result<Member, ClientError> {
        let csil_req = match encode_member_get_member_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "get-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "get-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_member(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-member-by-identity (request/response).
    pub async fn get_member_by_identity(&self, req: Member) -> Result<Member, ClientError> {
        let csil_req = match encode_member(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "get-member-by-identity", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "get-member-by-identity",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_member(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-member (request/response).
    pub async fn update_member(&self, req: Member) -> Result<Member, ClientError> {
        let csil_req = match encode_member(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "update-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "update-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_member(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// deactivate-member (request/response).
    pub async fn deactivate_member(&self, req: MemberID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_member_deactivate_member_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "deactivate-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "deactivate-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// reactivate-member (request/response).
    pub async fn reactivate_member(&self, req: MemberID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_member_reactivate_member_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "reactivate-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "reactivate-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-members (request/response).
    pub async fn list_members(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<MemberItems, ClientError> {
        let csil_req = match encode_house_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberService", "list-members", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberService",
            "list-members",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_member_list_members_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_trusted_domain(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrustedDomainService", "add-trusted-domain", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrustedDomainService",
            "add-trusted-domain",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_trusted_domain(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-trusted-domain (request/response).
    pub async fn remove_trusted_domain(
        &self,
        req: TrustedDomainID,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_trusted_domain_remove_trusted_domain_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrustedDomainService", "remove-trusted-domain", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrustedDomainService",
            "remove-trusted-domain",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-trusted-domains (request/response).
    pub async fn list_trusted_domains(
        &self,
        req: HouseID,
    ) -> Result<TrustedDomainItems, ClientError> {
        let csil_req = match encode_trusted_domain_list_trusted_domains_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrustedDomainService", "list-trusted-domains", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrustedDomainService",
            "list-trusted-domains",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_trusted_domain_list_trusted_domains_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// is-domain-trusted (request/response).
    pub async fn is_domain_trusted(&self, req: TrustedDomain) -> Result<BoolResponse, ClientError> {
        let csil_req = match encode_trusted_domain(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrustedDomainService", "is-domain-trusted", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrustedDomainService",
            "is-domain-trusted",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_bool_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_role(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "create-role", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "create-role",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_role(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-role (request/response).
    pub async fn update_role(&self, req: Role) -> Result<Role, ClientError> {
        let csil_req = match encode_role(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "update-role", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "update-role",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_role(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-role (request/response).
    pub async fn delete_role(&self, req: RoleID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_role_delete_role_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "delete-role", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "delete-role",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-roles (request/response).
    pub async fn list_roles(&self, req: HouseScopedListRequest) -> Result<RoleItems, ClientError> {
        let csil_req = match encode_house_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "list-roles", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "list-roles",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_role_list_roles_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// grant-role (request/response).
    pub async fn grant_role(&self, req: MemberRoleRef) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_member_role_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "grant-role", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "grant-role",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// revoke-role (request/response).
    pub async fn revoke_role(&self, req: MemberRoleRef) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_member_role_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "revoke-role", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "revoke-role",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-member-roles (request/response).
    pub async fn list_member_roles(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<RoleItems, ClientError> {
        let csil_req = match encode_member_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("RoleService", "list-member-roles", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "RoleService",
            "list-member-roles",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_role_list_member_roles_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_skill(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "create-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "create-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_skill(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-skill (request/response).
    pub async fn update_skill(&self, req: Skill) -> Result<Skill, ClientError> {
        let csil_req = match encode_skill(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "update-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "update-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_skill(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-skill (request/response).
    pub async fn delete_skill(&self, req: SkillID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_skill_delete_skill_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "delete-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "delete-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-skills (request/response).
    pub async fn list_skills(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<SkillItems, ClientError> {
        let csil_req = match encode_house_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "list-skills", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "list-skills",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_skill_list_skills_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// add-member-skill (request/response).
    pub async fn add_member_skill(
        &self,
        req: MemberSkillRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_member_skill_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "add-member-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "add-member-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-member-skill (request/response).
    pub async fn remove_member_skill(
        &self,
        req: MemberSkillRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_member_skill_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "remove-member-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "remove-member-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-member-skills (request/response).
    pub async fn list_member_skills(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<SkillItems, ClientError> {
        let csil_req = match encode_member_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "list-member-skills", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "list-member-skills",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_skill_list_member_skills_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// add-group-skill (request/response).
    pub async fn add_group_skill(&self, req: GroupSkillRef) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_skill_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "add-group-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "add-group-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-group-skill (request/response).
    pub async fn remove_group_skill(
        &self,
        req: GroupSkillRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_skill_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "remove-group-skill", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "remove-group-skill",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-group-skills (request/response).
    pub async fn list_group_skills(&self, req: GroupID) -> Result<SkillItems, ClientError> {
        let csil_req = match encode_skill_list_group_skills_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SkillService", "list-group-skills", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SkillService",
            "list-group-skills",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_skill_list_group_skills_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_group(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "create-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "create-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-group (request/response).
    pub async fn update_group(&self, req: Group) -> Result<Group, ClientError> {
        let csil_req = match encode_group(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "update-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "update-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-group (request/response).
    pub async fn delete_group(&self, req: GroupID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_delete_group_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "delete-group", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "delete-group",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-groups (request/response).
    pub async fn list_groups(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<GroupItems, ClientError> {
        let csil_req = match encode_house_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "list-groups", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "list-groups",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group_list_groups_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// add-group-member (request/response).
    pub async fn add_group_member(
        &self,
        req: GroupMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_member_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "add-group-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "add-group-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-group-member (request/response).
    pub async fn remove_group_member(
        &self,
        req: GroupMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_group_member_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "remove-group-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "remove-group-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-group-members (request/response).
    pub async fn list_group_members(
        &self,
        req: MemberScopedListRequest,
    ) -> Result<MemberItems, ClientError> {
        let csil_req = match encode_member_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("GroupService", "list-group-members", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "GroupService",
            "list-group-members",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_group_list_group_members_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_project(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "create-project", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "create-project",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-project (request/response).
    pub async fn get_project(&self, req: ProjectID) -> Result<Project, ClientError> {
        let csil_req = match encode_project_get_project_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "get-project", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "get-project",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-project (request/response).
    pub async fn update_project(&self, req: Project) -> Result<Project, ClientError> {
        let csil_req = match encode_project(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "update-project", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "update-project",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-project (request/response).
    pub async fn delete_project(&self, req: ProjectID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_delete_project_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "delete-project", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "delete-project",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-projects (request/response).
    pub async fn list_projects(
        &self,
        req: HouseScopedListRequest,
    ) -> Result<ProjectList, ClientError> {
        let csil_req = match encode_house_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "list-projects", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "list-projects",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-project-tasks (request/response).
    pub async fn list_project_tasks(
        &self,
        req: ProjectScopedListRequest,
    ) -> Result<TaskList, ClientError> {
        let csil_req = match encode_project_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "list-project-tasks", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "list-project-tasks",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// add-project-task (request/response).
    pub async fn add_project_task(
        &self,
        req: ProjectTaskOrderRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_task_order_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "add-project-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "add-project-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-project-task (request/response).
    pub async fn remove_project_task(
        &self,
        req: ProjectTaskRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_task_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "remove-project-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "remove-project-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-project-task-position (request/response).
    pub async fn set_project_task_position(
        &self,
        req: ProjectTaskOrderRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_task_order_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "set-project-task-position", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "set-project-task-position",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-project-members (request/response).
    pub async fn list_project_members(&self, req: ProjectID) -> Result<MemberItems, ClientError> {
        let csil_req = match encode_project_list_project_members_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "list-project-members", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "list-project-members",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project_list_project_members_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// add-project-member (request/response).
    pub async fn add_project_member(
        &self,
        req: ProjectMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_member_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "add-project-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "add-project-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-project-member (request/response).
    pub async fn remove_project_member(
        &self,
        req: ProjectMemberRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_member_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "remove-project-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "remove-project-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-project-owners (request/response).
    pub async fn list_project_owners(&self, req: ProjectID) -> Result<MemberItems, ClientError> {
        let csil_req = match encode_project_list_project_owners_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "list-project-owners", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "list-project-owners",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project_list_project_owners_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// add-project-owner (request/response).
    pub async fn add_project_owner(
        &self,
        req: ProjectOwnerRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_owner_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "add-project-owner", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "add-project-owner",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-project-owner (request/response).
    pub async fn remove_project_owner(
        &self,
        req: ProjectOwnerRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_owner_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "remove-project-owner", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "remove-project-owner",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-milestones (request/response).
    pub async fn list_milestones(&self, req: ProjectID) -> Result<MilestoneItems, ClientError> {
        let csil_req = match encode_project_list_milestones_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "list-milestones", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "list-milestones",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project_list_milestones_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-milestone (request/response).
    pub async fn create_milestone(&self, req: Milestone) -> Result<Milestone, ClientError> {
        let csil_req = match encode_milestone(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "create-milestone", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "create-milestone",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_milestone(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-milestone (request/response).
    pub async fn update_milestone(&self, req: Milestone) -> Result<Milestone, ClientError> {
        let csil_req = match encode_milestone(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "update-milestone", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "update-milestone",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_milestone(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-milestone (request/response).
    pub async fn delete_milestone(&self, req: MilestoneID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_delete_milestone_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "delete-milestone", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "delete-milestone",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-project-visibility (request/response).
    pub async fn set_project_visibility(
        &self,
        req: SetProjectVisibilityRequest,
    ) -> Result<Project, ClientError> {
        let csil_req = match encode_set_project_visibility_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "set-project-visibility", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "set-project-visibility",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-project-grants (request/response).
    pub async fn list_project_grants(&self, req: ProjectID) -> Result<GrantItems, ClientError> {
        let csil_req = match encode_project_list_project_grants_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "list-project-grants", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "list-project-grants",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project_list_project_grants_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// put-project-grant (request/response).
    pub async fn put_project_grant(
        &self,
        req: PutProjectGrantRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_put_project_grant_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "put-project-grant", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "put-project-grant",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-project-grant (request/response).
    pub async fn delete_project_grant(
        &self,
        req: ProjectGrantRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_project_grant_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ProjectService", "delete-project-grant", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ProjectService",
            "delete-project-grant",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_event(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "create-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "create-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-event (request/response).
    pub async fn get_event(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_req = match encode_event_get_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "get-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-event (request/response).
    pub async fn update_event(&self, req: Event) -> Result<Event, ClientError> {
        let csil_req = match encode_event(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "update-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "update-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-event (request/response).
    pub async fn delete_event(&self, req: EventID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_event_delete_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "delete-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "delete-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-event-and-future (request/response).
    pub async fn delete_event_and_future(
        &self,
        req: EventID,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_event_delete_event_and_future_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "delete-event-and-future", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "delete-event-and-future",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-events (request/response).
    pub async fn list_events(&self, req: EventListRequest) -> Result<EventItems, ClientError> {
        let csil_req = match encode_event_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-events", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "list-events",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event_list_events_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-calendar-view (request/response).
    pub async fn get_calendar_view(&self, req: HouseID) -> Result<CalendarView, ClientError> {
        let csil_req = match encode_event_get_calendar_view_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "get-calendar-view", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "get-calendar-view",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_view(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-calendar-view (request/response).
    pub async fn set_calendar_view(&self, req: CalendarView) -> Result<CalendarView, ClientError> {
        let csil_req = match encode_calendar_view(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "set-calendar-view", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "set-calendar-view",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_view(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// accept-event (request/response).
    pub async fn accept_event(&self, req: EventDecision) -> Result<Event, ClientError> {
        let csil_req = match encode_event_decision(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "accept-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "accept-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// recover-edit (request/response).
    pub async fn recover_edit(&self, req: EventID) -> Result<Event, ClientError> {
        let csil_req = match encode_event_recover_edit_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "recover-edit", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "recover-edit",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// decline-event (request/response).
    pub async fn decline_event(&self, req: EventDecision) -> Result<Event, ClientError> {
        let csil_req = match encode_event_decision(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "decline-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "decline-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-local-events (request/response).
    pub async fn list_local_events(&self, req: HouseID) -> Result<EventItems, ClientError> {
        let csil_req = match encode_event_list_local_events_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("EventService", "list-local-events", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "EventService",
            "list-local-events",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event_list_local_events_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }
}

/// Typed client for the CalendarService service.
pub struct CalendarAsyncClient<T: AsyncTransport> {
    #[allow(dead_code)]
    transport: T,
}

impl<T: AsyncTransport> CalendarAsyncClient<T> {
    pub fn new(transport: T) -> Self {
        Self { transport }
    }

    /// list-project-deadlines (request/response).
    pub async fn list_project_deadlines(
        &self,
        req: ProjectDeadlineRequest,
    ) -> Result<ProjectDeadlineList, ClientError> {
        let csil_req = match encode_project_deadline_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "list-project-deadlines", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "list-project-deadlines",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_project_deadline_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-status (request/response).
    pub async fn get_status(&self, req: HouseID) -> Result<CalendarStatus, ClientError> {
        let csil_req = match encode_calendar_get_status_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "get-status", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "get-status",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_status(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// configure-calendar (request/response).
    pub async fn configure_calendar(
        &self,
        req: ConfigureCalendarRequest,
    ) -> Result<CalendarStatus, ClientError> {
        let csil_req = match encode_configure_calendar_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "configure-calendar", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "configure-calendar",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_status(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// begin-connect (request/response).
    pub async fn begin_connect(
        &self,
        req: HouseID,
    ) -> Result<CalendarConnectResponse, ClientError> {
        let csil_req = match encode_calendar_begin_connect_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "begin-connect", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "begin-connect",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_connect_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// complete-connect (request/response).
    pub async fn complete_connect(
        &self,
        req: CompleteCalendarConnectRequest,
    ) -> Result<CalendarStatus, ClientError> {
        let csil_req = match encode_complete_calendar_connect_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "complete-connect", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "complete-connect",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_status(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// disconnect (request/response).
    pub async fn disconnect(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_calendar_disconnect_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "disconnect", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "disconnect",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// preview-migration (request/response).
    pub async fn preview_migration(
        &self,
        req: HouseID,
    ) -> Result<CalendarMigrationRowItems, ClientError> {
        let csil_req = match encode_calendar_preview_migration_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "preview-migration", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "preview-migration",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_preview_migration_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-trash (request/response).
    pub async fn list_trash(&self, req: HouseID) -> Result<CalendarTrashEntryItems, ClientError> {
        let csil_req = match encode_calendar_list_trash_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "list-trash", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "list-trash",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_calendar_list_trash_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// restore-event (request/response).
    pub async fn restore_event(
        &self,
        req: RestoreCalendarEventRequest,
    ) -> Result<Event, ClientError> {
        let csil_req = match encode_restore_calendar_event_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CalendarService", "restore-event", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CalendarService",
            "restore-event",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_event(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_task(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "create-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "create-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-task (request/response).
    pub async fn get_task(&self, req: TaskID) -> Result<Task, ClientError> {
        let csil_req = match encode_task_get_task_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "get-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "get-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-task (request/response).
    pub async fn update_task(&self, req: Task) -> Result<Task, ClientError> {
        let csil_req = match encode_task(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "update-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "update-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-task (request/response).
    pub async fn delete_task(&self, req: TaskID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_task_delete_task_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "delete-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "delete-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-tasks (request/response).
    pub async fn list_tasks(&self, req: TaskListRequest) -> Result<TaskList, ClientError> {
        let csil_req = match encode_task_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "list-tasks", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "list-tasks",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task_list(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// create-personal-task (request/response).
    pub async fn create_personal_task(&self, req: Task) -> Result<Task, ClientError> {
        let csil_req = match encode_task(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "create-personal-task", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "create-personal-task",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// set-task-visibility (request/response).
    pub async fn set_task_visibility(
        &self,
        req: SetTaskVisibilityRequest,
    ) -> Result<Task, ClientError> {
        let csil_req = match encode_set_task_visibility_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "set-task-visibility", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "set-task-visibility",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-task-grants (request/response).
    pub async fn list_task_grants(&self, req: TaskID) -> Result<GrantItems, ClientError> {
        let csil_req = match encode_task_list_task_grants_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "list-task-grants", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "list-task-grants",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task_list_task_grants_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// put-task-grant (request/response).
    pub async fn put_task_grant(
        &self,
        req: PutTaskGrantRequest,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_put_task_grant_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "put-task-grant", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "put-task-grant",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-task-grant (request/response).
    pub async fn delete_task_grant(&self, req: TaskGrantRef) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_task_grant_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TaskService", "delete-task-grant", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TaskService",
            "delete-task-grant",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_dependency_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DependencyService", "add-dependency", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DependencyService",
            "add-dependency",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// remove-dependency (request/response).
    pub async fn remove_dependency(
        &self,
        req: DependencyRef,
    ) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_dependency_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DependencyService", "remove-dependency", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DependencyService",
            "remove-dependency",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-dependencies (request/response).
    pub async fn get_dependencies(
        &self,
        req: DependencyTarget,
    ) -> Result<DependencyGraph, ClientError> {
        let csil_req = match encode_dependency_target(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("DependencyService", "get-dependencies", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "DependencyService",
            "get-dependencies",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_dependency_graph(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_comment(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CommentService", "create-comment", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CommentService",
            "create-comment",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// get-comment (request/response).
    pub async fn get_comment(&self, req: CommentID) -> Result<Comment, ClientError> {
        let csil_req = match encode_comment_get_comment_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CommentService", "get-comment", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CommentService",
            "get-comment",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-comment (request/response).
    pub async fn update_comment(&self, req: Comment) -> Result<Comment, ClientError> {
        let csil_req = match encode_comment(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CommentService", "update-comment", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CommentService",
            "update-comment",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_comment(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-comment (request/response).
    pub async fn delete_comment(&self, req: CommentID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_comment_delete_comment_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CommentService", "delete-comment", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CommentService",
            "delete-comment",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-comments (request/response).
    pub async fn list_comments(
        &self,
        req: CommentListRequest,
    ) -> Result<CommentItems, ClientError> {
        let csil_req = match encode_comment_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("CommentService", "list-comments", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "CommentService",
            "list-comments",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_comment_list_comments_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
    ) -> Result<NotificationItems, ClientError> {
        let csil_req = match encode_notification_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("NotificationService", "list-notifications", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "NotificationService",
            "list-notifications",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_notification_list_notifications_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// unread-count (request/response).
    pub async fn unread_count(&self, req: HouseID) -> Result<NotificationUnreadCount, ClientError> {
        let csil_req = match encode_notification_unread_count_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("NotificationService", "unread-count", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "NotificationService",
            "unread-count",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_notification_unread_count(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// mark-read (request/response).
    pub async fn mark_read(&self, req: NotificationID) -> Result<Notification, ClientError> {
        let csil_req = match encode_notification_mark_read_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("NotificationService", "mark-read", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "NotificationService",
            "mark-read",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_notification(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// mark-all-read (request/response).
    pub async fn mark_all_read(&self, req: HouseID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_notification_mark_all_read_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("NotificationService", "mark-all-read", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "NotificationService",
            "mark-all-read",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_share(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "create-share", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "create-share",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// delete-share (request/response).
    pub async fn delete_share(&self, req: ShareID) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_share_delete_share_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "delete-share", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "delete-share",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// list-shares-by-resource (request/response).
    pub async fn list_shares_by_resource(
        &self,
        req: ResourceRef,
    ) -> Result<ShareItems, ClientError> {
        let csil_req = match encode_resource_ref(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "list-shares-by-resource", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "list-shares-by-resource",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share_list_shares_by_resource_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// check-access (request/response).
    pub async fn check_access(&self, req: ShareAccessRequest) -> Result<Share, ClientError> {
        let csil_req = match encode_share_access_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("ShareService", "check-access", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "ShareService",
            "check-access",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_share(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
    ) -> Result<MemberAuditItems, ClientError> {
        let csil_req = match encode_member_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("MemberAuditService", "list-audits-for-member", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "MemberAuditService",
            "list-audits-for-member",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_member_audit_list_audits_for_member_response(&csil_resp)
            .map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_settings_get_settings_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SettingsService", "get-settings", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SettingsService",
            "get-settings",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_effective_settings(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// update-settings (request/response).
    pub async fn update_settings(
        &self,
        req: UpdateSettingsRequest,
    ) -> Result<EffectiveSettings, ClientError> {
        let csil_req = match encode_update_settings_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("SettingsService", "update-settings", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "SettingsService",
            "update-settings",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_effective_settings(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_bug_report_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("BugService", "report-bug", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "BugService",
            "report-bug",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_task(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_audit_query(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("AuditService", "query-audit", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "AuditService",
            "query-audit",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_audit_page(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
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
        let csil_req = match encode_house_scoped_list_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrashService", "list-trash", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrashService",
            "list-trash",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_trash_page(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// restore (request/response).
    pub async fn restore(&self, req: RestoreRequest) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_restore_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrashService", "restore", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrashService",
            "restore",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }

    /// purge (request/response).
    pub async fn purge(&self, req: PurgeRequest) -> Result<EmptyResponse, ClientError> {
        let csil_req = match encode_purge_request(&req) {
            Ok(csil_bytes) => csil_bytes,
            Err(e) => return Err(ClientError::Encode(e.to_string())),
        };
        let csil_reply = self
            .transport
            .call_reply("TrashService", "purge", &csil_req)
            .await?;
        let csil_resp = super::client::resolve_rpc_reply(
            "TrashService",
            "purge",
            csil_reply.variant.as_deref(),
            csil_reply.payload,
        )?;
        decode_empty_response(&csil_resp).map_err(|e| ClientError::Decode(e.to_string()))
    }
}
