//! Generated self-contained canonical-CBOR codec from CSIL specification.
//!
//! CSIL is the CBOR Service Interface Language; this codec owns the payload
//! wire (a CBOR map keyed by the verbatim CSIL field name in canonical RFC
//! 8949 order) so the generated types need no serde derive. One
//! `encode_`/`decode_` pair is emitted per record type.
#![allow(dead_code, clippy::vec_init_then_push)]

use super::types::*;

/// A decode failure: the CBOR was malformed or did not match the expected shape.
#[derive(Debug, Clone, PartialEq)]
pub struct CsilCborError(pub String);

impl std::fmt::Display for CsilCborError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for CsilCborError {}

/// A minimal canonical-CBOR value tree: a closed set of variants the generated codec
/// builds and walks. A map is an ordered list of pairs, so the encoder controls the
/// wire order of a record's keys explicitly (laid down in canonical order).
#[derive(Debug, Clone, PartialEq)]
pub enum CsilCborValue {
    Uint(u64),
    Int(i64),
    Bool(bool),
    Float(f64),
    Null,
    Text(String),
    Bytes(Vec<u8>),
    Array(Vec<CsilCborValue>),
    Map(Vec<(CsilCborValue, CsilCborValue)>),
    Tag(u64, Box<CsilCborValue>),
}

fn cbor_int(x: i64) -> CsilCborValue {
    CsilCborValue::Int(x)
}
fn cbor_uint(x: u64) -> CsilCborValue {
    CsilCborValue::Uint(x)
}
fn cbor_float(x: f64) -> CsilCborValue {
    CsilCborValue::Float(x)
}
fn cbor_bool(x: bool) -> CsilCborValue {
    CsilCborValue::Bool(x)
}
fn cbor_text(x: &str) -> CsilCborValue {
    CsilCborValue::Text(x.to_string())
}
fn cbor_bytes(x: &[u8]) -> CsilCborValue {
    CsilCborValue::Bytes(x.to_vec())
}

/// Serialize a value tree to canonical CBOR bytes.
fn cbor_encode(v: &CsilCborValue) -> Vec<u8> {
    let mut out = Vec::new();
    cbor_enc(v, &mut out);
    out
}

fn cbor_head(major: u8, n: u64, out: &mut Vec<u8>) {
    let mt = major << 5;
    if n < 24 {
        out.push(mt | n as u8);
    } else if n < 0x100 {
        out.push(mt | 24);
        out.push(n as u8);
    } else if n < 0x10000 {
        out.push(mt | 25);
        out.extend_from_slice(&(n as u16).to_be_bytes());
    } else if n < 0x1_0000_0000 {
        out.push(mt | 26);
        out.extend_from_slice(&(n as u32).to_be_bytes());
    } else {
        out.push(mt | 27);
        out.extend_from_slice(&n.to_be_bytes());
    }
}

fn cbor_enc(v: &CsilCborValue, out: &mut Vec<u8>) {
    match v {
        CsilCborValue::Uint(x) => cbor_head(0, *x, out),
        // A non-negative `Int` rides major type 0 so it is byte-identical to a `Uint`
        // of the same magnitude; only a genuinely negative value uses major type 1.
        CsilCborValue::Int(x) => {
            if *x >= 0 {
                cbor_head(0, *x as u64, out);
            } else {
                cbor_head(1, (-(*x + 1)) as u64, out);
            }
        }
        CsilCborValue::Bool(x) => out.push(if *x { 0xf5 } else { 0xf4 }),
        CsilCborValue::Null => out.push(0xf6),
        CsilCborValue::Float(x) => {
            out.push(0xfb);
            out.extend_from_slice(&x.to_bits().to_be_bytes());
        }
        CsilCborValue::Text(s) => {
            let bytes = s.as_bytes();
            cbor_head(3, bytes.len() as u64, out);
            out.extend_from_slice(bytes);
        }
        CsilCborValue::Bytes(b) => {
            cbor_head(2, b.len() as u64, out);
            out.extend_from_slice(b);
        }
        CsilCborValue::Array(items) => {
            cbor_head(4, items.len() as u64, out);
            for item in items {
                cbor_enc(item, out);
            }
        }
        CsilCborValue::Map(entries) => {
            cbor_head(5, entries.len() as u64, out);
            for (k, val) in entries {
                cbor_enc(k, out);
                cbor_enc(val, out);
            }
        }
        CsilCborValue::Tag(num, inner) => {
            cbor_head(6, *num, out);
            cbor_enc(inner, out);
        }
    }
}

/// Parse a full CBOR item and reject trailing bytes, so a payload that is not
/// exactly one value is an error rather than a silently-truncated read.
fn cbor_decode(b: &[u8]) -> Result<CsilCborValue, CsilCborError> {
    let mut pos = 0usize;
    let v = cbor_dec(b, &mut pos)?;
    if pos != b.len() {
        return Err(CsilCborError(format!(
            "csil cbor: {} trailing bytes",
            b.len() - pos
        )));
    }
    Ok(v)
}

fn cbor_read_arg(b: &[u8], pos: &mut usize, low: u8) -> Result<u64, CsilCborError> {
    if low < 24 {
        *pos += 1;
        return Ok(low as u64);
    }
    let width = match low {
        24 => 1usize,
        25 => 2,
        26 => 4,
        27 => 8,
        _ => return Err(CsilCborError(format!("csil cbor: reserved additional info {low}"))),
    };
    if *pos + 1 + width > b.len() {
        return Err(CsilCborError("csil cbor: truncated argument".to_string()));
    }
    let mut v = 0u64;
    for &byte in &b[*pos + 1..*pos + 1 + width] {
        v = (v << 8) | byte as u64;
    }
    *pos += 1 + width;
    Ok(v)
}

fn cbor_dec(b: &[u8], pos: &mut usize) -> Result<CsilCborValue, CsilCborError> {
    if *pos >= b.len() {
        return Err(CsilCborError("csil cbor: unexpected end of input".to_string()));
    }
    let ib = b[*pos];
    let major = ib >> 5;
    let low = ib & 0x1f;
    if major == 7 {
        return match low {
            20 => {
                *pos += 1;
                Ok(CsilCborValue::Bool(false))
            }
            21 => {
                *pos += 1;
                Ok(CsilCborValue::Bool(true))
            }
            22 | 23 => {
                *pos += 1;
                Ok(CsilCborValue::Null)
            }
            26 => {
                let bits = cbor_read_arg(b, pos, low)?;
                Ok(CsilCborValue::Float(f32::from_bits(bits as u32) as f64))
            }
            27 => {
                let bits = cbor_read_arg(b, pos, low)?;
                Ok(CsilCborValue::Float(f64::from_bits(bits)))
            }
            _ => Err(CsilCborError(format!("csil cbor: unsupported simple value {low}"))),
        };
    }
    let arg = cbor_read_arg(b, pos, low)?;
    match major {
        0 => Ok(CsilCborValue::Uint(arg)),
        1 => {
            if arg > i64::MAX as u64 {
                return Err(CsilCborError("csil cbor: negative integer out of range".to_string()));
            }
            Ok(CsilCborValue::Int(-1 - arg as i64))
        }
        2 => {
            let n = arg as usize;
            if *pos + n > b.len() {
                return Err(CsilCborError("csil cbor: truncated byte string".to_string()));
            }
            let slice = b[*pos..*pos + n].to_vec();
            *pos += n;
            Ok(CsilCborValue::Bytes(slice))
        }
        3 => {
            let n = arg as usize;
            if *pos + n > b.len() {
                return Err(CsilCborError("csil cbor: truncated text string".to_string()));
            }
            let s = std::str::from_utf8(&b[*pos..*pos + n])
                .map_err(|e| CsilCborError(format!("csil cbor: invalid utf-8: {e}")))?
                .to_string();
            *pos += n;
            Ok(CsilCborValue::Text(s))
        }
        4 => {
            let n = arg as usize;
            let mut items = Vec::with_capacity(n);
            for _ in 0..n {
                items.push(cbor_dec(b, pos)?);
            }
            Ok(CsilCborValue::Array(items))
        }
        5 => {
            let n = arg as usize;
            let mut entries = Vec::with_capacity(n);
            for _ in 0..n {
                let k = cbor_dec(b, pos)?;
                let val = cbor_dec(b, pos)?;
                entries.push((k, val));
            }
            Ok(CsilCborValue::Map(entries))
        }
        6 => {
            let inner = cbor_dec(b, pos)?;
            Ok(CsilCborValue::Tag(arg, Box::new(inner)))
        }
        _ => Err(CsilCborError(format!("csil cbor: unexpected major type {major}"))),
    }
}

/// Map a typed slice to a CBOR array via the per-element encoder.
fn cbor_enc_array<E>(xs: &[E], f: impl Fn(&E) -> CsilCborValue) -> CsilCborValue {
    CsilCborValue::Array(xs.iter().map(f).collect())
}

/// Map a typed map to a CBOR map. Rust `HashMap` iteration is unordered, so the inner
/// map's entry order is not canonicalized; the record's own keys (laid down at
/// generation time) are what the cross-language wire contract pins.
fn cbor_enc_map<K, V>(
    m: &std::collections::HashMap<K, V>,
    kf: impl Fn(&K) -> CsilCborValue,
    vf: impl Fn(&V) -> CsilCborValue,
) -> CsilCborValue {
    CsilCborValue::Map(m.iter().map(|(k, v)| (kf(k), vf(v))).collect())
}

fn cbor_dec_array<E>(
    v: &CsilCborValue,
    f: impl Fn(&CsilCborValue) -> Result<E, CsilCborError>,
) -> Result<Vec<E>, CsilCborError> {
    cbor_as_array(v)?.iter().map(f).collect()
}

fn cbor_dec_map<K: std::cmp::Eq + std::hash::Hash, V>(
    v: &CsilCborValue,
    kf: impl Fn(&CsilCborValue) -> Result<K, CsilCborError>,
    vf: impl Fn(&CsilCborValue) -> Result<V, CsilCborError>,
) -> Result<std::collections::HashMap<K, V>, CsilCborError> {
    let entries = cbor_as_map(v)?;
    let mut out = std::collections::HashMap::with_capacity(entries.len());
    for (k, val) in entries {
        out.insert(kf(k)?, vf(val)?);
    }
    Ok(out)
}

fn cbor_map_get<'a>(v: &'a CsilCborValue, key: &str) -> Option<&'a CsilCborValue> {
    if let CsilCborValue::Map(entries) = v {
        for (k, val) in entries {
            if let CsilCborValue::Text(name) = k {
                if name == key {
                    return Some(val);
                }
            }
        }
    }
    None
}

fn cbor_require<'a>(v: &'a CsilCborValue, key: &str) -> Result<&'a CsilCborValue, CsilCborError> {
    cbor_map_get(v, key)
        .ok_or_else(|| CsilCborError(format!("csil cbor: missing field {key:?}")))
}

fn cbor_as_i64(v: &CsilCborValue) -> Result<i64, CsilCborError> {
    match v {
        CsilCborValue::Uint(x) => i64::try_from(*x)
            .map_err(|_| CsilCborError("csil cbor: integer overflows i64".to_string())),
        CsilCborValue::Int(x) => Ok(*x),
        _ => Err(CsilCborError("csil cbor: expected integer".to_string())),
    }
}

fn cbor_as_u64(v: &CsilCborValue) -> Result<u64, CsilCborError> {
    match v {
        CsilCborValue::Uint(x) => Ok(*x),
        CsilCborValue::Int(x) if *x >= 0 => Ok(*x as u64),
        CsilCborValue::Int(_) => {
            Err(CsilCborError("csil cbor: negative integer where unsigned expected".to_string()))
        }
        _ => Err(CsilCborError("csil cbor: expected unsigned integer".to_string())),
    }
}

fn cbor_as_f64(v: &CsilCborValue) -> Result<f64, CsilCborError> {
    match v {
        CsilCborValue::Float(x) => Ok(*x),
        CsilCborValue::Uint(x) => Ok(*x as f64),
        CsilCborValue::Int(x) => Ok(*x as f64),
        _ => Err(CsilCborError("csil cbor: expected float".to_string())),
    }
}

fn cbor_as_bool(v: &CsilCborValue) -> Result<bool, CsilCborError> {
    match v {
        CsilCborValue::Bool(b) => Ok(*b),
        _ => Err(CsilCborError("csil cbor: expected bool".to_string())),
    }
}

fn cbor_as_text(v: &CsilCborValue) -> Result<String, CsilCborError> {
    match v {
        CsilCborValue::Text(s) => Ok(s.clone()),
        _ => Err(CsilCborError("csil cbor: expected text".to_string())),
    }
}

fn cbor_as_bytes(v: &CsilCborValue) -> Result<Vec<u8>, CsilCborError> {
    match v {
        CsilCborValue::Bytes(b) => Ok(b.clone()),
        _ => Err(CsilCborError("csil cbor: expected byte string".to_string())),
    }
}

fn cbor_as_array(v: &CsilCborValue) -> Result<&[CsilCborValue], CsilCborError> {
    match v {
        CsilCborValue::Array(a) => Ok(a),
        _ => Err(CsilCborError("csil cbor: expected array".to_string())),
    }
}

fn cbor_as_map(v: &CsilCborValue) -> Result<&[(CsilCborValue, CsilCborValue)], CsilCborError> {
    match v {
        CsilCborValue::Map(m) => Ok(m),
        _ => Err(CsilCborError("csil cbor: expected map".to_string())),
    }
}

/// Build the canonical CBOR value tree for a House.
fn csil_enc_house(csil_v: &House) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(5);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a House from a decoded CBOR value tree.
fn csil_dec_house(csil_root: &CsilCborValue) -> Result<House, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(House {
        house_id,
        name,
        description,
        created_at,
        updated_at,
    })
}

