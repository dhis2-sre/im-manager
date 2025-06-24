package inspector

import (
	"context"
	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"log/slog"
	"strings"
	"time"
)

func NewInspector(logger *slog.Logger, service *group.Service, handlers ...Handler) *inspector {
	handlerMap := createHandlersByLabelMap(handlers)
	logger.Info("Handlers loaded", "count", slog.IntValue(len(handlers)))

	return &inspector{
		logger:       logger,
		groupService: service,
		handlerMap:   handlerMap,
	}
}

type inspector struct {
	logger       *slog.Logger
	handlerMap   map[string][]Handler
	groupService *group.Service
}

func (i inspector) Inspect(ctx context.Context) {
	for {
		time.Sleep(2 * time.Minute)

		i.logger.InfoContext(ctx, "Starting inspection...")

		groups, err := i.groupService.FindAll(ctx, &model.User{
			Groups: []model.Group{
				{
					Name: model.AdministratorGroupName,
				},
			},
		}, true)
		if err != nil {
			i.logger.ErrorContext(ctx, "failed to find groups", "error", err)
			continue
		}

		groupsWithDetails := make([]model.Group, len(groups))
		for index := range groups {
			groupWithDetails, err := i.groupService.FindWithDetails(ctx, groups[index].Name)
			if err != nil {
				i.logger.ErrorContext(ctx, "Failed to find group with details", "error", err)
				continue
			}
			groupsWithDetails[index] = *groupWithDetails
		}

		/* TODO: Only visit each namespace once...
		uniqueByNameAndNamespace := map[string]model.Group{}
		for _, group := range groupsWithDetails {
			var remote string
			if group.ClusterConfiguration != nil {
				remote = group.ClusterConfiguration.GroupName
			}
			key := fmt.Sprintf("%s-%s", group.Namespace, remote)
			uniqueByNameAndNamespace[key] = group
		}
		*/

		for _, group := range groups {
			i.logger.InfoContext(ctx, "Inspecting...", "name", group.Name, "namespace", group.Namespace)

			groupWithDetails, err := i.groupService.FindWithDetails(ctx, group.Name)
			if err != nil {
				i.logger.ErrorContext(ctx, "Failed to find group with details", "error", err)
				continue
			}

			kubernetesService, err := instance.NewKubernetesService(groupWithDetails.ClusterConfiguration)
			if err != nil {
				i.logger.ErrorContext(ctx, "Failed to create Kubernetes service", "error", err)
				continue
			}

			pods, err := kubernetesService.GetPods(groupWithDetails.Namespace)
			if err != nil {
				i.logger.ErrorContext(ctx, "failed to get pods", "error", err)
				continue
			}

			i.logger.InfoContext(ctx, "Inspecting pods", "count", slog.IntValue(len(pods)))
			for _, pod := range pods {
				i.logger.InfoContext(ctx, "Inspecting pod", "name", pod.Name, "namespace", pod.Namespace, "group", groupWithDetails.Name)
				for label := range pod.Labels {
					handlers, exists := i.handlerMap[label]
					if exists && strings.HasPrefix(label, "im-") {
						for _, h := range handlers {
							err := h.Handle(pod)
							if err != nil {
								i.logger.ErrorContext(ctx, "Failed to handle pod", "pod", pod.Name, "namespace", pod.Namespace, "error", err)
							}
						}
					}
				}
			}
		}

		i.logger.InfoContext(ctx, "Inspection ended")
	}
}

func createHandlersByLabelMap(handlers []Handler) map[string][]Handler {
	handlerMap := make(map[string][]Handler)
	for index := 0; index < len(handlers); index++ {
		key := handlers[index].Supports()
		handlerMap[key] = append(handlerMap[key], handlers[index])
	}
	return handlerMap
}
