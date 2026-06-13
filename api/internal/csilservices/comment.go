package csilservices

import (
	"context"
	"errors"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"gorm.io/gorm"
)

// CommentService implements the CSIL CommentService over the dispatcher.
// Comments are discussion threads hung off a target resource (task, project,
// or event). Authorization is resource-based: the target's house_id is
// resolved from the DB and membership is checked against *that* house, so a
// client can't post into a house it doesn't belong to by spoofing house_id.
//
// Mutation rules (enforced here, not just in the SPA):
//   - create: any member of the target's house may comment.
//   - update: only the author may edit. Admins explicitly cannot edit other
//     members' comments — editing someone's words is different from removing
//     them.
//   - delete: the author OR a house admin may delete.
type CommentService struct{ Store store.Store }

const maxCommentBody = 10000

func (s *CommentService) Register(d *csilrpc.Dispatcher) {
	d.Register("comment", "ListComments", s.listComments)
	d.Register("comment", "GetComment", s.getComment)
	d.Register("comment", "CreateComment", s.createComment)
	d.Register("comment", "UpdateComment", s.updateComment)
	d.Register("comment", "DeleteComment", s.deleteComment)
}

func (s *CommentService) listComments(ctx context.Context, body []byte) (any, error) {
	var req csil.CommentListRequest
	if err := csilrpc.Decode(body, &req); err != nil {
		return nil, err
	}
	targetType, err := targetTypeStr(req.TargetType)
	if err != nil {
		return nil, err
	}
	if req.TargetId == "" {
		return nil, csilrpc.BadRequest("target_id is required")
	}
	houseID, err := s.houseForTarget(ctx, targetType, req.TargetId)
	if err != nil {
		return nil, err
	}
	if _, _, err := requireMemberForHouse(ctx, houseID); err != nil {
		return nil, err
	}
	limit, offset := normalizePaging(req.Limit, req.Offset)
	comments, err := s.Store.ListCommentsByTarget(ctx, targetType, req.TargetId, limit, offset)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return commentsToCSIL(comments), nil
}

func (s *CommentService) getComment(ctx context.Context, body []byte) (any, error) {
	var id csil.CommentID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	c, err := s.Store.GetCommentByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, csilrpc.NotFound("comment not found")
		}
		return nil, csilrpc.Internal("internal error")
	}
	if _, _, err := requireMemberForHouse(ctx, c.HouseID); err != nil {
		return nil, err
	}
	return commentToCSIL(c), nil
}

func (s *CommentService) createComment(ctx context.Context, body []byte) (any, error) {
	var in csil.Comment
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	targetType, err := targetTypeStr(in.TargetType)
	if err != nil {
		return nil, err
	}
	if in.TargetId == "" {
		return nil, csilrpc.BadRequest("target_id is required")
	}
	bodyText := strings.TrimSpace(in.Body)
	if bodyText == "" {
		return nil, csilrpc.BadRequest("body is required")
	}
	if len(bodyText) > maxCommentBody {
		return nil, csilrpc.BadRequest("body is too long")
	}
	// Resolve the target itself (house, title, watchers) — the house scopes
	// and authorizes the comment, the title/watchers feed the notification
	// snapshot + fan-out.
	ti, err := s.resolveTarget(ctx, targetType, in.TargetId)
	if err != nil {
		return nil, err
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, ti.houseID)
	if err != nil {
		return nil, err
	}
	c := &models.Comment{
		HouseID:    ti.houseID,
		MemberID:   callerMemberID,
		TargetType: targetType,
		TargetID:   in.TargetId,
		Body:       bodyText,
	}
	// Fan out a notification to every watcher of the target except the
	// author. The event snapshot is self-contained (no FK to the comment or
	// its target) and is written in the SAME transaction as the comment.
	recipients := dedupExcept(ti.watchers, callerMemberID)
	var event *models.NotificationEvent
	if len(recipients) > 0 {
		actorName := callerMemberID
		if m, err := s.Store.GetMemberByID(ctx, callerMemberID); err == nil && m != nil {
			actorName = memberDisplayName(m)
		}
		actor := callerMemberID
		tt := targetType
		tid := in.TargetId
		event = &models.NotificationEvent{
			HouseID:       ti.houseID,
			Kind:          "comment_created",
			ActorMemberID: &actor,
			ActorName:     actorName,
			TargetType:    &tt,
			TargetID:      &tid,
			TargetTitle:   ti.title,
			Body:          bodyText,
		}
	}
	if err := s.Store.CreateCommentWithNotifications(ctx, c, event, recipients); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return commentToCSIL(c), nil
}

func (s *CommentService) updateComment(ctx context.Context, body []byte) (any, error) {
	var in csil.Comment
	if err := csilrpc.Decode(body, &in); err != nil {
		return nil, err
	}
	if in.CommentId == "" {
		return nil, csilrpc.BadRequest("comment_id is required")
	}
	bodyText := strings.TrimSpace(in.Body)
	if bodyText == "" {
		return nil, csilrpc.BadRequest("body is required")
	}
	if len(bodyText) > maxCommentBody {
		return nil, csilrpc.BadRequest("body is too long")
	}
	existing, err := s.Store.GetCommentByID(ctx, string(in.CommentId))
	if err != nil {
		return nil, csilrpc.NotFound("comment not found")
	}
	_, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	// Author-only: an admin may delete but not rewrite someone else's comment.
	if callerMemberID != existing.MemberID {
		return nil, csilrpc.Forbidden("only the author may edit a comment")
	}
	existing.Body = bodyText
	if err := s.Store.UpdateComment(ctx, existing); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	return commentToCSIL(existing), nil
}