/// Encode a House to canonical CSIL CBOR bytes.
pub fn encode_house(csil_v: &House) -> Vec<u8> {
    cbor_encode(&csil_enc_house(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a House.
pub fn decode_house(csil_data: &[u8]) -> Result<House, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_house(&csil_root)
}

/// Build the canonical CBOR value tree for a Member.
fn csil_enc_member(csil_v: &Member) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(12);
    if let Some(csil_inner) = &csil_v.email {
        csil_entries.push((cbor_text("email"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    if let Some(csil_inner) = &csil_v.avatar_url {
        csil_entries.push((cbor_text("avatar_url"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.last_seen_at {
        csil_entries.push((cbor_text("last_seen_at"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.deactivated_at {
        csil_entries.push((cbor_text("deactivated_at"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("linkkeys_domain"), cbor_text(&csil_v.linkkeys_domain)));
    csil_entries.push((cbor_text("linkkeys_user_id"), cbor_text(&csil_v.linkkeys_user_id)));
    if let Some(csil_inner) = &csil_v.cached_public_key {
        csil_entries.push((cbor_text("cached_public_key"), cbor_bytes(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Member from a decoded CBOR value tree.
fn csil_dec_member(csil_root: &CsilCborValue) -> Result<Member, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let linkkeys_domain = {
        let csil_field = cbor_require(csil_root, "linkkeys_domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let linkkeys_user_id = {
        let csil_field = cbor_require(csil_root, "linkkeys_user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let email = match cbor_map_get(csil_root, "email") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let avatar_url = match cbor_map_get(csil_root, "avatar_url") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let cached_public_key = match cbor_map_get(csil_root, "cached_public_key") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bytes;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let last_seen_at = match cbor_map_get(csil_root, "last_seen_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let deactivated_at = match cbor_map_get(csil_root, "deactivated_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(Member {
        member_id,
        house_id,
        linkkeys_domain,
        linkkeys_user_id,
        display_name,
        email,
        avatar_url,
        cached_public_key,
        created_at,
        updated_at,
        last_seen_at,
        deactivated_at,
    })
}

/// Encode a Member to canonical CSIL CBOR bytes.
pub fn encode_member(csil_v: &Member) -> Vec<u8> {
    cbor_encode(&csil_enc_member(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Member.
pub fn decode_member(csil_data: &[u8]) -> Result<Member, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member(&csil_root)
}

/// Build the canonical CBOR value tree for a TrustedDomain.
fn csil_enc_trusted_domain(csil_v: &TrustedDomain) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("trusted_domain_id"), cbor_text(&csil_v.trusted_domain_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a TrustedDomain from a decoded CBOR value tree.
fn csil_dec_trusted_domain(csil_root: &CsilCborValue) -> Result<TrustedDomain, CsilCborError> {
    let trusted_domain_id = {
        let csil_field = cbor_require(csil_root, "trusted_domain_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(TrustedDomain {
        trusted_domain_id,
        house_id,
        domain,
        created_at,
    })
}

/// Encode a TrustedDomain to canonical CSIL CBOR bytes.
pub fn encode_trusted_domain(csil_v: &TrustedDomain) -> Vec<u8> {
    cbor_encode(&csil_enc_trusted_domain(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a TrustedDomain.
pub fn decode_trusted_domain(csil_data: &[u8]) -> Result<TrustedDomain, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_trusted_domain(&csil_root)
}

/// Build the canonical CBOR value tree for a Role.
fn csil_enc_role(csil_v: &Role) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(6);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    csil_entries.push((cbor_text("role_id"), cbor_text(&csil_v.role_id)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Role from a decoded CBOR value tree.
fn csil_dec_role(csil_root: &CsilCborValue) -> Result<Role, CsilCborError> {
    let role_id = {
        let csil_field = cbor_require(csil_root, "role_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Role {
        role_id,
        house_id,
        name,
        description,
        created_at,
        updated_at,
    })
}

/// Encode a Role to canonical CSIL CBOR bytes.
pub fn encode_role(csil_v: &Role) -> Vec<u8> {
    cbor_encode(&csil_enc_role(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Role.
pub fn decode_role(csil_data: &[u8]) -> Result<Role, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_role(&csil_root)
}

/// Build the canonical CBOR value tree for a MemberRole.
fn csil_enc_member_role(csil_v: &MemberRole) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("role_id"), cbor_text(&csil_v.role_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MemberRole from a decoded CBOR value tree.
fn csil_dec_member_role(csil_root: &CsilCborValue) -> Result<MemberRole, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let role_id = {
        let csil_field = cbor_require(csil_root, "role_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(MemberRole {
        member_id,
        role_id,
        created_at,
    })
}

/// Encode a MemberRole to canonical CSIL CBOR bytes.
pub fn encode_member_role(csil_v: &MemberRole) -> Vec<u8> {
    cbor_encode(&csil_enc_member_role(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MemberRole.
pub fn decode_member_role(csil_data: &[u8]) -> Result<MemberRole, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member_role(&csil_root)
}

/// Build the canonical CBOR value tree for a MemberAudit.
fn csil_enc_member_audit(csil_v: &MemberAudit) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(9);
    csil_entries.push((cbor_text("action"), cbor_text(&csil_v.action)));
    if let Some(csil_inner) = &csil_v.detail {
        csil_entries.push((cbor_text("detail"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("audit_id"), cbor_text(&csil_v.audit_id)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.target_id {
        csil_entries.push((cbor_text("target_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.target_type {
        csil_entries.push((cbor_text("target_type"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.actor_member_id {
        csil_entries.push((cbor_text("actor_member_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("subject_member_id"), cbor_text(&csil_v.subject_member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MemberAudit from a decoded CBOR value tree.
fn csil_dec_member_audit(csil_root: &CsilCborValue) -> Result<MemberAudit, CsilCborError> {
    let audit_id = {
        let csil_field = cbor_require(csil_root, "audit_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let subject_member_id = {
        let csil_field = cbor_require(csil_root, "subject_member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let actor_member_id = match cbor_map_get(csil_root, "actor_member_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let action = {
        let csil_field = cbor_require(csil_root, "action")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let target_type = match cbor_map_get(csil_root, "target_type") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let target_id = match cbor_map_get(csil_root, "target_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let detail = match cbor_map_get(csil_root, "detail") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(MemberAudit {
        audit_id,
        house_id,
        subject_member_id,
        actor_member_id,
        action,
        target_type,
        target_id,
        detail,
        created_at,
    })
}

/// Encode a MemberAudit to canonical CSIL CBOR bytes.
pub fn encode_member_audit(csil_v: &MemberAudit) -> Vec<u8> {
    cbor_encode(&csil_enc_member_audit(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MemberAudit.
pub fn decode_member_audit(csil_data: &[u8]) -> Result<MemberAudit, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member_audit(&csil_root)
}

/// Build the canonical CBOR value tree for a Skill.
fn csil_enc_skill(csil_v: &Skill) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(6);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("skill_id"), cbor_text(&csil_v.skill_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Skill from a decoded CBOR value tree.
fn csil_dec_skill(csil_root: &CsilCborValue) -> Result<Skill, CsilCborError> {
    let skill_id = {
        let csil_field = cbor_require(csil_root, "skill_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Skill {
        skill_id,
        house_id,
        name,
        description,
        created_at,
        updated_at,
    })
}

/// Encode a Skill to canonical CSIL CBOR bytes.
pub fn encode_skill(csil_v: &Skill) -> Vec<u8> {
    cbor_encode(&csil_enc_skill(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Skill.
pub fn decode_skill(csil_data: &[u8]) -> Result<Skill, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_skill(&csil_root)
}

/// Build the canonical CBOR value tree for a MemberSkill.
fn csil_enc_member_skill(csil_v: &MemberSkill) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("skill_id"), cbor_text(&csil_v.skill_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MemberSkill from a decoded CBOR value tree.
fn csil_dec_member_skill(csil_root: &CsilCborValue) -> Result<MemberSkill, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let skill_id = {
        let csil_field = cbor_require(csil_root, "skill_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(MemberSkill {
        member_id,
        skill_id,
        created_at,
    })
}

/// Encode a MemberSkill to canonical CSIL CBOR bytes.
pub fn encode_member_skill(csil_v: &MemberSkill) -> Vec<u8> {
    cbor_encode(&csil_enc_member_skill(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MemberSkill.
pub fn decode_member_skill(csil_data: &[u8]) -> Result<MemberSkill, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member_skill(&csil_root)
}

/// Build the canonical CBOR value tree for a GroupSkill.
fn csil_enc_group_skill(csil_v: &GroupSkill) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("group_id"), cbor_text(&csil_v.group_id)));
    csil_entries.push((cbor_text("skill_id"), cbor_text(&csil_v.skill_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a GroupSkill from a decoded CBOR value tree.
fn csil_dec_group_skill(csil_root: &CsilCborValue) -> Result<GroupSkill, CsilCborError> {
    let group_id = {
        let csil_field = cbor_require(csil_root, "group_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let skill_id = {
        let csil_field = cbor_require(csil_root, "skill_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(GroupSkill {
        group_id,
        skill_id,
        created_at,
    })
}

/// Encode a GroupSkill to canonical CSIL CBOR bytes.
pub fn encode_group_skill(csil_v: &GroupSkill) -> Vec<u8> {
    cbor_encode(&csil_enc_group_skill(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a GroupSkill.
pub fn decode_group_skill(csil_data: &[u8]) -> Result<GroupSkill, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_group_skill(&csil_root)
}

/// Build the canonical CBOR value tree for a Group.
fn csil_enc_group(csil_v: &Group) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(6);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    csil_entries.push((cbor_text("group_id"), cbor_text(&csil_v.group_id)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Group from a decoded CBOR value tree.
fn csil_dec_group(csil_root: &CsilCborValue) -> Result<Group, CsilCborError> {
    let group_id = {
        let csil_field = cbor_require(csil_root, "group_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Group {
        group_id,
        house_id,
        name,
        description,
        created_at,
        updated_at,
    })
}

/// Encode a Group to canonical CSIL CBOR bytes.
pub fn encode_group(csil_v: &Group) -> Vec<u8> {
    cbor_encode(&csil_enc_group(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Group.
pub fn decode_group(csil_data: &[u8]) -> Result<Group, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_group(&csil_root)
}

/// Build the canonical CBOR value tree for a GroupMember.
fn csil_enc_group_member(csil_v: &GroupMember) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("group_id"), cbor_text(&csil_v.group_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a GroupMember from a decoded CBOR value tree.
fn csil_dec_group_member(csil_root: &CsilCborValue) -> Result<GroupMember, CsilCborError> {
    let group_id = {
        let csil_field = cbor_require(csil_root, "group_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(GroupMember {
        group_id,
        member_id,
        created_at,
    })
}

/// Encode a GroupMember to canonical CSIL CBOR bytes.
pub fn encode_group_member(csil_v: &GroupMember) -> Vec<u8> {
    cbor_encode(&csil_enc_group_member(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a GroupMember.
pub fn decode_group_member(csil_data: &[u8]) -> Result<GroupMember, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_group_member(&csil_root)
}

/// Build the canonical CBOR value tree for a Project.
fn csil_enc_project(csil_v: &Project) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(10);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    if let Some(csil_inner) = &csil_v.status {
        csil_entries.push((cbor_text("status"), csil_enc_project_status(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.category {
        csil_entries.push((cbor_text("category"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.visibility {
        csil_entries.push((cbor_text("visibility"), csil_enc_access_level(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.created_by_member_id {
        csil_entries.push((cbor_text("created_by_member_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Project from a decoded CBOR value tree.
fn csil_dec_project(csil_root: &CsilCborValue) -> Result<Project, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let category = match cbor_map_get(csil_root, "category") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let status = match cbor_map_get(csil_root, "status") {
        Some(csil_field) => {
            let csil_decode = csil_dec_project_status;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let visibility = match cbor_map_get(csil_root, "visibility") {
        Some(csil_field) => {
            let csil_decode = csil_dec_access_level;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_by_member_id = match cbor_map_get(csil_root, "created_by_member_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Project {
        project_id,
        house_id,
        name,
        description,
        category,
        status,
        visibility,
        created_by_member_id,
        created_at,
        updated_at,
    })
}

/// Encode a Project to canonical CSIL CBOR bytes.
pub fn encode_project(csil_v: &Project) -> Vec<u8> {
    cbor_encode(&csil_enc_project(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Project.
pub fn decode_project(csil_data: &[u8]) -> Result<Project, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectTask.
fn csil_enc_project_task(csil_v: &ProjectTask) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("position"), cbor_int(csil_v.position)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectTask from a decoded CBOR value tree.
fn csil_dec_project_task(csil_root: &CsilCborValue) -> Result<ProjectTask, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let position = {
        let csil_field = cbor_require(csil_root, "position")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectTask {
        project_id,
        task_id,
        position,
        created_at,
    })
}

/// Encode a ProjectTask to canonical CSIL CBOR bytes.
pub fn encode_project_task(csil_v: &ProjectTask) -> Vec<u8> {
    cbor_encode(&csil_enc_project_task(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectTask.
pub fn decode_project_task(csil_data: &[u8]) -> Result<ProjectTask, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_task(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectMember.
fn csil_enc_project_member(csil_v: &ProjectMember) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectMember from a decoded CBOR value tree.
fn csil_dec_project_member(csil_root: &CsilCborValue) -> Result<ProjectMember, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectMember {
        project_id,
        member_id,
        created_at,
    })
}

/// Encode a ProjectMember to canonical CSIL CBOR bytes.
pub fn encode_project_member(csil_v: &ProjectMember) -> Vec<u8> {
    cbor_encode(&csil_enc_project_member(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectMember.
pub fn decode_project_member(csil_data: &[u8]) -> Result<ProjectMember, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_member(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectOwner.
fn csil_enc_project_owner(csil_v: &ProjectOwner) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectOwner from a decoded CBOR value tree.
fn csil_dec_project_owner(csil_root: &CsilCborValue) -> Result<ProjectOwner, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectOwner {
        project_id,
        member_id,
        created_at,
    })
}

/// Encode a ProjectOwner to canonical CSIL CBOR bytes.
pub fn encode_project_owner(csil_v: &ProjectOwner) -> Vec<u8> {
    cbor_encode(&csil_enc_project_owner(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectOwner.
pub fn decode_project_owner(csil_data: &[u8]) -> Result<ProjectOwner, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_owner(&csil_root)
}

/// Build the canonical CBOR value tree for a Milestone.
fn csil_enc_milestone(csil_v: &Milestone) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(8);
    csil_entries.push((cbor_text("label"), cbor_text(&csil_v.label)));
    csil_entries.push((cbor_text("state"), csil_enc_milestone_state(&csil_v.state)));
    csil_entries.push((cbor_text("position"), cbor_int(csil_v.position)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    csil_entries.push((cbor_text("when_label"), cbor_text(&csil_v.when_label)));
    csil_entries.push((cbor_text("milestone_id"), cbor_text(&csil_v.milestone_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Milestone from a decoded CBOR value tree.
fn csil_dec_milestone(csil_root: &CsilCborValue) -> Result<Milestone, CsilCborError> {
    let milestone_id = {
        let csil_field = cbor_require(csil_root, "milestone_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let label = {
        let csil_field = cbor_require(csil_root, "label")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let when_label = {
        let csil_field = cbor_require(csil_root, "when_label")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let state = {
        let csil_field = cbor_require(csil_root, "state")?;
        let csil_decode = csil_dec_milestone_state;
        csil_decode(csil_field)?
    };
    let position = {
        let csil_field = cbor_require(csil_root, "position")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Milestone {
        milestone_id,
        project_id,
        label,
        when_label,
        state,
        position,
        created_at,
        updated_at,
    })
}

/// Encode a Milestone to canonical CSIL CBOR bytes.
pub fn encode_milestone(csil_v: &Milestone) -> Vec<u8> {
    cbor_encode(&csil_enc_milestone(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Milestone.
pub fn decode_milestone(csil_data: &[u8]) -> Result<Milestone, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_milestone(&csil_root)
}

/// Build the canonical CBOR value tree for a Event.
fn csil_enc_event(csil_v: &Event) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(17);
    csil_entries.push((cbor_text("title"), cbor_text(&csil_v.title)));
    if let Some(csil_inner) = &csil_v.all_day {
        csil_entries.push((cbor_text("all_day"), cbor_bool(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.ends_at {
        csil_entries.push((cbor_text("ends_at"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("event_id"), cbor_text(&csil_v.event_id)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.location {
        csil_entries.push((cbor_text("location"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.starts_at {
        csil_entries.push((cbor_text("starts_at"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("owner_member_id"), cbor_text(&csil_v.owner_member_id)));
    if let Some(csil_inner) = &csil_v.recurrence_freq {
        csil_entries.push((cbor_text("recurrence_freq"), csil_enc_recurrence_freq(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.next_recurrence_at {
        csil_entries.push((cbor_text("next_recurrence_at"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_interval {
        csil_entries.push((cbor_text("recurrence_interval"), cbor_int(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_by_setpos {
        csil_entries.push((cbor_text("recurrence_by_setpos"), cbor_int(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_by_weekday {
        csil_entries.push((cbor_text("recurrence_by_weekday"), cbor_enc_array(csil_inner, |csil_elem| cbor_int(*csil_elem))));
    }
    if let Some(csil_inner) = &csil_v.recurrence_root_event_id {
        csil_entries.push((cbor_text("recurrence_root_event_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Event from a decoded CBOR value tree.
fn csil_dec_event(csil_root: &CsilCborValue) -> Result<Event, CsilCborError> {
    let event_id = {
        let csil_field = cbor_require(csil_root, "event_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let owner_member_id = {
        let csil_field = cbor_require(csil_root, "owner_member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let title = {
        let csil_field = cbor_require(csil_root, "title")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let location = match cbor_map_get(csil_root, "location") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let starts_at = match cbor_map_get(csil_root, "starts_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let ends_at = match cbor_map_get(csil_root, "ends_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let all_day = match cbor_map_get(csil_root, "all_day") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_freq = match cbor_map_get(csil_root, "recurrence_freq") {
        Some(csil_field) => {
            let csil_decode = csil_dec_recurrence_freq;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_interval = match cbor_map_get(csil_root, "recurrence_interval") {
        Some(csil_field) => {
            let csil_decode = cbor_as_i64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_by_weekday = match cbor_map_get(csil_root, "recurrence_by_weekday") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_i64);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_by_setpos = match cbor_map_get(csil_root, "recurrence_by_setpos") {
        Some(csil_field) => {
            let csil_decode = cbor_as_i64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let next_recurrence_at = match cbor_map_get(csil_root, "next_recurrence_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_root_event_id = match cbor_map_get(csil_root, "recurrence_root_event_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Event {
        event_id,
        house_id,
        owner_member_id,
        title,
        description,
        location,
        starts_at,
        ends_at,
        all_day,
        recurrence_freq,
        recurrence_interval,
        recurrence_by_weekday,
        recurrence_by_setpos,
        next_recurrence_at,
        recurrence_root_event_id,
        created_at,
        updated_at,
    })
}

/// Encode a Event to canonical CSIL CBOR bytes.
pub fn encode_event(csil_v: &Event) -> Vec<u8> {
    cbor_encode(&csil_enc_event(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Event.
pub fn decode_event(csil_data: &[u8]) -> Result<Event, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_event(&csil_root)
}

/// Build the canonical CBOR value tree for a Task.
fn csil_enc_task(csil_v: &Task) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(22);
    if let Some(csil_inner) = &csil_v.tag {
        csil_entries.push((cbor_text("tag"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("title"), cbor_text(&csil_v.title)));
    if let Some(csil_inner) = &csil_v.due_at {
        csil_entries.push((cbor_text("due_at"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.status {
        csil_entries.push((cbor_text("status"), csil_enc_task_status(csil_inner)));
    }
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.assignees {
        csil_entries.push((cbor_text("assignees"), cbor_enc_array(csil_inner, |csil_elem| cbor_text(csil_elem))));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.deleted_at {
        csil_entries.push((cbor_text("deleted_at"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    if let Some(csil_inner) = &csil_v.visibility {
        csil_entries.push((cbor_text("visibility"), csil_enc_access_level(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.parent_task_id {
        csil_entries.push((cbor_text("parent_task_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("owner_member_id"), cbor_text(&csil_v.owner_member_id)));
    if let Some(csil_inner) = &csil_v.recurrence_freq {
        csil_entries.push((cbor_text("recurrence_freq"), csil_enc_recurrence_freq(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.estimate_minutes {
        csil_entries.push((cbor_text("estimate_minutes"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.next_recurrence_at {
        csil_entries.push((cbor_text("next_recurrence_at"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_interval {
        csil_entries.push((cbor_text("recurrence_interval"), cbor_int(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.assigned_to_skill_id {
        csil_entries.push((cbor_text("assigned_to_skill_id"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_by_setpos {
        csil_entries.push((cbor_text("recurrence_by_setpos"), cbor_int(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.recurrence_by_weekday {
        csil_entries.push((cbor_text("recurrence_by_weekday"), cbor_enc_array(csil_inner, |csil_elem| cbor_int(*csil_elem))));
    }
    if let Some(csil_inner) = &csil_v.recurrence_root_task_id {
        csil_entries.push((cbor_text("recurrence_root_task_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Task from a decoded CBOR value tree.
fn csil_dec_task(csil_root: &CsilCborValue) -> Result<Task, CsilCborError> {
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let owner_member_id = {
        let csil_field = cbor_require(csil_root, "owner_member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let assignees = match cbor_map_get(csil_root, "assignees") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let assigned_to_skill_id = match cbor_map_get(csil_root, "assigned_to_skill_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let parent_task_id = match cbor_map_get(csil_root, "parent_task_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let visibility = match cbor_map_get(csil_root, "visibility") {
        Some(csil_field) => {
            let csil_decode = csil_dec_access_level;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let title = {
        let csil_field = cbor_require(csil_root, "title")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let status = match cbor_map_get(csil_root, "status") {
        Some(csil_field) => {
            let csil_decode = csil_dec_task_status;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let due_at = match cbor_map_get(csil_root, "due_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let tag = match cbor_map_get(csil_root, "tag") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let estimate_minutes = match cbor_map_get(csil_root, "estimate_minutes") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_freq = match cbor_map_get(csil_root, "recurrence_freq") {
        Some(csil_field) => {
            let csil_decode = csil_dec_recurrence_freq;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_interval = match cbor_map_get(csil_root, "recurrence_interval") {
        Some(csil_field) => {
            let csil_decode = cbor_as_i64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_by_weekday = match cbor_map_get(csil_root, "recurrence_by_weekday") {
        Some(csil_field) => {
            let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_i64);
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_by_setpos = match cbor_map_get(csil_root, "recurrence_by_setpos") {
        Some(csil_field) => {
            let csil_decode = cbor_as_i64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let next_recurrence_at = match cbor_map_get(csil_root, "next_recurrence_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let recurrence_root_task_id = match cbor_map_get(csil_root, "recurrence_root_task_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let deleted_at = match cbor_map_get(csil_root, "deleted_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Task {
        task_id,
        house_id,
        owner_member_id,
        assignees,
        assigned_to_skill_id,
        parent_task_id,
        visibility,
        title,
        description,
        status,
        due_at,
        tag,
        estimate_minutes,
        recurrence_freq,
        recurrence_interval,
        recurrence_by_weekday,
        recurrence_by_setpos,
        next_recurrence_at,
        recurrence_root_task_id,
        deleted_at,
        created_at,
        updated_at,
    })
}

/// Encode a Task to canonical CSIL CBOR bytes.
pub fn encode_task(csil_v: &Task) -> Vec<u8> {
    cbor_encode(&csil_enc_task(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Task.
pub fn decode_task(csil_data: &[u8]) -> Result<Task, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_task(&csil_root)
}

/// Build the canonical CBOR value tree for a Comment.
fn csil_enc_comment(csil_v: &Comment) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(8);
    csil_entries.push((cbor_text("body"), cbor_text(&csil_v.body)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("target_id"), cbor_text(&csil_v.target_id)));
    csil_entries.push((cbor_text("comment_id"), cbor_text(&csil_v.comment_id)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    csil_entries.push((cbor_text("updated_at"), cbor_text(&csil_v.updated_at)));
    csil_entries.push((cbor_text("target_type"), csil_enc_target_type(&csil_v.target_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Comment from a decoded CBOR value tree.
fn csil_dec_comment(csil_root: &CsilCborValue) -> Result<Comment, CsilCborError> {
    let comment_id = {
        let csil_field = cbor_require(csil_root, "comment_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let target_type = {
        let csil_field = cbor_require(csil_root, "target_type")?;
        let csil_decode = csil_dec_target_type;
        csil_decode(csil_field)?
    };
    let target_id = {
        let csil_field = cbor_require(csil_root, "target_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let body = {
        let csil_field = cbor_require(csil_root, "body")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let updated_at = {
        let csil_field = cbor_require(csil_root, "updated_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Comment {
        comment_id,
        house_id,
        member_id,
        target_type,
        target_id,
        body,
        created_at,
        updated_at,
    })
}

/// Encode a Comment to canonical CSIL CBOR bytes.
pub fn encode_comment(csil_v: &Comment) -> Vec<u8> {
    cbor_encode(&csil_enc_comment(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Comment.
pub fn decode_comment(csil_data: &[u8]) -> Result<Comment, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_comment(&csil_root)
}

/// Build the canonical CBOR value tree for a Share.
fn csil_enc_share(csil_v: &Share) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(10);
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("share_id"), cbor_text(&csil_v.share_id)));
    csil_entries.push((cbor_text("shared_by"), cbor_text(&csil_v.shared_by)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.expires_at {
        csil_entries.push((cbor_text("expires_at"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("resource_id"), cbor_text(&csil_v.resource_id)));
    if let Some(csil_inner) = &csil_v.access_level {
        csil_entries.push((cbor_text("access_level"), csil_enc_access_level(csil_inner)));
    }
    csil_entries.push((cbor_text("resource_type"), csil_enc_resource_type(&csil_v.resource_type)));
    csil_entries.push((cbor_text("linkkeys_domain"), cbor_text(&csil_v.linkkeys_domain)));
    csil_entries.push((cbor_text("linkkeys_user_id"), cbor_text(&csil_v.linkkeys_user_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Share from a decoded CBOR value tree.
fn csil_dec_share(csil_root: &CsilCborValue) -> Result<Share, CsilCborError> {
    let share_id = {
        let csil_field = cbor_require(csil_root, "share_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let shared_by = {
        let csil_field = cbor_require(csil_root, "shared_by")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let linkkeys_domain = {
        let csil_field = cbor_require(csil_root, "linkkeys_domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let linkkeys_user_id = {
        let csil_field = cbor_require(csil_root, "linkkeys_user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let resource_type = {
        let csil_field = cbor_require(csil_root, "resource_type")?;
        let csil_decode = csil_dec_resource_type;
        csil_decode(csil_field)?
    };
    let resource_id = {
        let csil_field = cbor_require(csil_root, "resource_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let access_level = match cbor_map_get(csil_root, "access_level") {
        Some(csil_field) => {
            let csil_decode = csil_dec_access_level;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let expires_at = match cbor_map_get(csil_root, "expires_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(Share {
        share_id,
        house_id,
        shared_by,
        linkkeys_domain,
        linkkeys_user_id,
        resource_type,
        resource_id,
        access_level,
        created_at,
        expires_at,
    })
}

/// Encode a Share to canonical CSIL CBOR bytes.
pub fn encode_share(csil_v: &Share) -> Vec<u8> {
    cbor_encode(&csil_enc_share(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Share.
pub fn decode_share(csil_data: &[u8]) -> Result<Share, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_share(&csil_root)
}

/// Build the canonical CBOR value tree for a HouseSummary.
fn csil_enc_house_summary(csil_v: &HouseSummary) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("name"), cbor_text(&csil_v.name)));
    csil_entries.push((cbor_text("roles"), cbor_enc_array(&csil_v.roles, |csil_elem| cbor_text(csil_elem))));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a HouseSummary from a decoded CBOR value tree.
fn csil_dec_house_summary(csil_root: &CsilCborValue) -> Result<HouseSummary, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let name = {
        let csil_field = cbor_require(csil_root, "name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let roles = {
        let csil_field = cbor_require(csil_root, "roles")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
        csil_decode(csil_field)?
    };
    Ok(HouseSummary {
        house_id,
        name,
        member_id,
        roles,
    })
}

/// Encode a HouseSummary to canonical CSIL CBOR bytes.
pub fn encode_house_summary(csil_v: &HouseSummary) -> Vec<u8> {
    cbor_encode(&csil_enc_house_summary(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a HouseSummary.
pub fn decode_house_summary(csil_data: &[u8]) -> Result<HouseSummary, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_house_summary(&csil_root)
}

/// Build the canonical CBOR value tree for a HouseRoles.
fn csil_enc_house_roles(csil_v: &HouseRoles) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("house"), cbor_text(&csil_v.house)));
    csil_entries.push((cbor_text("roles"), cbor_enc_array(&csil_v.roles, |csil_elem| cbor_text(csil_elem))));
    csil_entries.push((cbor_text("member"), cbor_text(&csil_v.member)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a HouseRoles from a decoded CBOR value tree.
fn csil_dec_house_roles(csil_root: &CsilCborValue) -> Result<HouseRoles, CsilCborError> {
    let house = {
        let csil_field = cbor_require(csil_root, "house")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member = {
        let csil_field = cbor_require(csil_root, "member")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let roles = {
        let csil_field = cbor_require(csil_root, "roles")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
        csil_decode(csil_field)?
    };
    Ok(HouseRoles {
        house,
        member,
        roles,
    })
}

/// Encode a HouseRoles to canonical CSIL CBOR bytes.
pub fn encode_house_roles(csil_v: &HouseRoles) -> Vec<u8> {
    cbor_encode(&csil_enc_house_roles(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a HouseRoles.
pub fn decode_house_roles(csil_data: &[u8]) -> Result<HouseRoles, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_house_roles(&csil_root)
}

/// Build the canonical CBOR value tree for a Identity.
fn csil_enc_identity(csil_v: &Identity) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(6);
    csil_entries.push((cbor_text("exp"), cbor_int(csil_v.exp)));
    csil_entries.push((cbor_text("iat"), cbor_int(csil_v.iat)));
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    csil_entries.push((cbor_text("houses"), cbor_enc_array(&csil_v.houses, |csil_elem| csil_enc_house_roles(csil_elem))));
    csil_entries.push((cbor_text("user_id"), cbor_text(&csil_v.user_id)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Identity from a decoded CBOR value tree.
fn csil_dec_identity(csil_root: &CsilCborValue) -> Result<Identity, CsilCborError> {
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let user_id = {
        let csil_field = cbor_require(csil_root, "user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let houses = {
        let csil_field = cbor_require(csil_root, "houses")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_house_roles);
        csil_decode(csil_field)?
    };
    let iat = {
        let csil_field = cbor_require(csil_root, "iat")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    let exp = {
        let csil_field = cbor_require(csil_root, "exp")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    Ok(Identity {
        domain,
        user_id,
        display_name,
        houses,
        iat,
        exp,
    })
}

/// Encode a Identity to canonical CSIL CBOR bytes.
pub fn encode_identity(csil_v: &Identity) -> Vec<u8> {
    cbor_encode(&csil_enc_identity(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Identity.
pub fn decode_identity(csil_data: &[u8]) -> Result<Identity, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_identity(&csil_root)
}

/// Build the canonical CBOR value tree for a LoginRequest.
fn csil_enc_login_request(csil_v: &LoginRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("signed_assertion"), cbor_text(&csil_v.signed_assertion)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a LoginRequest from a decoded CBOR value tree.
fn csil_dec_login_request(csil_root: &CsilCborValue) -> Result<LoginRequest, CsilCborError> {
    let signed_assertion = {
        let csil_field = cbor_require(csil_root, "signed_assertion")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(LoginRequest {
        signed_assertion,
    })
}

/// Encode a LoginRequest to canonical CSIL CBOR bytes.
pub fn encode_login_request(csil_v: &LoginRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_login_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a LoginRequest.
pub fn decode_login_request(csil_data: &[u8]) -> Result<LoginRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_login_request(&csil_root)
}

/// Build the canonical CBOR value tree for a CompleteRequest.
fn csil_enc_complete_request(csil_v: &CompleteRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("encrypted_token"), cbor_text(&csil_v.encrypted_token)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a CompleteRequest from a decoded CBOR value tree.
fn csil_dec_complete_request(csil_root: &CsilCborValue) -> Result<CompleteRequest, CsilCborError> {
    let encrypted_token = {
        let csil_field = cbor_require(csil_root, "encrypted_token")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(CompleteRequest {
        encrypted_token,
    })
}

/// Encode a CompleteRequest to canonical CSIL CBOR bytes.
pub fn encode_complete_request(csil_v: &CompleteRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_complete_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a CompleteRequest.
pub fn decode_complete_request(csil_data: &[u8]) -> Result<CompleteRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_complete_request(&csil_root)
}

/// Build the canonical CBOR value tree for a LoginResponse.
fn csil_enc_login_response(csil_v: &LoginResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(5);
    csil_entries.push((cbor_text("token"), cbor_text(&csil_v.token)));
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    csil_entries.push((cbor_text("user_id"), cbor_text(&csil_v.user_id)));
    csil_entries.push((cbor_text("expires_at"), cbor_text(&csil_v.expires_at)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a LoginResponse from a decoded CBOR value tree.
fn csil_dec_login_response(csil_root: &CsilCborValue) -> Result<LoginResponse, CsilCborError> {
    let token = {
        let csil_field = cbor_require(csil_root, "token")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let user_id = {
        let csil_field = cbor_require(csil_root, "user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let expires_at = {
        let csil_field = cbor_require(csil_root, "expires_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(LoginResponse {
        token,
        domain,
        user_id,
        display_name,
        expires_at,
    })
}

/// Encode a LoginResponse to canonical CSIL CBOR bytes.
pub fn encode_login_response(csil_v: &LoginResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_login_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a LoginResponse.
pub fn decode_login_response(csil_data: &[u8]) -> Result<LoginResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_login_response(&csil_root)
}

/// Build the canonical CBOR value tree for a DevUserEntry.
fn csil_enc_dev_user_entry(csil_v: &DevUserEntry) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(7);
    csil_entries.push((cbor_text("roles"), cbor_enc_array(&csil_v.roles, |csil_elem| cbor_text(csil_elem))));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("house_name"), cbor_text(&csil_v.house_name)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.linkkeys_domain {
        csil_entries.push((cbor_text("linkkeys_domain"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.linkkeys_user_id {
        csil_entries.push((cbor_text("linkkeys_user_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DevUserEntry from a decoded CBOR value tree.
fn csil_dec_dev_user_entry(csil_root: &CsilCborValue) -> Result<DevUserEntry, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_name = {
        let csil_field = cbor_require(csil_root, "house_name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let linkkeys_domain = match cbor_map_get(csil_root, "linkkeys_domain") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let linkkeys_user_id = match cbor_map_get(csil_root, "linkkeys_user_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let roles = {
        let csil_field = cbor_require(csil_root, "roles")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, cbor_as_text);
        csil_decode(csil_field)?
    };
    Ok(DevUserEntry {
        member_id,
        house_id,
        house_name,
        display_name,
        linkkeys_domain,
        linkkeys_user_id,
        roles,
    })
}

/// Encode a DevUserEntry to canonical CSIL CBOR bytes.
pub fn encode_dev_user_entry(csil_v: &DevUserEntry) -> Vec<u8> {
    cbor_encode(&csil_enc_dev_user_entry(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DevUserEntry.
pub fn decode_dev_user_entry(csil_data: &[u8]) -> Result<DevUserEntry, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dev_user_entry(&csil_root)
}

/// Build the canonical CBOR value tree for a DevUsersResponse.
fn csil_enc_dev_users_response(csil_v: &DevUsersResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("users"), cbor_enc_array(&csil_v.users, |csil_elem| csil_enc_dev_user_entry(csil_elem))));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DevUsersResponse from a decoded CBOR value tree.
fn csil_dec_dev_users_response(csil_root: &CsilCborValue) -> Result<DevUsersResponse, CsilCborError> {
    let users = {
        let csil_field = cbor_require(csil_root, "users")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_dev_user_entry);
        csil_decode(csil_field)?
    };
    Ok(DevUsersResponse {
        users,
    })
}

/// Encode a DevUsersResponse to canonical CSIL CBOR bytes.
pub fn encode_dev_users_response(csil_v: &DevUsersResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_dev_users_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DevUsersResponse.
pub fn decode_dev_users_response(csil_data: &[u8]) -> Result<DevUsersResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dev_users_response(&csil_root)
}

/// Build the canonical CBOR value tree for a DevLoginRequest.
fn csil_enc_dev_login_request(csil_v: &DevLoginRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DevLoginRequest from a decoded CBOR value tree.
fn csil_dec_dev_login_request(csil_root: &CsilCborValue) -> Result<DevLoginRequest, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(DevLoginRequest {
        member_id,
    })
}

/// Encode a DevLoginRequest to canonical CSIL CBOR bytes.
pub fn encode_dev_login_request(csil_v: &DevLoginRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_dev_login_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DevLoginRequest.
pub fn decode_dev_login_request(csil_data: &[u8]) -> Result<DevLoginRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dev_login_request(&csil_root)
}

/// Build the canonical CBOR value tree for a MeResponse.
fn csil_enc_me_response(csil_v: &MeResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(5);
    csil_entries.push((cbor_text("domain"), cbor_text(&csil_v.domain)));
    csil_entries.push((cbor_text("houses"), cbor_enc_array(&csil_v.houses, |csil_elem| csil_enc_house_summary(csil_elem))));
    csil_entries.push((cbor_text("user_id"), cbor_text(&csil_v.user_id)));
    csil_entries.push((cbor_text("expires_at"), cbor_text(&csil_v.expires_at)));
    if let Some(csil_inner) = &csil_v.display_name {
        csil_entries.push((cbor_text("display_name"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MeResponse from a decoded CBOR value tree.
fn csil_dec_me_response(csil_root: &CsilCborValue) -> Result<MeResponse, CsilCborError> {
    let domain = {
        let csil_field = cbor_require(csil_root, "domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let user_id = {
        let csil_field = cbor_require(csil_root, "user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let display_name = match cbor_map_get(csil_root, "display_name") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let expires_at = {
        let csil_field = cbor_require(csil_root, "expires_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let houses = {
        let csil_field = cbor_require(csil_root, "houses")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_house_summary);
        csil_decode(csil_field)?
    };
    Ok(MeResponse {
        domain,
        user_id,
        display_name,
        expires_at,
        houses,
    })
}

/// Encode a MeResponse to canonical CSIL CBOR bytes.
pub fn encode_me_response(csil_v: &MeResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_me_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MeResponse.
pub fn decode_me_response(csil_data: &[u8]) -> Result<MeResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_me_response(&csil_root)
}

/// Build the canonical CBOR value tree for a EmptyRequest.
fn csil_enc_empty_request(csil_v: &EmptyRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(0);
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a EmptyRequest from a decoded CBOR value tree.
fn csil_dec_empty_request(csil_root: &CsilCborValue) -> Result<EmptyRequest, CsilCborError> {
    Ok(EmptyRequest {
    })
}

/// Encode a EmptyRequest to canonical CSIL CBOR bytes.
pub fn encode_empty_request(csil_v: &EmptyRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_empty_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a EmptyRequest.
pub fn decode_empty_request(csil_data: &[u8]) -> Result<EmptyRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_empty_request(&csil_root)
}

/// Build the canonical CBOR value tree for a EmptyResponse.
fn csil_enc_empty_response(csil_v: &EmptyResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(0);
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a EmptyResponse from a decoded CBOR value tree.
fn csil_dec_empty_response(csil_root: &CsilCborValue) -> Result<EmptyResponse, CsilCborError> {
    Ok(EmptyResponse {
    })
}

/// Encode a EmptyResponse to canonical CSIL CBOR bytes.
pub fn encode_empty_response(csil_v: &EmptyResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_empty_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a EmptyResponse.
pub fn decode_empty_response(csil_data: &[u8]) -> Result<EmptyResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_empty_response(&csil_root)
}

/// Build the canonical CBOR value tree for a BoolResponse.
fn csil_enc_bool_response(csil_v: &BoolResponse) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("value"), cbor_bool(csil_v.value)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a BoolResponse from a decoded CBOR value tree.
fn csil_dec_bool_response(csil_root: &CsilCborValue) -> Result<BoolResponse, CsilCborError> {
    let value = {
        let csil_field = cbor_require(csil_root, "value")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    Ok(BoolResponse {
        value,
    })
}

/// Encode a BoolResponse to canonical CSIL CBOR bytes.
pub fn encode_bool_response(csil_v: &BoolResponse) -> Vec<u8> {
    cbor_encode(&csil_enc_bool_response(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a BoolResponse.
pub fn decode_bool_response(csil_data: &[u8]) -> Result<BoolResponse, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_bool_response(&csil_root)
}

/// Build the canonical CBOR value tree for a HouseListRequest.
fn csil_enc_house_list_request(csil_v: &HouseListRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.offset {
        csil_entries.push((cbor_text("offset"), cbor_uint(*csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a HouseListRequest from a decoded CBOR value tree.
fn csil_dec_house_list_request(csil_root: &CsilCborValue) -> Result<HouseListRequest, CsilCborError> {
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let offset = match cbor_map_get(csil_root, "offset") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(HouseListRequest {
        limit,
        offset,
    })
}

/// Encode a HouseListRequest to canonical CSIL CBOR bytes.
pub fn encode_house_list_request(csil_v: &HouseListRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_house_list_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a HouseListRequest.
pub fn decode_house_list_request(csil_data: &[u8]) -> Result<HouseListRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_house_list_request(&csil_root)
}

/// Build the canonical CBOR value tree for a HouseScopedListRequest.
fn csil_enc_house_scoped_list_request(csil_v: &HouseScopedListRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.offset {
        csil_entries.push((cbor_text("offset"), cbor_uint(*csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a HouseScopedListRequest from a decoded CBOR value tree.
fn csil_dec_house_scoped_list_request(csil_root: &CsilCborValue) -> Result<HouseScopedListRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let offset = match cbor_map_get(csil_root, "offset") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(HouseScopedListRequest {
        house_id,
        limit,
        offset,
    })
}

/// Encode a HouseScopedListRequest to canonical CSIL CBOR bytes.
pub fn encode_house_scoped_list_request(csil_v: &HouseScopedListRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_house_scoped_list_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a HouseScopedListRequest.
pub fn decode_house_scoped_list_request(csil_data: &[u8]) -> Result<HouseScopedListRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_house_scoped_list_request(&csil_root)
}

/// Build the canonical CBOR value tree for a TaskList.
fn csil_enc_task_list(csil_v: &TaskList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("tasks"), cbor_enc_array(&csil_v.tasks, |csil_elem| csil_enc_task(csil_elem))));
    csil_entries.push((cbor_text("hidden_count"), cbor_uint(csil_v.hidden_count)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a TaskList from a decoded CBOR value tree.
fn csil_dec_task_list(csil_root: &CsilCborValue) -> Result<TaskList, CsilCborError> {
    let tasks = {
        let csil_field = cbor_require(csil_root, "tasks")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_task);
        csil_decode(csil_field)?
    };
    let hidden_count = {
        let csil_field = cbor_require(csil_root, "hidden_count")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    Ok(TaskList {
        tasks,
        hidden_count,
    })
}

/// Encode a TaskList to canonical CSIL CBOR bytes.
pub fn encode_task_list(csil_v: &TaskList) -> Vec<u8> {
    cbor_encode(&csil_enc_task_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a TaskList.
pub fn decode_task_list(csil_data: &[u8]) -> Result<TaskList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_task_list(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectList.
fn csil_enc_project_list(csil_v: &ProjectList) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("projects"), cbor_enc_array(&csil_v.projects, |csil_elem| csil_enc_project(csil_elem))));
    csil_entries.push((cbor_text("hidden_count"), cbor_uint(csil_v.hidden_count)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectList from a decoded CBOR value tree.
fn csil_dec_project_list(csil_root: &CsilCborValue) -> Result<ProjectList, CsilCborError> {
    let projects = {
        let csil_field = cbor_require(csil_root, "projects")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_project);
        csil_decode(csil_field)?
    };
    let hidden_count = {
        let csil_field = cbor_require(csil_root, "hidden_count")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    Ok(ProjectList {
        projects,
        hidden_count,
    })
}

/// Encode a ProjectList to canonical CSIL CBOR bytes.
pub fn encode_project_list(csil_v: &ProjectList) -> Vec<u8> {
    cbor_encode(&csil_enc_project_list(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectList.
pub fn decode_project_list(csil_data: &[u8]) -> Result<ProjectList, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_list(&csil_root)
}

/// Build the canonical CBOR value tree for a MemberScopedListRequest.
fn csil_enc_member_scoped_list_request(csil_v: &MemberScopedListRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.offset {
        csil_entries.push((cbor_text("offset"), cbor_uint(*csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MemberScopedListRequest from a decoded CBOR value tree.
fn csil_dec_member_scoped_list_request(csil_root: &CsilCborValue) -> Result<MemberScopedListRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let offset = match cbor_map_get(csil_root, "offset") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(MemberScopedListRequest {
        house_id,
        member_id,
        limit,
        offset,
    })
}

/// Encode a MemberScopedListRequest to canonical CSIL CBOR bytes.
pub fn encode_member_scoped_list_request(csil_v: &MemberScopedListRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_member_scoped_list_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MemberScopedListRequest.
pub fn decode_member_scoped_list_request(csil_data: &[u8]) -> Result<MemberScopedListRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member_scoped_list_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectScopedListRequest.
fn csil_enc_project_scoped_list_request(csil_v: &ProjectScopedListRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.offset {
        csil_entries.push((cbor_text("offset"), cbor_uint(*csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectScopedListRequest from a decoded CBOR value tree.
fn csil_dec_project_scoped_list_request(csil_root: &CsilCborValue) -> Result<ProjectScopedListRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let offset = match cbor_map_get(csil_root, "offset") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(ProjectScopedListRequest {
        house_id,
        project_id,
        limit,
        offset,
    })
}

/// Encode a ProjectScopedListRequest to canonical CSIL CBOR bytes.
pub fn encode_project_scoped_list_request(csil_v: &ProjectScopedListRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_project_scoped_list_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectScopedListRequest.
pub fn decode_project_scoped_list_request(csil_data: &[u8]) -> Result<ProjectScopedListRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_scoped_list_request(&csil_root)
}

/// Build the canonical CBOR value tree for a CommentListRequest.
fn csil_enc_comment_list_request(csil_v: &CommentListRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.offset {
        csil_entries.push((cbor_text("offset"), cbor_uint(*csil_inner)));
    }
    csil_entries.push((cbor_text("target_id"), cbor_text(&csil_v.target_id)));
    csil_entries.push((cbor_text("target_type"), csil_enc_target_type(&csil_v.target_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a CommentListRequest from a decoded CBOR value tree.
fn csil_dec_comment_list_request(csil_root: &CsilCborValue) -> Result<CommentListRequest, CsilCborError> {
    let target_type = {
        let csil_field = cbor_require(csil_root, "target_type")?;
        let csil_decode = csil_dec_target_type;
        csil_decode(csil_field)?
    };
    let target_id = {
        let csil_field = cbor_require(csil_root, "target_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let offset = match cbor_map_get(csil_root, "offset") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(CommentListRequest {
        target_type,
        target_id,
        limit,
        offset,
    })
}

/// Encode a CommentListRequest to canonical CSIL CBOR bytes.
pub fn encode_comment_list_request(csil_v: &CommentListRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_comment_list_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a CommentListRequest.
pub fn decode_comment_list_request(csil_data: &[u8]) -> Result<CommentListRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_comment_list_request(&csil_root)
}

/// Build the canonical CBOR value tree for a Notification.
fn csil_enc_notification(csil_v: &Notification) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(13);
    csil_entries.push((cbor_text("body"), cbor_text(&csil_v.body)));
    csil_entries.push((cbor_text("kind"), cbor_text(&csil_v.kind)));
    csil_entries.push((cbor_text("read"), cbor_bool(csil_v.read)));
    if let Some(csil_inner) = &csil_v.read_at {
        csil_entries.push((cbor_text("read_at"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    if let Some(csil_inner) = &csil_v.target_id {
        csil_entries.push((cbor_text("target_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("actor_name"), cbor_text(&csil_v.actor_name)));
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.target_type {
        csil_entries.push((cbor_text("target_type"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("target_title"), cbor_text(&csil_v.target_title)));
    if let Some(csil_inner) = &csil_v.actor_member_id {
        csil_entries.push((cbor_text("actor_member_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("notification_id"), cbor_text(&csil_v.notification_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Notification from a decoded CBOR value tree.
fn csil_dec_notification(csil_root: &CsilCborValue) -> Result<Notification, CsilCborError> {
    let notification_id = {
        let csil_field = cbor_require(csil_root, "notification_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let kind = {
        let csil_field = cbor_require(csil_root, "kind")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let actor_member_id = match cbor_map_get(csil_root, "actor_member_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let actor_name = {
        let csil_field = cbor_require(csil_root, "actor_name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let target_type = match cbor_map_get(csil_root, "target_type") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let target_id = match cbor_map_get(csil_root, "target_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let target_title = {
        let csil_field = cbor_require(csil_root, "target_title")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let body = {
        let csil_field = cbor_require(csil_root, "body")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let read = {
        let csil_field = cbor_require(csil_root, "read")?;
        let csil_decode = cbor_as_bool;
        csil_decode(csil_field)?
    };
    let read_at = match cbor_map_get(csil_root, "read_at") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(Notification {
        notification_id,
        house_id,
        member_id,
        kind,
        actor_member_id,
        actor_name,
        target_type,
        target_id,
        target_title,
        body,
        read,
        read_at,
        created_at,
    })
}

/// Encode a Notification to canonical CSIL CBOR bytes.
pub fn encode_notification(csil_v: &Notification) -> Vec<u8> {
    cbor_encode(&csil_enc_notification(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Notification.
pub fn decode_notification(csil_data: &[u8]) -> Result<Notification, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_notification(&csil_root)
}

/// Build the canonical CBOR value tree for a NotificationListRequest.
fn csil_enc_notification_list_request(csil_v: &NotificationListRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.offset {
        csil_entries.push((cbor_text("offset"), cbor_uint(*csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.unread_only {
        csil_entries.push((cbor_text("unread_only"), cbor_bool(*csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a NotificationListRequest from a decoded CBOR value tree.
fn csil_dec_notification_list_request(csil_root: &CsilCborValue) -> Result<NotificationListRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let unread_only = match cbor_map_get(csil_root, "unread_only") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let offset = match cbor_map_get(csil_root, "offset") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(NotificationListRequest {
        house_id,
        unread_only,
        limit,
        offset,
    })
}

/// Encode a NotificationListRequest to canonical CSIL CBOR bytes.
pub fn encode_notification_list_request(csil_v: &NotificationListRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_notification_list_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a NotificationListRequest.
pub fn decode_notification_list_request(csil_data: &[u8]) -> Result<NotificationListRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_notification_list_request(&csil_root)
}

/// Build the canonical CBOR value tree for a NotificationUnreadCount.
fn csil_enc_notification_unread_count(csil_v: &NotificationUnreadCount) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(1);
    csil_entries.push((cbor_text("count"), cbor_uint(csil_v.count)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a NotificationUnreadCount from a decoded CBOR value tree.
fn csil_dec_notification_unread_count(csil_root: &CsilCborValue) -> Result<NotificationUnreadCount, CsilCborError> {
    let count = {
        let csil_field = cbor_require(csil_root, "count")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    Ok(NotificationUnreadCount {
        count,
    })
}

/// Encode a NotificationUnreadCount to canonical CSIL CBOR bytes.
pub fn encode_notification_unread_count(csil_v: &NotificationUnreadCount) -> Vec<u8> {
    cbor_encode(&csil_enc_notification_unread_count(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a NotificationUnreadCount.
pub fn decode_notification_unread_count(csil_data: &[u8]) -> Result<NotificationUnreadCount, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_notification_unread_count(&csil_root)
}

/// Build the canonical CBOR value tree for a ShareAccessRequest.
fn csil_enc_share_access_request(csil_v: &ShareAccessRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("resource_id"), cbor_text(&csil_v.resource_id)));
    csil_entries.push((cbor_text("resource_type"), csil_enc_resource_type(&csil_v.resource_type)));
    csil_entries.push((cbor_text("linkkeys_domain"), cbor_text(&csil_v.linkkeys_domain)));
    csil_entries.push((cbor_text("linkkeys_user_id"), cbor_text(&csil_v.linkkeys_user_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ShareAccessRequest from a decoded CBOR value tree.
fn csil_dec_share_access_request(csil_root: &CsilCborValue) -> Result<ShareAccessRequest, CsilCborError> {
    let linkkeys_domain = {
        let csil_field = cbor_require(csil_root, "linkkeys_domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let linkkeys_user_id = {
        let csil_field = cbor_require(csil_root, "linkkeys_user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let resource_type = {
        let csil_field = cbor_require(csil_root, "resource_type")?;
        let csil_decode = csil_dec_resource_type;
        csil_decode(csil_field)?
    };
    let resource_id = {
        let csil_field = cbor_require(csil_root, "resource_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ShareAccessRequest {
        linkkeys_domain,
        linkkeys_user_id,
        resource_type,
        resource_id,
    })
}

/// Encode a ShareAccessRequest to canonical CSIL CBOR bytes.
pub fn encode_share_access_request(csil_v: &ShareAccessRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_share_access_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ShareAccessRequest.
pub fn decode_share_access_request(csil_data: &[u8]) -> Result<ShareAccessRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_share_access_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ResourceRef.
fn csil_enc_resource_ref(csil_v: &ResourceRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("resource_id"), cbor_text(&csil_v.resource_id)));
    csil_entries.push((cbor_text("resource_type"), csil_enc_resource_type(&csil_v.resource_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ResourceRef from a decoded CBOR value tree.
fn csil_dec_resource_ref(csil_root: &CsilCborValue) -> Result<ResourceRef, CsilCborError> {
    let resource_type = {
        let csil_field = cbor_require(csil_root, "resource_type")?;
        let csil_decode = csil_dec_resource_type;
        csil_decode(csil_field)?
    };
    let resource_id = {
        let csil_field = cbor_require(csil_root, "resource_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ResourceRef {
        resource_type,
        resource_id,
    })
}

/// Encode a ResourceRef to canonical CSIL CBOR bytes.
pub fn encode_resource_ref(csil_v: &ResourceRef) -> Vec<u8> {
    cbor_encode(&csil_enc_resource_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ResourceRef.
pub fn decode_resource_ref(csil_data: &[u8]) -> Result<ResourceRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_resource_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a MemberRoleRef.
fn csil_enc_member_role_ref(csil_v: &MemberRoleRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("role_id"), cbor_text(&csil_v.role_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MemberRoleRef from a decoded CBOR value tree.
fn csil_dec_member_role_ref(csil_root: &CsilCborValue) -> Result<MemberRoleRef, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let role_id = {
        let csil_field = cbor_require(csil_root, "role_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(MemberRoleRef {
        member_id,
        role_id,
    })
}

/// Encode a MemberRoleRef to canonical CSIL CBOR bytes.
pub fn encode_member_role_ref(csil_v: &MemberRoleRef) -> Vec<u8> {
    cbor_encode(&csil_enc_member_role_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MemberRoleRef.
pub fn decode_member_role_ref(csil_data: &[u8]) -> Result<MemberRoleRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member_role_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a MemberSkillRef.
fn csil_enc_member_skill_ref(csil_v: &MemberSkillRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("skill_id"), cbor_text(&csil_v.skill_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a MemberSkillRef from a decoded CBOR value tree.
fn csil_dec_member_skill_ref(csil_root: &CsilCborValue) -> Result<MemberSkillRef, CsilCborError> {
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let skill_id = {
        let csil_field = cbor_require(csil_root, "skill_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(MemberSkillRef {
        member_id,
        skill_id,
    })
}

/// Encode a MemberSkillRef to canonical CSIL CBOR bytes.
pub fn encode_member_skill_ref(csil_v: &MemberSkillRef) -> Vec<u8> {
    cbor_encode(&csil_enc_member_skill_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a MemberSkillRef.
pub fn decode_member_skill_ref(csil_data: &[u8]) -> Result<MemberSkillRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_member_skill_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a GroupSkillRef.
fn csil_enc_group_skill_ref(csil_v: &GroupSkillRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("group_id"), cbor_text(&csil_v.group_id)));
    csil_entries.push((cbor_text("skill_id"), cbor_text(&csil_v.skill_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a GroupSkillRef from a decoded CBOR value tree.
fn csil_dec_group_skill_ref(csil_root: &CsilCborValue) -> Result<GroupSkillRef, CsilCborError> {
    let group_id = {
        let csil_field = cbor_require(csil_root, "group_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let skill_id = {
        let csil_field = cbor_require(csil_root, "skill_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(GroupSkillRef {
        group_id,
        skill_id,
    })
}

/// Encode a GroupSkillRef to canonical CSIL CBOR bytes.
pub fn encode_group_skill_ref(csil_v: &GroupSkillRef) -> Vec<u8> {
    cbor_encode(&csil_enc_group_skill_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a GroupSkillRef.
pub fn decode_group_skill_ref(csil_data: &[u8]) -> Result<GroupSkillRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_group_skill_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a GroupMemberRef.
fn csil_enc_group_member_ref(csil_v: &GroupMemberRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("group_id"), cbor_text(&csil_v.group_id)));
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a GroupMemberRef from a decoded CBOR value tree.
fn csil_dec_group_member_ref(csil_root: &CsilCborValue) -> Result<GroupMemberRef, CsilCborError> {
    let group_id = {
        let csil_field = cbor_require(csil_root, "group_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(GroupMemberRef {
        group_id,
        member_id,
    })
}

/// Encode a GroupMemberRef to canonical CSIL CBOR bytes.
pub fn encode_group_member_ref(csil_v: &GroupMemberRef) -> Vec<u8> {
    cbor_encode(&csil_enc_group_member_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a GroupMemberRef.
pub fn decode_group_member_ref(csil_data: &[u8]) -> Result<GroupMemberRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_group_member_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectTaskRef.
fn csil_enc_project_task_ref(csil_v: &ProjectTaskRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectTaskRef from a decoded CBOR value tree.
fn csil_dec_project_task_ref(csil_root: &CsilCborValue) -> Result<ProjectTaskRef, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectTaskRef {
        project_id,
        task_id,
    })
}

/// Encode a ProjectTaskRef to canonical CSIL CBOR bytes.
pub fn encode_project_task_ref(csil_v: &ProjectTaskRef) -> Vec<u8> {
    cbor_encode(&csil_enc_project_task_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectTaskRef.
pub fn decode_project_task_ref(csil_data: &[u8]) -> Result<ProjectTaskRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_task_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectTaskOrderRequest.
fn csil_enc_project_task_order_request(csil_v: &ProjectTaskOrderRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("position"), cbor_int(csil_v.position)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectTaskOrderRequest from a decoded CBOR value tree.
fn csil_dec_project_task_order_request(csil_root: &CsilCborValue) -> Result<ProjectTaskOrderRequest, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let position = {
        let csil_field = cbor_require(csil_root, "position")?;
        let csil_decode = cbor_as_i64;
        csil_decode(csil_field)?
    };
    Ok(ProjectTaskOrderRequest {
        project_id,
        task_id,
        position,
    })
}

/// Encode a ProjectTaskOrderRequest to canonical CSIL CBOR bytes.
pub fn encode_project_task_order_request(csil_v: &ProjectTaskOrderRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_project_task_order_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectTaskOrderRequest.
pub fn decode_project_task_order_request(csil_data: &[u8]) -> Result<ProjectTaskOrderRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_task_order_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectMemberRef.
fn csil_enc_project_member_ref(csil_v: &ProjectMemberRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectMemberRef from a decoded CBOR value tree.
fn csil_dec_project_member_ref(csil_root: &CsilCborValue) -> Result<ProjectMemberRef, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectMemberRef {
        project_id,
        member_id,
    })
}

/// Encode a ProjectMemberRef to canonical CSIL CBOR bytes.
pub fn encode_project_member_ref(csil_v: &ProjectMemberRef) -> Vec<u8> {
    cbor_encode(&csil_enc_project_member_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectMemberRef.
pub fn decode_project_member_ref(csil_data: &[u8]) -> Result<ProjectMemberRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_member_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectOwnerRef.
fn csil_enc_project_owner_ref(csil_v: &ProjectOwnerRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("member_id"), cbor_text(&csil_v.member_id)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectOwnerRef from a decoded CBOR value tree.
fn csil_dec_project_owner_ref(csil_root: &CsilCborValue) -> Result<ProjectOwnerRef, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let member_id = {
        let csil_field = cbor_require(csil_root, "member_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectOwnerRef {
        project_id,
        member_id,
    })
}

/// Encode a ProjectOwnerRef to canonical CSIL CBOR bytes.
pub fn encode_project_owner_ref(csil_v: &ProjectOwnerRef) -> Vec<u8> {
    cbor_encode(&csil_enc_project_owner_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectOwnerRef.
pub fn decode_project_owner_ref(csil_data: &[u8]) -> Result<ProjectOwnerRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_owner_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a DependencyRef.
fn csil_enc_dependency_ref(csil_v: &DependencyRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("dependent_id"), cbor_text(&csil_v.dependent_id)));
    csil_entries.push((cbor_text("dependency_id"), cbor_text(&csil_v.dependency_id)));
    csil_entries.push((cbor_text("dependent_type"), csil_enc_dependency_node_type(&csil_v.dependent_type)));
    csil_entries.push((cbor_text("dependency_type"), csil_enc_dependency_node_type(&csil_v.dependency_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DependencyRef from a decoded CBOR value tree.
fn csil_dec_dependency_ref(csil_root: &CsilCborValue) -> Result<DependencyRef, CsilCborError> {
    let dependent_type = {
        let csil_field = cbor_require(csil_root, "dependent_type")?;
        let csil_decode = csil_dec_dependency_node_type;
        csil_decode(csil_field)?
    };
    let dependent_id = {
        let csil_field = cbor_require(csil_root, "dependent_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let dependency_type = {
        let csil_field = cbor_require(csil_root, "dependency_type")?;
        let csil_decode = csil_dec_dependency_node_type;
        csil_decode(csil_field)?
    };
    let dependency_id = {
        let csil_field = cbor_require(csil_root, "dependency_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(DependencyRef {
        dependent_type,
        dependent_id,
        dependency_type,
        dependency_id,
    })
}

/// Encode a DependencyRef to canonical CSIL CBOR bytes.
pub fn encode_dependency_ref(csil_v: &DependencyRef) -> Vec<u8> {
    cbor_encode(&csil_enc_dependency_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DependencyRef.
pub fn decode_dependency_ref(csil_data: &[u8]) -> Result<DependencyRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dependency_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a DependencyTarget.
fn csil_enc_dependency_target(csil_v: &DependencyTarget) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("id"), cbor_text(&csil_v.id)));
    csil_entries.push((cbor_text("type"), csil_enc_dependency_node_type(&csil_v.r#type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DependencyTarget from a decoded CBOR value tree.
fn csil_dec_dependency_target(csil_root: &CsilCborValue) -> Result<DependencyTarget, CsilCborError> {
    let r#type = {
        let csil_field = cbor_require(csil_root, "type")?;
        let csil_decode = csil_dec_dependency_node_type;
        csil_decode(csil_field)?
    };
    let id = {
        let csil_field = cbor_require(csil_root, "id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(DependencyTarget {
        r#type,
        id,
    })
}

/// Encode a DependencyTarget to canonical CSIL CBOR bytes.
pub fn encode_dependency_target(csil_v: &DependencyTarget) -> Vec<u8> {
    cbor_encode(&csil_enc_dependency_target(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DependencyTarget.
pub fn decode_dependency_target(csil_data: &[u8]) -> Result<DependencyTarget, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dependency_target(&csil_root)
}

/// Build the canonical CBOR value tree for a DependencyNode.
fn csil_enc_dependency_node(csil_v: &DependencyNode) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("id"), cbor_text(&csil_v.id)));
    csil_entries.push((cbor_text("type"), csil_enc_dependency_node_type(&csil_v.r#type)));
    csil_entries.push((cbor_text("title"), cbor_text(&csil_v.title)));
    if let Some(csil_inner) = &csil_v.status {
        csil_entries.push((cbor_text("status"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DependencyNode from a decoded CBOR value tree.
fn csil_dec_dependency_node(csil_root: &CsilCborValue) -> Result<DependencyNode, CsilCborError> {
    let r#type = {
        let csil_field = cbor_require(csil_root, "type")?;
        let csil_decode = csil_dec_dependency_node_type;
        csil_decode(csil_field)?
    };
    let id = {
        let csil_field = cbor_require(csil_root, "id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let title = {
        let csil_field = cbor_require(csil_root, "title")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let status = match cbor_map_get(csil_root, "status") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(DependencyNode {
        r#type,
        id,
        title,
        status,
    })
}

/// Encode a DependencyNode to canonical CSIL CBOR bytes.
pub fn encode_dependency_node(csil_v: &DependencyNode) -> Vec<u8> {
    cbor_encode(&csil_enc_dependency_node(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DependencyNode.
pub fn decode_dependency_node(csil_data: &[u8]) -> Result<DependencyNode, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dependency_node(&csil_root)
}

/// Build the canonical CBOR value tree for a DependencyGraph.
fn csil_enc_dependency_graph(csil_v: &DependencyGraph) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("dependents"), cbor_enc_array(&csil_v.dependents, |csil_elem| csil_enc_dependency_node(csil_elem))));
    csil_entries.push((cbor_text("dependencies"), cbor_enc_array(&csil_v.dependencies, |csil_elem| csil_enc_dependency_node(csil_elem))));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a DependencyGraph from a decoded CBOR value tree.
fn csil_dec_dependency_graph(csil_root: &CsilCborValue) -> Result<DependencyGraph, CsilCborError> {
    let dependencies = {
        let csil_field = cbor_require(csil_root, "dependencies")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_dependency_node);
        csil_decode(csil_field)?
    };
    let dependents = {
        let csil_field = cbor_require(csil_root, "dependents")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_dependency_node);
        csil_decode(csil_field)?
    };
    Ok(DependencyGraph {
        dependencies,
        dependents,
    })
}

/// Encode a DependencyGraph to canonical CSIL CBOR bytes.
pub fn encode_dependency_graph(csil_v: &DependencyGraph) -> Vec<u8> {
    cbor_encode(&csil_enc_dependency_graph(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a DependencyGraph.
pub fn decode_dependency_graph(csil_data: &[u8]) -> Result<DependencyGraph, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_dependency_graph(&csil_root)
}

/// Build the canonical CBOR value tree for a Grant.
fn csil_enc_grant(csil_v: &Grant) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("grantee_id"), cbor_text(&csil_v.grantee_id)));
    csil_entries.push((cbor_text("access_level"), csil_enc_access_level(&csil_v.access_level)));
    csil_entries.push((cbor_text("grantee_type"), csil_enc_grantee_type(&csil_v.grantee_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a Grant from a decoded CBOR value tree.
fn csil_dec_grant(csil_root: &CsilCborValue) -> Result<Grant, CsilCborError> {
    let grantee_type = {
        let csil_field = cbor_require(csil_root, "grantee_type")?;
        let csil_decode = csil_dec_grantee_type;
        csil_decode(csil_field)?
    };
    let grantee_id = {
        let csil_field = cbor_require(csil_root, "grantee_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let access_level = {
        let csil_field = cbor_require(csil_root, "access_level")?;
        let csil_decode = csil_dec_access_level;
        csil_decode(csil_field)?
    };
    Ok(Grant {
        grantee_type,
        grantee_id,
        access_level,
    })
}

/// Encode a Grant to canonical CSIL CBOR bytes.
pub fn encode_grant(csil_v: &Grant) -> Vec<u8> {
    cbor_encode(&csil_enc_grant(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a Grant.
pub fn decode_grant(csil_data: &[u8]) -> Result<Grant, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_grant(&csil_root)
}

/// Build the canonical CBOR value tree for a TaskGrantRef.
fn csil_enc_task_grant_ref(csil_v: &TaskGrantRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("grantee_id"), cbor_text(&csil_v.grantee_id)));
    csil_entries.push((cbor_text("grantee_type"), csil_enc_grantee_type(&csil_v.grantee_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a TaskGrantRef from a decoded CBOR value tree.
fn csil_dec_task_grant_ref(csil_root: &CsilCborValue) -> Result<TaskGrantRef, CsilCborError> {
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee_type = {
        let csil_field = cbor_require(csil_root, "grantee_type")?;
        let csil_decode = csil_dec_grantee_type;
        csil_decode(csil_field)?
    };
    let grantee_id = {
        let csil_field = cbor_require(csil_root, "grantee_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(TaskGrantRef {
        task_id,
        grantee_type,
        grantee_id,
    })
}

/// Encode a TaskGrantRef to canonical CSIL CBOR bytes.
pub fn encode_task_grant_ref(csil_v: &TaskGrantRef) -> Vec<u8> {
    cbor_encode(&csil_enc_task_grant_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a TaskGrantRef.
pub fn decode_task_grant_ref(csil_data: &[u8]) -> Result<TaskGrantRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_task_grant_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a PutTaskGrantRequest.
fn csil_enc_put_task_grant_request(csil_v: &PutTaskGrantRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("grantee_id"), cbor_text(&csil_v.grantee_id)));
    csil_entries.push((cbor_text("access_level"), csil_enc_access_level(&csil_v.access_level)));
    csil_entries.push((cbor_text("grantee_type"), csil_enc_grantee_type(&csil_v.grantee_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a PutTaskGrantRequest from a decoded CBOR value tree.
fn csil_dec_put_task_grant_request(csil_root: &CsilCborValue) -> Result<PutTaskGrantRequest, CsilCborError> {
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee_type = {
        let csil_field = cbor_require(csil_root, "grantee_type")?;
        let csil_decode = csil_dec_grantee_type;
        csil_decode(csil_field)?
    };
    let grantee_id = {
        let csil_field = cbor_require(csil_root, "grantee_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let access_level = {
        let csil_field = cbor_require(csil_root, "access_level")?;
        let csil_decode = csil_dec_access_level;
        csil_decode(csil_field)?
    };
    Ok(PutTaskGrantRequest {
        task_id,
        grantee_type,
        grantee_id,
        access_level,
    })
}

/// Encode a PutTaskGrantRequest to canonical CSIL CBOR bytes.
pub fn encode_put_task_grant_request(csil_v: &PutTaskGrantRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_put_task_grant_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a PutTaskGrantRequest.
pub fn decode_put_task_grant_request(csil_data: &[u8]) -> Result<PutTaskGrantRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_put_task_grant_request(&csil_root)
}

/// Build the canonical CBOR value tree for a SetTaskVisibilityRequest.
fn csil_enc_set_task_visibility_request(csil_v: &SetTaskVisibilityRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("task_id"), cbor_text(&csil_v.task_id)));
    csil_entries.push((cbor_text("visibility"), csil_enc_access_level(&csil_v.visibility)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a SetTaskVisibilityRequest from a decoded CBOR value tree.
fn csil_dec_set_task_visibility_request(csil_root: &CsilCborValue) -> Result<SetTaskVisibilityRequest, CsilCborError> {
    let task_id = {
        let csil_field = cbor_require(csil_root, "task_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let visibility = {
        let csil_field = cbor_require(csil_root, "visibility")?;
        let csil_decode = csil_dec_access_level;
        csil_decode(csil_field)?
    };
    Ok(SetTaskVisibilityRequest {
        task_id,
        visibility,
    })
}

/// Encode a SetTaskVisibilityRequest to canonical CSIL CBOR bytes.
pub fn encode_set_task_visibility_request(csil_v: &SetTaskVisibilityRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_set_task_visibility_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a SetTaskVisibilityRequest.
pub fn decode_set_task_visibility_request(csil_data: &[u8]) -> Result<SetTaskVisibilityRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_set_task_visibility_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ProjectGrantRef.
fn csil_enc_project_grant_ref(csil_v: &ProjectGrantRef) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("grantee_id"), cbor_text(&csil_v.grantee_id)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    csil_entries.push((cbor_text("grantee_type"), csil_enc_grantee_type(&csil_v.grantee_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ProjectGrantRef from a decoded CBOR value tree.
fn csil_dec_project_grant_ref(csil_root: &CsilCborValue) -> Result<ProjectGrantRef, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee_type = {
        let csil_field = cbor_require(csil_root, "grantee_type")?;
        let csil_decode = csil_dec_grantee_type;
        csil_decode(csil_field)?
    };
    let grantee_id = {
        let csil_field = cbor_require(csil_root, "grantee_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ProjectGrantRef {
        project_id,
        grantee_type,
        grantee_id,
    })
}

/// Encode a ProjectGrantRef to canonical CSIL CBOR bytes.
pub fn encode_project_grant_ref(csil_v: &ProjectGrantRef) -> Vec<u8> {
    cbor_encode(&csil_enc_project_grant_ref(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ProjectGrantRef.
pub fn decode_project_grant_ref(csil_data: &[u8]) -> Result<ProjectGrantRef, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_project_grant_ref(&csil_root)
}

/// Build the canonical CBOR value tree for a PutProjectGrantRequest.
fn csil_enc_put_project_grant_request(csil_v: &PutProjectGrantRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("grantee_id"), cbor_text(&csil_v.grantee_id)));
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    csil_entries.push((cbor_text("access_level"), csil_enc_access_level(&csil_v.access_level)));
    csil_entries.push((cbor_text("grantee_type"), csil_enc_grantee_type(&csil_v.grantee_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a PutProjectGrantRequest from a decoded CBOR value tree.
fn csil_dec_put_project_grant_request(csil_root: &CsilCborValue) -> Result<PutProjectGrantRequest, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let grantee_type = {
        let csil_field = cbor_require(csil_root, "grantee_type")?;
        let csil_decode = csil_dec_grantee_type;
        csil_decode(csil_field)?
    };
    let grantee_id = {
        let csil_field = cbor_require(csil_root, "grantee_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let access_level = {
        let csil_field = cbor_require(csil_root, "access_level")?;
        let csil_decode = csil_dec_access_level;
        csil_decode(csil_field)?
    };
    Ok(PutProjectGrantRequest {
        project_id,
        grantee_type,
        grantee_id,
        access_level,
    })
}

/// Encode a PutProjectGrantRequest to canonical CSIL CBOR bytes.
pub fn encode_put_project_grant_request(csil_v: &PutProjectGrantRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_put_project_grant_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a PutProjectGrantRequest.
pub fn decode_put_project_grant_request(csil_data: &[u8]) -> Result<PutProjectGrantRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_put_project_grant_request(&csil_root)
}

/// Build the canonical CBOR value tree for a SetProjectVisibilityRequest.
fn csil_enc_set_project_visibility_request(csil_v: &SetProjectVisibilityRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("project_id"), cbor_text(&csil_v.project_id)));
    csil_entries.push((cbor_text("visibility"), csil_enc_access_level(&csil_v.visibility)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a SetProjectVisibilityRequest from a decoded CBOR value tree.
fn csil_dec_set_project_visibility_request(csil_root: &CsilCborValue) -> Result<SetProjectVisibilityRequest, CsilCborError> {
    let project_id = {
        let csil_field = cbor_require(csil_root, "project_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let visibility = {
        let csil_field = cbor_require(csil_root, "visibility")?;
        let csil_decode = csil_dec_access_level;
        csil_decode(csil_field)?
    };
    Ok(SetProjectVisibilityRequest {
        project_id,
        visibility,
    })
}

/// Encode a SetProjectVisibilityRequest to canonical CSIL CBOR bytes.
pub fn encode_set_project_visibility_request(csil_v: &SetProjectVisibilityRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_set_project_visibility_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a SetProjectVisibilityRequest.
pub fn decode_set_project_visibility_request(csil_data: &[u8]) -> Result<SetProjectVisibilityRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_set_project_visibility_request(&csil_root)
}

/// Build the canonical CBOR value tree for a EffectiveSettings.
fn csil_enc_effective_settings(csil_v: &EffectiveSettings) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    if let Some(csil_inner) = &csil_v.bug_reports_enabled {
        csil_entries.push((cbor_text("bug_reports_enabled"), cbor_bool(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.bug_reports_project_id {
        csil_entries.push((cbor_text("bug_reports_project_id"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.default_project_visibility {
        csil_entries.push((cbor_text("default_project_visibility"), csil_enc_access_level(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a EffectiveSettings from a decoded CBOR value tree.
fn csil_dec_effective_settings(csil_root: &CsilCborValue) -> Result<EffectiveSettings, CsilCborError> {
    let bug_reports_enabled = match cbor_map_get(csil_root, "bug_reports_enabled") {
        Some(csil_field) => {
            let csil_decode = cbor_as_bool;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let bug_reports_project_id = match cbor_map_get(csil_root, "bug_reports_project_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let default_project_visibility = match cbor_map_get(csil_root, "default_project_visibility") {
        Some(csil_field) => {
            let csil_decode = csil_dec_access_level;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(EffectiveSettings {
        bug_reports_enabled,
        bug_reports_project_id,
        default_project_visibility,
    })
}

/// Encode a EffectiveSettings to canonical CSIL CBOR bytes.
pub fn encode_effective_settings(csil_v: &EffectiveSettings) -> Vec<u8> {
    cbor_encode(&csil_enc_effective_settings(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a EffectiveSettings.
pub fn decode_effective_settings(csil_data: &[u8]) -> Result<EffectiveSettings, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_effective_settings(&csil_root)
}

/// Build the canonical CBOR value tree for a UpdateSettingsRequest.
fn csil_enc_update_settings_request(csil_v: &UpdateSettingsRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("settings"), csil_enc_effective_settings(&csil_v.settings)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a UpdateSettingsRequest from a decoded CBOR value tree.
fn csil_dec_update_settings_request(csil_root: &CsilCborValue) -> Result<UpdateSettingsRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let settings = {
        let csil_field = cbor_require(csil_root, "settings")?;
        let csil_decode = csil_dec_effective_settings;
        csil_decode(csil_field)?
    };
    Ok(UpdateSettingsRequest {
        house_id,
        settings,
    })
}

/// Encode a UpdateSettingsRequest to canonical CSIL CBOR bytes.
pub fn encode_update_settings_request(csil_v: &UpdateSettingsRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_update_settings_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a UpdateSettingsRequest.
pub fn decode_update_settings_request(csil_data: &[u8]) -> Result<UpdateSettingsRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_update_settings_request(&csil_root)
}

/// Build the canonical CBOR value tree for a BugReportRequest.
fn csil_enc_bug_report_request(csil_v: &BugReportRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("title"), cbor_text(&csil_v.title)));
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.description {
        csil_entries.push((cbor_text("description"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a BugReportRequest from a decoded CBOR value tree.
fn csil_dec_bug_report_request(csil_root: &CsilCborValue) -> Result<BugReportRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let title = {
        let csil_field = cbor_require(csil_root, "title")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let description = match cbor_map_get(csil_root, "description") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(BugReportRequest {
        house_id,
        title,
        description,
    })
}

/// Encode a BugReportRequest to canonical CSIL CBOR bytes.
pub fn encode_bug_report_request(csil_v: &BugReportRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_bug_report_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a BugReportRequest.
pub fn decode_bug_report_request(csil_data: &[u8]) -> Result<BugReportRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_bug_report_request(&csil_root)
}

/// Build the canonical CBOR value tree for a ServiceError.
fn csil_enc_service_error(csil_v: &ServiceError) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("code"), cbor_uint(csil_v.code)));
    csil_entries.push((cbor_text("message"), cbor_text(&csil_v.message)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a ServiceError from a decoded CBOR value tree.
fn csil_dec_service_error(csil_root: &CsilCborValue) -> Result<ServiceError, CsilCborError> {
    let code = {
        let csil_field = cbor_require(csil_root, "code")?;
        let csil_decode = cbor_as_u64;
        csil_decode(csil_field)?
    };
    let message = {
        let csil_field = cbor_require(csil_root, "message")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(ServiceError {
        code,
        message,
    })
}

/// Encode a ServiceError to canonical CSIL CBOR bytes.
pub fn encode_service_error(csil_v: &ServiceError) -> Vec<u8> {
    cbor_encode(&csil_enc_service_error(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a ServiceError.
pub fn decode_service_error(csil_data: &[u8]) -> Result<ServiceError, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_service_error(&csil_root)
}

/// Build the canonical CBOR value tree for a AuditEntry.
fn csil_enc_audit_entry(csil_v: &AuditEntry) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(15);
    if let Some(csil_inner) = &csil_v.after {
        csil_entries.push((cbor_text("after"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("action"), cbor_text(&csil_v.action)));
    if let Some(csil_inner) = &csil_v.before {
        csil_entries.push((cbor_text("before"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.detail {
        csil_entries.push((cbor_text("detail"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("method"), cbor_text(&csil_v.method)));
    csil_entries.push((cbor_text("outcome"), cbor_text(&csil_v.outcome)));
    csil_entries.push((cbor_text("audit_id"), cbor_text(&csil_v.audit_id)));
    if let Some(csil_inner) = &csil_v.house_id {
        csil_entries.push((cbor_text("house_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("created_at"), cbor_text(&csil_v.created_at)));
    if let Some(csil_inner) = &csil_v.resource_id {
        csil_entries.push((cbor_text("resource_id"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("actor_domain"), cbor_text(&csil_v.actor_domain)));
    csil_entries.push((cbor_text("service_name"), cbor_text(&csil_v.service_name)));
    csil_entries.push((cbor_text("actor_user_id"), cbor_text(&csil_v.actor_user_id)));
    if let Some(csil_inner) = &csil_v.resource_type {
        csil_entries.push((cbor_text("resource_type"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.actor_member_id {
        csil_entries.push((cbor_text("actor_member_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AuditEntry from a decoded CBOR value tree.
fn csil_dec_audit_entry(csil_root: &CsilCborValue) -> Result<AuditEntry, CsilCborError> {
    let audit_id = {
        let csil_field = cbor_require(csil_root, "audit_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = match cbor_map_get(csil_root, "house_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let actor_member_id = match cbor_map_get(csil_root, "actor_member_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let actor_domain = {
        let csil_field = cbor_require(csil_root, "actor_domain")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let actor_user_id = {
        let csil_field = cbor_require(csil_root, "actor_user_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let service_name = {
        let csil_field = cbor_require(csil_root, "service_name")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let method = {
        let csil_field = cbor_require(csil_root, "method")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let action = {
        let csil_field = cbor_require(csil_root, "action")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let resource_type = match cbor_map_get(csil_root, "resource_type") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let resource_id = match cbor_map_get(csil_root, "resource_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let outcome = {
        let csil_field = cbor_require(csil_root, "outcome")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let before = match cbor_map_get(csil_root, "before") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let after = match cbor_map_get(csil_root, "after") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let detail = match cbor_map_get(csil_root, "detail") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let created_at = {
        let csil_field = cbor_require(csil_root, "created_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(AuditEntry {
        audit_id,
        house_id,
        actor_member_id,
        actor_domain,
        actor_user_id,
        service_name,
        method,
        action,
        resource_type,
        resource_id,
        outcome,
        before,
        after,
        detail,
        created_at,
    })
}

/// Encode a AuditEntry to canonical CSIL CBOR bytes.
pub fn encode_audit_entry(csil_v: &AuditEntry) -> Vec<u8> {
    cbor_encode(&csil_enc_audit_entry(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AuditEntry.
pub fn decode_audit_entry(csil_data: &[u8]) -> Result<AuditEntry, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_audit_entry(&csil_root)
}

/// Build the canonical CBOR value tree for a AuditQuery.
fn csil_enc_audit_query(csil_v: &AuditQuery) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(8);
    if let Some(csil_inner) = &csil_v.limit {
        csil_entries.push((cbor_text("limit"), cbor_uint(*csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.since {
        csil_entries.push((cbor_text("since"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.until {
        csil_entries.push((cbor_text("until"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.action {
        csil_entries.push((cbor_text("action"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.cursor {
        csil_entries.push((cbor_text("cursor"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.resource_type {
        csil_entries.push((cbor_text("resource_type"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.actor_member_id {
        csil_entries.push((cbor_text("actor_member_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AuditQuery from a decoded CBOR value tree.
fn csil_dec_audit_query(csil_root: &CsilCborValue) -> Result<AuditQuery, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let actor_member_id = match cbor_map_get(csil_root, "actor_member_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let resource_type = match cbor_map_get(csil_root, "resource_type") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let action = match cbor_map_get(csil_root, "action") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let since = match cbor_map_get(csil_root, "since") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let until = match cbor_map_get(csil_root, "until") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let cursor = match cbor_map_get(csil_root, "cursor") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let limit = match cbor_map_get(csil_root, "limit") {
        Some(csil_field) => {
            let csil_decode = cbor_as_u64;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(AuditQuery {
        house_id,
        actor_member_id,
        resource_type,
        action,
        since,
        until,
        cursor,
        limit,
    })
}

/// Encode a AuditQuery to canonical CSIL CBOR bytes.
pub fn encode_audit_query(csil_v: &AuditQuery) -> Vec<u8> {
    cbor_encode(&csil_enc_audit_query(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AuditQuery.
pub fn decode_audit_query(csil_data: &[u8]) -> Result<AuditQuery, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_audit_query(&csil_root)
}

/// Build the canonical CBOR value tree for a AuditPage.
fn csil_enc_audit_page(csil_v: &AuditPage) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("entries"), cbor_enc_array(&csil_v.entries, |csil_elem| csil_enc_audit_entry(csil_elem))));
    if let Some(csil_inner) = &csil_v.next_cursor {
        csil_entries.push((cbor_text("next_cursor"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a AuditPage from a decoded CBOR value tree.
fn csil_dec_audit_page(csil_root: &CsilCborValue) -> Result<AuditPage, CsilCborError> {
    let entries = {
        let csil_field = cbor_require(csil_root, "entries")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_audit_entry);
        csil_decode(csil_field)?
    };
    let next_cursor = match cbor_map_get(csil_root, "next_cursor") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(AuditPage {
        entries,
        next_cursor,
    })
}

/// Encode a AuditPage to canonical CSIL CBOR bytes.
pub fn encode_audit_page(csil_v: &AuditPage) -> Vec<u8> {
    cbor_encode(&csil_enc_audit_page(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a AuditPage.
pub fn decode_audit_page(csil_data: &[u8]) -> Result<AuditPage, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_audit_page(&csil_root)
}

/// Build the canonical CBOR value tree for a TrashItem.
fn csil_enc_trash_item(csil_v: &TrashItem) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(7);
    if let Some(csil_inner) = &csil_v.title {
        csil_entries.push((cbor_text("title"), cbor_text(csil_inner)));
    }
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("deleted_at"), cbor_text(&csil_v.deleted_at)));
    csil_entries.push((cbor_text("resource_id"), cbor_text(&csil_v.resource_id)));
    csil_entries.push((cbor_text("deleted_op_id"), cbor_text(&csil_v.deleted_op_id)));
    csil_entries.push((cbor_text("resource_type"), cbor_text(&csil_v.resource_type)));
    if let Some(csil_inner) = &csil_v.deleted_by_member_id {
        csil_entries.push((cbor_text("deleted_by_member_id"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a TrashItem from a decoded CBOR value tree.
fn csil_dec_trash_item(csil_root: &CsilCborValue) -> Result<TrashItem, CsilCborError> {
    let resource_type = {
        let csil_field = cbor_require(csil_root, "resource_type")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let resource_id = {
        let csil_field = cbor_require(csil_root, "resource_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let title = match cbor_map_get(csil_root, "title") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let deleted_at = {
        let csil_field = cbor_require(csil_root, "deleted_at")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let deleted_by_member_id = match cbor_map_get(csil_root, "deleted_by_member_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let deleted_op_id = {
        let csil_field = cbor_require(csil_root, "deleted_op_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(TrashItem {
        resource_type,
        resource_id,
        house_id,
        title,
        deleted_at,
        deleted_by_member_id,
        deleted_op_id,
    })
}

/// Encode a TrashItem to canonical CSIL CBOR bytes.
pub fn encode_trash_item(csil_v: &TrashItem) -> Vec<u8> {
    cbor_encode(&csil_enc_trash_item(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a TrashItem.
pub fn decode_trash_item(csil_data: &[u8]) -> Result<TrashItem, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_trash_item(&csil_root)
}

/// Build the canonical CBOR value tree for a TrashPage.
fn csil_enc_trash_page(csil_v: &TrashPage) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(2);
    csil_entries.push((cbor_text("items"), cbor_enc_array(&csil_v.items, |csil_elem| csil_enc_trash_item(csil_elem))));
    if let Some(csil_inner) = &csil_v.next_cursor {
        csil_entries.push((cbor_text("next_cursor"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a TrashPage from a decoded CBOR value tree.
fn csil_dec_trash_page(csil_root: &CsilCborValue) -> Result<TrashPage, CsilCborError> {
    let items = {
        let csil_field = cbor_require(csil_root, "items")?;
        let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_trash_item);
        csil_decode(csil_field)?
    };
    let next_cursor = match cbor_map_get(csil_root, "next_cursor") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(TrashPage {
        items,
        next_cursor,
    })
}

/// Encode a TrashPage to canonical CSIL CBOR bytes.
pub fn encode_trash_page(csil_v: &TrashPage) -> Vec<u8> {
    cbor_encode(&csil_enc_trash_page(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a TrashPage.
pub fn decode_trash_page(csil_data: &[u8]) -> Result<TrashPage, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_trash_page(&csil_root)
}

/// Build the canonical CBOR value tree for a RestoreRequest.
fn csil_enc_restore_request(csil_v: &RestoreRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(4);
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    if let Some(csil_inner) = &csil_v.resource_id {
        csil_entries.push((cbor_text("resource_id"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.deleted_op_id {
        csil_entries.push((cbor_text("deleted_op_id"), cbor_text(csil_inner)));
    }
    if let Some(csil_inner) = &csil_v.resource_type {
        csil_entries.push((cbor_text("resource_type"), cbor_text(csil_inner)));
    }
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a RestoreRequest from a decoded CBOR value tree.
fn csil_dec_restore_request(csil_root: &CsilCborValue) -> Result<RestoreRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let deleted_op_id = match cbor_map_get(csil_root, "deleted_op_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let resource_type = match cbor_map_get(csil_root, "resource_type") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    let resource_id = match cbor_map_get(csil_root, "resource_id") {
        Some(csil_field) => {
            let csil_decode = cbor_as_text;
            Some(csil_decode(csil_field)?)
        }
        None => None,
    };
    Ok(RestoreRequest {
        house_id,
        deleted_op_id,
        resource_type,
        resource_id,
    })
}

/// Encode a RestoreRequest to canonical CSIL CBOR bytes.
pub fn encode_restore_request(csil_v: &RestoreRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_restore_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a RestoreRequest.
pub fn decode_restore_request(csil_data: &[u8]) -> Result<RestoreRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_restore_request(&csil_root)
}

/// Build the canonical CBOR value tree for a PurgeRequest.
fn csil_enc_purge_request(csil_v: &PurgeRequest) -> CsilCborValue {
    let mut csil_entries: Vec<(CsilCborValue, CsilCborValue)> = Vec::with_capacity(3);
    csil_entries.push((cbor_text("house_id"), cbor_text(&csil_v.house_id)));
    csil_entries.push((cbor_text("resource_id"), cbor_text(&csil_v.resource_id)));
    csil_entries.push((cbor_text("resource_type"), cbor_text(&csil_v.resource_type)));
    CsilCborValue::Map(csil_entries)
}

/// Reconstruct a PurgeRequest from a decoded CBOR value tree.
fn csil_dec_purge_request(csil_root: &CsilCborValue) -> Result<PurgeRequest, CsilCborError> {
    let house_id = {
        let csil_field = cbor_require(csil_root, "house_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let resource_type = {
        let csil_field = cbor_require(csil_root, "resource_type")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    let resource_id = {
        let csil_field = cbor_require(csil_root, "resource_id")?;
        let csil_decode = cbor_as_text;
        csil_decode(csil_field)?
    };
    Ok(PurgeRequest {
        house_id,
        resource_type,
        resource_id,
    })
}

/// Encode a PurgeRequest to canonical CSIL CBOR bytes.
pub fn encode_purge_request(csil_v: &PurgeRequest) -> Vec<u8> {
    cbor_encode(&csil_enc_purge_request(csil_v))
}

/// Decode canonical CSIL CBOR bytes into a PurgeRequest.
pub fn decode_purge_request(csil_data: &[u8]) -> Result<PurgeRequest, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    csil_dec_purge_request(&csil_root)
}

/// Encode a TaskStatus enum as its bare literal value.
fn csil_enc_task_status(csil_v: &TaskStatus) -> CsilCborValue {
    match csil_v {
        TaskStatus::Open => cbor_text("open"),
        TaskStatus::InProgress => cbor_text("in_progress"),
        TaskStatus::Done => cbor_text("done"),
        TaskStatus::Cancelled => cbor_text("cancelled"),
    }
}

/// Decode a bare literal value into a TaskStatus enum.
fn csil_dec_task_status(csil_v: &CsilCborValue) -> Result<TaskStatus, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "open" => Ok(TaskStatus::Open),
        "in_progress" => Ok(TaskStatus::InProgress),
        "done" => Ok(TaskStatus::Done),
        "cancelled" => Ok(TaskStatus::Cancelled),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown TaskStatus value {csil_other:?}"))),
    }
}

/// Encode a TargetType enum as its bare literal value.
fn csil_enc_target_type(csil_v: &TargetType) -> CsilCborValue {
    match csil_v {
        TargetType::Event => cbor_text("event"),
        TargetType::Task => cbor_text("task"),
        TargetType::Project => cbor_text("project"),
    }
}

/// Decode a bare literal value into a TargetType enum.
fn csil_dec_target_type(csil_v: &CsilCborValue) -> Result<TargetType, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "event" => Ok(TargetType::Event),
        "task" => Ok(TargetType::Task),
        "project" => Ok(TargetType::Project),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown TargetType value {csil_other:?}"))),
    }
}

/// Encode a AccessLevel enum as its bare literal value.
fn csil_enc_access_level(csil_v: &AccessLevel) -> CsilCborValue {
    match csil_v {
        AccessLevel::None => cbor_text("none"),
        AccessLevel::Read => cbor_text("read"),
        AccessLevel::Edit => cbor_text("edit"),
        AccessLevel::Full => cbor_text("full"),
    }
}

/// Decode a bare literal value into a AccessLevel enum.
fn csil_dec_access_level(csil_v: &CsilCborValue) -> Result<AccessLevel, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "none" => Ok(AccessLevel::None),
        "read" => Ok(AccessLevel::Read),
        "edit" => Ok(AccessLevel::Edit),
        "full" => Ok(AccessLevel::Full),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown AccessLevel value {csil_other:?}"))),
    }
}

/// Encode a GranteeType enum as its bare literal value.
fn csil_enc_grantee_type(csil_v: &GranteeType) -> CsilCborValue {
    match csil_v {
        GranteeType::Member => cbor_text("member"),
        GranteeType::Group => cbor_text("group"),
    }
}

/// Decode a bare literal value into a GranteeType enum.
fn csil_dec_grantee_type(csil_v: &CsilCborValue) -> Result<GranteeType, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "member" => Ok(GranteeType::Member),
        "group" => Ok(GranteeType::Group),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown GranteeType value {csil_other:?}"))),
    }
}

/// Encode a ResourceType enum as its bare literal value.
fn csil_enc_resource_type(csil_v: &ResourceType) -> CsilCborValue {
    match csil_v {
        ResourceType::Event => cbor_text("event"),
        ResourceType::Task => cbor_text("task"),
        ResourceType::House => cbor_text("house"),
    }
}

/// Decode a bare literal value into a ResourceType enum.
fn csil_dec_resource_type(csil_v: &CsilCborValue) -> Result<ResourceType, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "event" => Ok(ResourceType::Event),
        "task" => Ok(ResourceType::Task),
        "house" => Ok(ResourceType::House),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown ResourceType value {csil_other:?}"))),
    }
}

/// Encode a ProjectStatus enum as its bare literal value.
fn csil_enc_project_status(csil_v: &ProjectStatus) -> CsilCborValue {
    match csil_v {
        ProjectStatus::Active => cbor_text("active"),
        ProjectStatus::Archived => cbor_text("archived"),
    }
}

/// Decode a bare literal value into a ProjectStatus enum.
fn csil_dec_project_status(csil_v: &CsilCborValue) -> Result<ProjectStatus, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "active" => Ok(ProjectStatus::Active),
        "archived" => Ok(ProjectStatus::Archived),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown ProjectStatus value {csil_other:?}"))),
    }
}

/// Encode a DependencyNodeType enum as its bare literal value.
fn csil_enc_dependency_node_type(csil_v: &DependencyNodeType) -> CsilCborValue {
    match csil_v {
        DependencyNodeType::Task => cbor_text("task"),
        DependencyNodeType::Project => cbor_text("project"),
    }
}

/// Decode a bare literal value into a DependencyNodeType enum.
fn csil_dec_dependency_node_type(csil_v: &CsilCborValue) -> Result<DependencyNodeType, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "task" => Ok(DependencyNodeType::Task),
        "project" => Ok(DependencyNodeType::Project),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown DependencyNodeType value {csil_other:?}"))),
    }
}

/// Encode a RecurrenceFreq enum as its bare literal value.
fn csil_enc_recurrence_freq(csil_v: &RecurrenceFreq) -> CsilCborValue {
    match csil_v {
        RecurrenceFreq::Hourly => cbor_text("hourly"),
        RecurrenceFreq::Daily => cbor_text("daily"),
        RecurrenceFreq::Weekly => cbor_text("weekly"),
        RecurrenceFreq::Monthly => cbor_text("monthly"),
        RecurrenceFreq::Quarterly => cbor_text("quarterly"),
        RecurrenceFreq::Yearly => cbor_text("yearly"),
    }
}

/// Decode a bare literal value into a RecurrenceFreq enum.
fn csil_dec_recurrence_freq(csil_v: &CsilCborValue) -> Result<RecurrenceFreq, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "hourly" => Ok(RecurrenceFreq::Hourly),
        "daily" => Ok(RecurrenceFreq::Daily),
        "weekly" => Ok(RecurrenceFreq::Weekly),
        "monthly" => Ok(RecurrenceFreq::Monthly),
        "quarterly" => Ok(RecurrenceFreq::Quarterly),
        "yearly" => Ok(RecurrenceFreq::Yearly),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown RecurrenceFreq value {csil_other:?}"))),
    }
}

/// Encode a MilestoneState enum as its bare literal value.
fn csil_enc_milestone_state(csil_v: &MilestoneState) -> CsilCborValue {
    match csil_v {
        MilestoneState::Done => cbor_text("done"),
        MilestoneState::Current => cbor_text("current"),
        MilestoneState::Future => cbor_text("future"),
    }
}

/// Decode a bare literal value into a MilestoneState enum.
fn csil_dec_milestone_state(csil_v: &CsilCborValue) -> Result<MilestoneState, CsilCborError> {
    let csil_val = cbor_as_text(csil_v)?;
    match csil_val.as_str() {
        "done" => Ok(MilestoneState::Done),
        "current" => Ok(MilestoneState::Current),
        "future" => Ok(MilestoneState::Future),
        csil_other => Err(CsilCborError(format!("csil cbor: unknown MilestoneState value {csil_other:?}"))),
    }
}

/// Encode the house_get_house_request payload to canonical CSIL CBOR bytes.
pub fn encode_house_get_house_request(csil_v: &HouseID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the house_get_house_request payload.
pub fn decode_house_get_house_request(csil_data: &[u8]) -> Result<HouseID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the house_delete_house_request payload to canonical CSIL CBOR bytes.
pub fn encode_house_delete_house_request(csil_v: &HouseID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the house_delete_house_request payload.
pub fn decode_house_delete_house_request(csil_data: &[u8]) -> Result<HouseID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the house_list_houses_response payload to canonical CSIL CBOR bytes.
pub fn encode_house_list_houses_response(csil_v: &Vec<House>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_house(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the house_list_houses_response payload.
pub fn decode_house_list_houses_response(csil_data: &[u8]) -> Result<Vec<House>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_house);
    csil_decode(&csil_root)
}

/// Encode the member_get_member_request payload to canonical CSIL CBOR bytes.
pub fn encode_member_get_member_request(csil_v: &MemberID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the member_get_member_request payload.
pub fn decode_member_get_member_request(csil_data: &[u8]) -> Result<MemberID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the member_deactivate_member_request payload to canonical CSIL CBOR bytes.
pub fn encode_member_deactivate_member_request(csil_v: &MemberID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the member_deactivate_member_request payload.
pub fn decode_member_deactivate_member_request(csil_data: &[u8]) -> Result<MemberID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the member_reactivate_member_request payload to canonical CSIL CBOR bytes.
pub fn encode_member_reactivate_member_request(csil_v: &MemberID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the member_reactivate_member_request payload.
pub fn decode_member_reactivate_member_request(csil_data: &[u8]) -> Result<MemberID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the member_list_members_response payload to canonical CSIL CBOR bytes.
pub fn encode_member_list_members_response(csil_v: &Vec<Member>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_member(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the member_list_members_response payload.
pub fn decode_member_list_members_response(csil_data: &[u8]) -> Result<Vec<Member>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_member);
    csil_decode(&csil_root)
}

/// Encode the trusted_domain_remove_trusted_domain_request payload to canonical CSIL CBOR bytes.
pub fn encode_trusted_domain_remove_trusted_domain_request(csil_v: &TrustedDomainID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the trusted_domain_remove_trusted_domain_request payload.
pub fn decode_trusted_domain_remove_trusted_domain_request(csil_data: &[u8]) -> Result<TrustedDomainID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the trusted_domain_list_trusted_domains_request payload to canonical CSIL CBOR bytes.
pub fn encode_trusted_domain_list_trusted_domains_request(csil_v: &HouseID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the trusted_domain_list_trusted_domains_request payload.
pub fn decode_trusted_domain_list_trusted_domains_request(csil_data: &[u8]) -> Result<HouseID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the trusted_domain_list_trusted_domains_response payload to canonical CSIL CBOR bytes.
pub fn encode_trusted_domain_list_trusted_domains_response(csil_v: &Vec<TrustedDomain>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_trusted_domain(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the trusted_domain_list_trusted_domains_response payload.
pub fn decode_trusted_domain_list_trusted_domains_response(csil_data: &[u8]) -> Result<Vec<TrustedDomain>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_trusted_domain);
    csil_decode(&csil_root)
}

/// Encode the role_delete_role_request payload to canonical CSIL CBOR bytes.
pub fn encode_role_delete_role_request(csil_v: &RoleID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the role_delete_role_request payload.
pub fn decode_role_delete_role_request(csil_data: &[u8]) -> Result<RoleID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the role_list_roles_response payload to canonical CSIL CBOR bytes.
pub fn encode_role_list_roles_response(csil_v: &Vec<Role>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_role(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the role_list_roles_response payload.
pub fn decode_role_list_roles_response(csil_data: &[u8]) -> Result<Vec<Role>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_role);
    csil_decode(&csil_root)
}

/// Encode the role_list_member_roles_response payload to canonical CSIL CBOR bytes.
pub fn encode_role_list_member_roles_response(csil_v: &Vec<Role>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_role(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the role_list_member_roles_response payload.
pub fn decode_role_list_member_roles_response(csil_data: &[u8]) -> Result<Vec<Role>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_role);
    csil_decode(&csil_root)
}

/// Encode the skill_delete_skill_request payload to canonical CSIL CBOR bytes.
pub fn encode_skill_delete_skill_request(csil_v: &SkillID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the skill_delete_skill_request payload.
pub fn decode_skill_delete_skill_request(csil_data: &[u8]) -> Result<SkillID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the skill_list_skills_response payload to canonical CSIL CBOR bytes.
pub fn encode_skill_list_skills_response(csil_v: &Vec<Skill>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_skill(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the skill_list_skills_response payload.
pub fn decode_skill_list_skills_response(csil_data: &[u8]) -> Result<Vec<Skill>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_skill);
    csil_decode(&csil_root)
}

/// Encode the skill_list_member_skills_response payload to canonical CSIL CBOR bytes.
pub fn encode_skill_list_member_skills_response(csil_v: &Vec<Skill>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_skill(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the skill_list_member_skills_response payload.
pub fn decode_skill_list_member_skills_response(csil_data: &[u8]) -> Result<Vec<Skill>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_skill);
    csil_decode(&csil_root)
}

/// Encode the skill_list_group_skills_request payload to canonical CSIL CBOR bytes.
pub fn encode_skill_list_group_skills_request(csil_v: &GroupID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the skill_list_group_skills_request payload.
pub fn decode_skill_list_group_skills_request(csil_data: &[u8]) -> Result<GroupID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the skill_list_group_skills_response payload to canonical CSIL CBOR bytes.
pub fn encode_skill_list_group_skills_response(csil_v: &Vec<Skill>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_skill(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the skill_list_group_skills_response payload.
pub fn decode_skill_list_group_skills_response(csil_data: &[u8]) -> Result<Vec<Skill>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_skill);
    csil_decode(&csil_root)
}

/// Encode the group_delete_group_request payload to canonical CSIL CBOR bytes.
pub fn encode_group_delete_group_request(csil_v: &GroupID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the group_delete_group_request payload.
pub fn decode_group_delete_group_request(csil_data: &[u8]) -> Result<GroupID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the group_list_groups_response payload to canonical CSIL CBOR bytes.
pub fn encode_group_list_groups_response(csil_v: &Vec<Group>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_group(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the group_list_groups_response payload.
pub fn decode_group_list_groups_response(csil_data: &[u8]) -> Result<Vec<Group>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_group);
    csil_decode(&csil_root)
}

/// Encode the group_list_group_members_response payload to canonical CSIL CBOR bytes.
pub fn encode_group_list_group_members_response(csil_v: &Vec<Member>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_member(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the group_list_group_members_response payload.
pub fn decode_group_list_group_members_response(csil_data: &[u8]) -> Result<Vec<Member>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_member);
    csil_decode(&csil_root)
}

/// Encode the project_get_project_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_get_project_request(csil_v: &ProjectID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_get_project_request payload.
pub fn decode_project_get_project_request(csil_data: &[u8]) -> Result<ProjectID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_delete_project_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_delete_project_request(csil_v: &ProjectID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_delete_project_request payload.
pub fn decode_project_delete_project_request(csil_data: &[u8]) -> Result<ProjectID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_list_project_members_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_project_members_request(csil_v: &ProjectID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_list_project_members_request payload.
pub fn decode_project_list_project_members_request(csil_data: &[u8]) -> Result<ProjectID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_list_project_members_response payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_project_members_response(csil_v: &Vec<Member>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_member(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the project_list_project_members_response payload.
pub fn decode_project_list_project_members_response(csil_data: &[u8]) -> Result<Vec<Member>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_member);
    csil_decode(&csil_root)
}

/// Encode the project_list_project_owners_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_project_owners_request(csil_v: &ProjectID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_list_project_owners_request payload.
pub fn decode_project_list_project_owners_request(csil_data: &[u8]) -> Result<ProjectID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_list_project_owners_response payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_project_owners_response(csil_v: &Vec<Member>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_member(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the project_list_project_owners_response payload.
pub fn decode_project_list_project_owners_response(csil_data: &[u8]) -> Result<Vec<Member>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_member);
    csil_decode(&csil_root)
}

/// Encode the project_list_milestones_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_milestones_request(csil_v: &ProjectID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_list_milestones_request payload.
pub fn decode_project_list_milestones_request(csil_data: &[u8]) -> Result<ProjectID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_list_milestones_response payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_milestones_response(csil_v: &Vec<Milestone>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_milestone(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the project_list_milestones_response payload.
pub fn decode_project_list_milestones_response(csil_data: &[u8]) -> Result<Vec<Milestone>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_milestone);
    csil_decode(&csil_root)
}

/// Encode the project_delete_milestone_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_delete_milestone_request(csil_v: &MilestoneID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_delete_milestone_request payload.
pub fn decode_project_delete_milestone_request(csil_data: &[u8]) -> Result<MilestoneID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_list_project_grants_request payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_project_grants_request(csil_v: &ProjectID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the project_list_project_grants_request payload.
pub fn decode_project_list_project_grants_request(csil_data: &[u8]) -> Result<ProjectID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the project_list_project_grants_response payload to canonical CSIL CBOR bytes.
pub fn encode_project_list_project_grants_response(csil_v: &Vec<Grant>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_grant(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the project_list_project_grants_response payload.
pub fn decode_project_list_project_grants_response(csil_data: &[u8]) -> Result<Vec<Grant>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_grant);
    csil_decode(&csil_root)
}

/// Encode the event_get_event_request payload to canonical CSIL CBOR bytes.
pub fn encode_event_get_event_request(csil_v: &EventID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the event_get_event_request payload.
pub fn decode_event_get_event_request(csil_data: &[u8]) -> Result<EventID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the event_delete_event_request payload to canonical CSIL CBOR bytes.
pub fn encode_event_delete_event_request(csil_v: &EventID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the event_delete_event_request payload.
pub fn decode_event_delete_event_request(csil_data: &[u8]) -> Result<EventID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the event_delete_event_and_future_request payload to canonical CSIL CBOR bytes.
pub fn encode_event_delete_event_and_future_request(csil_v: &EventID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the event_delete_event_and_future_request payload.
pub fn decode_event_delete_event_and_future_request(csil_data: &[u8]) -> Result<EventID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the event_list_events_response payload to canonical CSIL CBOR bytes.
pub fn encode_event_list_events_response(csil_v: &Vec<Event>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_event(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the event_list_events_response payload.
pub fn decode_event_list_events_response(csil_data: &[u8]) -> Result<Vec<Event>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_event);
    csil_decode(&csil_root)
}

/// Encode the task_get_task_request payload to canonical CSIL CBOR bytes.
pub fn encode_task_get_task_request(csil_v: &TaskID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the task_get_task_request payload.
pub fn decode_task_get_task_request(csil_data: &[u8]) -> Result<TaskID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the task_delete_task_request payload to canonical CSIL CBOR bytes.
pub fn encode_task_delete_task_request(csil_v: &TaskID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the task_delete_task_request payload.
pub fn decode_task_delete_task_request(csil_data: &[u8]) -> Result<TaskID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the task_list_task_grants_request payload to canonical CSIL CBOR bytes.
pub fn encode_task_list_task_grants_request(csil_v: &TaskID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the task_list_task_grants_request payload.
pub fn decode_task_list_task_grants_request(csil_data: &[u8]) -> Result<TaskID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the task_list_task_grants_response payload to canonical CSIL CBOR bytes.
pub fn encode_task_list_task_grants_response(csil_v: &Vec<Grant>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_grant(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the task_list_task_grants_response payload.
pub fn decode_task_list_task_grants_response(csil_data: &[u8]) -> Result<Vec<Grant>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_grant);
    csil_decode(&csil_root)
}

/// Encode the comment_get_comment_request payload to canonical CSIL CBOR bytes.
pub fn encode_comment_get_comment_request(csil_v: &CommentID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the comment_get_comment_request payload.
pub fn decode_comment_get_comment_request(csil_data: &[u8]) -> Result<CommentID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the comment_delete_comment_request payload to canonical CSIL CBOR bytes.
pub fn encode_comment_delete_comment_request(csil_v: &CommentID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the comment_delete_comment_request payload.
pub fn decode_comment_delete_comment_request(csil_data: &[u8]) -> Result<CommentID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the comment_list_comments_response payload to canonical CSIL CBOR bytes.
pub fn encode_comment_list_comments_response(csil_v: &Vec<Comment>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_comment(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the comment_list_comments_response payload.
pub fn decode_comment_list_comments_response(csil_data: &[u8]) -> Result<Vec<Comment>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_comment);
    csil_decode(&csil_root)
}

/// Encode the notification_list_notifications_response payload to canonical CSIL CBOR bytes.
pub fn encode_notification_list_notifications_response(csil_v: &Vec<Notification>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_notification(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the notification_list_notifications_response payload.
pub fn decode_notification_list_notifications_response(csil_data: &[u8]) -> Result<Vec<Notification>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_notification);
    csil_decode(&csil_root)
}

/// Encode the notification_unread_count_request payload to canonical CSIL CBOR bytes.
pub fn encode_notification_unread_count_request(csil_v: &HouseID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the notification_unread_count_request payload.
pub fn decode_notification_unread_count_request(csil_data: &[u8]) -> Result<HouseID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the notification_mark_read_request payload to canonical CSIL CBOR bytes.
pub fn encode_notification_mark_read_request(csil_v: &NotificationID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the notification_mark_read_request payload.
pub fn decode_notification_mark_read_request(csil_data: &[u8]) -> Result<NotificationID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the notification_mark_all_read_request payload to canonical CSIL CBOR bytes.
pub fn encode_notification_mark_all_read_request(csil_v: &HouseID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the notification_mark_all_read_request payload.
pub fn decode_notification_mark_all_read_request(csil_data: &[u8]) -> Result<HouseID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the share_delete_share_request payload to canonical CSIL CBOR bytes.
pub fn encode_share_delete_share_request(csil_v: &ShareID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the share_delete_share_request payload.
pub fn decode_share_delete_share_request(csil_data: &[u8]) -> Result<ShareID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

/// Encode the share_list_shares_by_resource_response payload to canonical CSIL CBOR bytes.
pub fn encode_share_list_shares_by_resource_response(csil_v: &Vec<Share>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_share(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the share_list_shares_by_resource_response payload.
pub fn decode_share_list_shares_by_resource_response(csil_data: &[u8]) -> Result<Vec<Share>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_share);
    csil_decode(&csil_root)
}

/// Encode the member_audit_list_audits_for_member_response payload to canonical CSIL CBOR bytes.
pub fn encode_member_audit_list_audits_for_member_response(csil_v: &Vec<MemberAudit>) -> Vec<u8> {
    cbor_encode(&cbor_enc_array(csil_v, |csil_elem| csil_enc_member_audit(csil_elem)))
}

/// Decode canonical CSIL CBOR bytes into the member_audit_list_audits_for_member_response payload.
pub fn decode_member_audit_list_audits_for_member_response(csil_data: &[u8]) -> Result<Vec<MemberAudit>, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = |csil_v| cbor_dec_array(csil_v, csil_dec_member_audit);
    csil_decode(&csil_root)
}

/// Encode the settings_get_settings_request payload to canonical CSIL CBOR bytes.
pub fn encode_settings_get_settings_request(csil_v: &HouseID) -> Vec<u8> {
    cbor_encode(&cbor_text(csil_v))
}

/// Decode canonical CSIL CBOR bytes into the settings_get_settings_request payload.
pub fn decode_settings_get_settings_request(csil_data: &[u8]) -> Result<HouseID, CsilCborError> {
    let csil_root = cbor_decode(csil_data)?;
    let csil_decode = cbor_as_text;
    csil_decode(&csil_root)
}

