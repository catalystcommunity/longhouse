package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// DependencyService manages the polymorphic dependency edges between tasks
// and projects. The data model is one-directional: a single row per edge,
// flowing from the dependent (the work item that has the dependency) to the
// dependency (the work item it requires). The reverse view ("what depends on
// X") is never stored — it is computed by reading the same table with the
// columns swapped. See csil/longhouse.csil + migration 000013.
type DependencyService struct{ Store store.Store }

func (s *DependencyService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("dependency", "AddDependency", csilrpc.Route(s.AddDependency, csil.DecodeDependencyAddDependencyRequest, csil.EncodeDependencyAddDependencyResponse))
	d.RegisterTyped("dependency", "RemoveDependency", csilrpc.Route(s.RemoveDependency, csil.DecodeDependencyRemoveDependencyRequest, csil.EncodeDependencyRemoveDependencyResponse))
	d.RegisterTyped("dependency", "GetDependencies", csilrpc.Route(s.GetDependencies, csil.DecodeDependencyGetDependenciesRequest, csil.EncodeDependencyGetDependenciesResponse))
}

// depNode is a resolved edge endpoint: exactly one of task/proj is set.
// It centralizes house/access/title/status so the forward and reverse
// directions share one resolution path.
type depNode struct {
	nodeType string
	task     *models.Task
	proj     *models.Project
}

func (n depNode) houseID() string {
	if n.task != nil {
		return n.task.HouseID
	}
	return n.proj.HouseID
}

func (n depNode) id() string {
	if n.task != nil {
		return n.task.TaskID
	}
	return n.proj.ProjectID
}

func (n depNode) access(ctx context.Context, pol *policy, g grantee) string {
	if n.task != nil {
		return pol.taskAccess(ctx, n.task, g)
	}
	return pol.projectAccess(ctx, n.proj, g)
}

// toCSIL builds the wire node, enriching title/status from the underlying
// row (neither lives on the edge — these are the API-only fields).
func (n depNode) toCSIL() csil.DependencyNode {
	out := csil.DependencyNode{Type: csil.DependencyNodeType(n.nodeType), Id: n.id()}
	if n.task != nil {
		out.Title = n.task.Title
		if n.task.Status != "" {
			st := n.task.Status
			out.Status = &st
		}
		return out
	}
	out.Title = n.proj.Name
	if n.proj.Status != "" {
		st := n.proj.Status
		out.Status = &st
	}
	return out
}

// loadNode fetches an edge endpoint. A soft-deleted task or a missing row
// returns (nil, nil): the caller treats that as "endpoint gone" and skips it
// (read) or errors (write), without leaking existence.
func (s *DependencyService) loadNode(ctx context.Context, nodeType, nodeID string) (*depNode, error) {
	switch nodeType {
	case models.DependencyTask:
		t, err := s.Store.GetTaskByID(ctx, nodeID)
		if err != nil || t == nil || t.DeletedAt != nil {
			return nil, nil
		}
		return &depNode{nodeType: nodeType, task: t}, nil
	case models.DependencyProject:
		p, err := s.Store.GetProjectByID(ctx, nodeID)
		if err != nil || p == nil {
			return nil, nil
		}
		return &depNode{nodeType: nodeType, proj: p}, nil
	}
	return nil, nil
}

// addDependency records that `dependent` depends on `dependency`. Requires
// edit on the dependent and read on the dependency; both ends must be in the
// same house; self-edges and cycle-creating edges are rejected.
func (s *DependencyService) AddDependency(ctx context.Context, in csil.DependencyRef) (csil.EmptyResponse, error) {
	depType, depID, onType, onID, err := decodeRef(in)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if depType == onType && depID == onID {
		return csil.EmptyResponse{}, csilrpc.BadRequest("a work item cannot depend on itself")
	}

	dependent, err := s.loadNode(ctx, depType, depID)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if dependent == nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("dependent not found")
	}
	dependency, err := s.loadNode(ctx, onType, onID)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if dependency == nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("dependency not found")
	}
	if dependent.houseID() != dependency.houseID() {
		return csil.EmptyResponse{}, csilrpc.BadRequest("both work items must belong to the same house")
	}

	ident, memberID, err := requireMemberForHouse(ctx, dependent.houseID())
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, dependent.houseID(), memberID)
	if !canEdit(dependent.access(ctx, pol, g)) {
		return csil.EmptyResponse{}, csilrpc.Forbidden("you need edit access to the dependent work item")
	}
	if !canRead(dependency.access(ctx, pol, g)) {
		// Don't confirm the existence of a dependency the caller can't see.
		return csil.EmptyResponse{}, csilrpc.NotFound("dependency not found")
	}

	// Cycle check (in the DB): if a path already runs from the dependency back
	// to the dependent, adding dependent->dependency would close a loop.
	cycles, err := s.Store.DependencyPathExists(ctx, onType, onID, depType, depID)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if cycles {
		return csil.EmptyResponse{}, csilrpc.Conflict("that dependency would create a cycle")
	}

	row := &models.Dependency{
		HouseID:        dependent.houseID(),
		DependentType:  depType,
		DependentID:    depID,
		DependencyType: onType,
		DependencyID:   onID,
	}
	if err := s.Store.AddDependency(ctx, row); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// removeDependency drops the edge. Requires edit on the dependent.
