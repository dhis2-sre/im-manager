package notification

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

type Service struct {
	repository *repository
}

func NewService(repository *repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) FindByUserID(ctx context.Context, userID uint) ([]model.Notification, error) {
	return s.repository.findByUserID(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, id, userID uint) error {
	return s.repository.markRead(ctx, id, userID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID uint) error {
	return s.repository.markAllRead(ctx, userID)
}

type subjectKind int

const (
	noSubject subjectKind = iota
	deploymentSubject
	databaseSubject
)

// subject is the deployment or database a notification is about, as named in its data.
type subject struct {
	kind subjectKind
	id   uint
}

// subjectOf reads the subject from the data of the kinds published by the deployment and database
// packages. Any other kind, or data without an id, has no subject.
func subjectOf(n model.Notification) subject {
	var kind subjectKind
	switch n.Kind {
	case "deployment":
		kind = deploymentSubject
	case "database-save", "filestore-backup":
		kind = databaseSubject
	default:
		return subject{}
	}

	var data struct {
		DeploymentID uint `json:"deploymentId"`
		DatabaseID   uint `json:"databaseId"`
	}
	if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
		return subject{}
	}

	id := data.DatabaseID
	if kind == deploymentSubject {
		id = data.DeploymentID
	}
	if id == 0 {
		return subject{}
	}
	return subject{kind: kind, id: id}
}

// Subjects are the deployments and databases notifications are about that still exist, by id. Each
// holds only the columns access is decided on.
type Subjects struct {
	deployments map[uint]*model.Deployment
	databases   map[uint]*model.Database
}

// FindSubjects looks up the subjects of the notifications with at most one query per kind of subject.
func (s *Service) FindSubjects(ctx context.Context, notifications []model.Notification) (Subjects, error) {
	var deploymentIDs, databaseIDs []uint
	for _, n := range notifications {
		switch subject := subjectOf(n); subject.kind {
		case deploymentSubject:
			deploymentIDs = append(deploymentIDs, subject.id)
		case databaseSubject:
			databaseIDs = append(databaseIDs, subject.id)
		}
	}

	subjects := Subjects{
		deployments: map[uint]*model.Deployment{},
		databases:   map[uint]*model.Database{},
	}

	if len(deploymentIDs) > 0 {
		slices.Sort(deploymentIDs)
		deployments, err := s.repository.findDeployments(ctx, slices.Compact(deploymentIDs))
		if err != nil {
			return Subjects{}, err
		}
		for i := range deployments {
			subjects.deployments[deployments[i].ID] = &deployments[i]
		}
	}

	if len(databaseIDs) > 0 {
		slices.Sort(databaseIDs)
		databases, err := s.repository.findDatabases(ctx, slices.Compact(databaseIDs))
		if err != nil {
			return Subjects{}, err
		}
		for i := range databases {
			subjects.databases[databases[i].ID] = &databases[i]
		}
	}

	return subjects, nil
}