func (s *CommentService) deleteComment(ctx context.Context, body []byte) (any, error) {
	var id csil.CommentID
	if err := csilrpc.Decode(body, &id); err != nil {
		return nil, err
	}
	existing, err := s.Store.GetCommentByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return csil.EmptyResponse{}, nil // idempotent
		}
		return nil, csilrpc.Internal("internal error")
	}
	if existing.DeletedAt != nil {
		return csil.EmptyResponse{}, nil // idempotent
	}
	ident, callerMemberID, err := requireMemberForHouse(ctx, existing.HouseID)
	if err != nil {
		return nil, err
	}
	if callerMemberID != existing.MemberID {
		if _, err := requireRole(ident, existing.HouseID, "admin"); err != nil {
			return nil, csilrpc.Forbidden("only the author or a house admin may delete a comment")
		}
	}
	opID, err := s.Store.NewID(ctx)
	if err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	if err := s.Store.SoftDeleteComment(ctx, string(id), callerMemberID, opID); err != nil {
		return nil, csilrpc.Internal("internal error")
	}
	annotateDelete(ctx, existing.HouseID, "comment", existing.CommentID, opID, existing)
	return csil.EmptyResponse{}, nil
}

// ---- helpers ----------------------------------------------------------

// targetInfo is what a comment target (task/project/event) contributes to
// authorization and the notification snapshot: the owning house, a display
// title, and the set of members watching it (who should be notified).
type targetInfo struct {
	houseID  string
	title    string
	watchers []string
}

// resolveTarget loads a comment target and derives its house, title, and
// watcher set. This is the authorization anchor for every comment op and the
// recipient source for notification fan-out.
//
// Watchers today are derived from existing relationships:
//   - task:    the owner + all assignees (assignees are watchers by
//     definition).
//   - project: all project members + owners.
//   - event:   none yet (events have no watcher model).
//
// An explicit watch/subscribe table could later be unioned in here without
// touching the notification model.
func (s *CommentService) resolveTarget(ctx context.Context, targetType, targetID string) (*targetInfo, error) {
	switch targetType {
	case "task":
		t, err := s.Store.GetTaskByID(ctx, targetID)
		if err != nil {
			return nil, csilrpc.NotFound("task not found")
		}
		watchers := []string{t.OwnerMemberID}
		if assignees, err := s.Store.ListTaskAssignees(ctx, targetID); err == nil {
			for _, a := range assignees {
				watchers = append(watchers, a.MemberID)
			}
		}
		return &targetInfo{houseID: t.HouseID, title: t.Title, watchers: watchers}, nil
	case "project":
		p, err := s.Store.GetProjectByID(ctx, targetID)
		if err != nil {
			return nil, csilrpc.NotFound("project not found")
		}
		var watchers []string
		if members, err := s.Store.ListProjectMembers(ctx, targetID); err == nil {
			for _, m := range members {
				watchers = append(watchers, m.MemberID)
			}
		}
		if owners, err := s.Store.ListProjectOwners(ctx, targetID); err == nil {
			for _, o := range owners {
				watchers = append(watchers, o.MemberID)
			}
		}
		return &targetInfo{houseID: p.HouseID, title: p.Name, watchers: watchers}, nil
	case "event":
		e, err := s.Store.GetEventByID(ctx, targetID)
		if err != nil {
			return nil, csilrpc.NotFound("event not found")
		}
		return &targetInfo{houseID: e.HouseID, title: e.Title}, nil
	default:
		return nil, csilrpc.BadRequest("invalid target_type")
	}
}

// houseForTarget resolves just the owning house of a comment target, for the
// read paths (list/get) that don't need the title or watchers.
func (s *CommentService) houseForTarget(ctx context.Context, targetType, targetID string) (string, error) {
	ti, err := s.resolveTarget(ctx, targetType, targetID)
	if err != nil {
		return "", err
	}
	return ti.houseID, nil
}

// dedupExcept returns the unique non-empty ids in order, skipping `except`.
func dedupExcept(ids []string, except string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || id == except || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// memberDisplayName mirrors the SPA's display-name precedence so notification
// snapshots read the same as the rest of the UI.
func memberDisplayName(m *models.Member) string {
	if name := strings.TrimSpace(m.DisplayName); name != "" {
		return name
	}
	if m.LinkkeysUserID != "" {
		return m.LinkkeysUserID
	}
	return m.MemberID
}

// targetTypeStr coerces the opaque CSIL TargetType (generated as interface{})
// to one of the known string values, rejecting anything else.
func targetTypeStr(t csil.TargetType) (string, error) {
	s, ok := t.(string)
	if !ok {
		return "", csilrpc.BadRequest("target_type is required")
	}
	switch s {
	case "task", "project", "event":
		return s, nil
	default:
		return "", csilrpc.BadRequest("invalid target_type")
	}
}