func (s *DependencyService) RemoveDependency(ctx context.Context, in csil.DependencyRef) (csil.EmptyResponse, error) {
	depType, depID, onType, onID, err := decodeRef(in)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	dependent, err := s.loadNode(ctx, depType, depID)
	if err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	if dependent == nil {
		return csil.EmptyResponse{}, csilrpc.NotFound("dependent not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, dependent.houseID())
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, dependent.houseID(), memberID)
	if !canEdit(dependent.access(ctx, pol, g)) {
		return csil.EmptyResponse{}, csilrpc.Forbidden("you need edit access to the dependent work item")
	}
	if err := s.Store.RemoveDependency(ctx, depType, depID, onType, onID); err != nil {
		return csil.EmptyResponse{}, csilrpc.Internal("internal error")
	}
	return csil.EmptyResponse{}, nil
}

// getDependencies returns both directions for one target. Requires read on
// the target; nodes the caller can't read (or that have gone away) are
// silently omitted from both lists.
func (s *DependencyService) GetDependencies(ctx context.Context, in csil.DependencyTarget) (csil.DependencyGraph, error) {
	nodeType := nodeTypeVal(in.Type)
	if !models.ValidDependencyNodeType(nodeType) || in.Id == "" {
		return csil.DependencyGraph{}, csilrpc.BadRequest("type and id are required")
	}

	target, err := s.loadNode(ctx, nodeType, in.Id)
	if err != nil {
		return csil.DependencyGraph{}, csilrpc.Internal("internal error")
	}
	if target == nil {
		return csil.DependencyGraph{}, csilrpc.NotFound("work item not found")
	}
	ident, memberID, err := requireMemberForHouse(ctx, target.houseID())
	if err != nil {
		return csil.DependencyGraph{}, err
	}
	pol := newPolicy(s.Store)
	g := pol.granteeFor(ctx, ident, target.houseID(), memberID)
	if !canRead(target.access(ctx, pol, g)) {
		return csil.DependencyGraph{}, csilrpc.NotFound("work item not found")
	}

	forward, err := s.Store.ListDependencies(ctx, nodeType, in.Id)
	if err != nil {
		return csil.DependencyGraph{}, csilrpc.Internal("internal error")
	}
	reverse, err := s.Store.ListDependents(ctx, nodeType, in.Id)
	if err != nil {
		return csil.DependencyGraph{}, csilrpc.Internal("internal error")
	}

	out := csil.DependencyGraph{
		// The "other end" of a forward edge is the dependency; of a reverse
		// edge, the dependent.
		Dependencies: s.resolveNodes(ctx, pol, g, forwardEnds(forward)),
		Dependents:   s.resolveNodes(ctx, pol, g, reverseEnds(reverse)),
	}
	return out, nil
}

// resolveNodes loads each (type,id), drops the gone and the unreadable, and
// returns wire nodes. Empty (never nil) so the response carries [] not null.
func (s *DependencyService) resolveNodes(ctx context.Context, pol *policy, g grantee, ends [][2]string) []csil.DependencyNode {
	out := make([]csil.DependencyNode, 0, len(ends))
	for _, e := range ends {
		n, err := s.loadNode(ctx, e[0], e[1])
		if err != nil || n == nil {
			continue
		}
		if !canRead(n.access(ctx, pol, g)) {
			continue
		}
		out = append(out, n.toCSIL())
	}
	return out
}

// forwardEnds extracts the dependency end (what the target depends on) of
// each stored edge.
func forwardEnds(rows []models.Dependency) [][2]string {
	out := make([][2]string, 0, len(rows))
	for i := range rows {
		out = append(out, [2]string{rows[i].DependencyType, rows[i].DependencyID})
	}
	return out
}

// reverseEnds extracts the dependent end (what depends on the target) of each
// stored edge.
func reverseEnds(rows []models.Dependency) [][2]string {
	out := make([][2]string, 0, len(rows))
	for i := range rows {
		out = append(out, [2]string{rows[i].DependentType, rows[i].DependentID})
	}
	return out
}

// ---- small helpers ----------------------------------------------------

// decodeRef validates a DependencyRef and returns the four parts.
func decodeRef(in csil.DependencyRef) (depType, depID, onType, onID string, err error) {
	depType = nodeTypeVal(in.DependentType)
	onType = nodeTypeVal(in.DependencyType)
	depID = in.DependentId
	onID = in.DependencyId
	if !models.ValidDependencyNodeType(depType) || !models.ValidDependencyNodeType(onType) {
		return "", "", "", "", csilrpc.BadRequest("dependent_type and dependency_type must be 'task' or 'project'")
	}
	if depID == "" || onID == "" {
		return "", "", "", "", csilrpc.BadRequest("dependent_id and dependency_id are required")
	}
	return depType, depID, onType, onID, nil
}

// nodeTypeVal returns the string value of the CSIL DependencyNodeType enum
// (a string-based type).
func nodeTypeVal(v csil.DependencyNodeType) string {
	return string(v)
}
