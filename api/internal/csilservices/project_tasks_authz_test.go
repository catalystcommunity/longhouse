package csilservices

import (
	"context"
	"errors"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type fakeProjectLookupStore struct {
	store.Store
	project *models.Project
}

func (f *fakeProjectLookupStore) GetProjectByID(context.Context, string) (*models.Project, error) {
	return f.project, nil
}

func TestListProjectTasksRejectsProjectInAnotherHouse(t *testing.T) {
	st := &fakeProjectLookupStore{project: &models.Project{ProjectID: "project-1", HouseID: "house-2"}}
	svc := &ProjectService{Store: st}
	_, err := svc.ListProjectTasks(adminContext(), csil.ProjectScopedListRequest{HouseId: "house-1", ProjectId: "project-1"})
	var serviceErr *csilrpc.Error
	if !errors.As(err, &serviceErr) || serviceErr.Code != 404 {
		t.Fatalf("cross-house list: got %v, want 404", err)
	}
}
