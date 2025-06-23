package inspector

import (
	"context"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"log/slog"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/model"
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
		time.Sleep(5 * time.Minute)

		i.logger.Info("Starting inspection...")

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

		uniqueByNameNamespace := map[string]model.Group{}
		for _, group := range groups {
			uniqueByNameNamespace[group.Name+group.Namespace] = group
		}

		for _, v := range uniqueByNameNamespace {
			kubernetesService, err := instance.NewKubernetesService(v.ClusterConfiguration)
			if err != nil {
				i.logger.ErrorContext(ctx, "Failed to create Kubernetes service", "error", err)
				continue
			}
			pods, err := kubernetesService.GetPods(v.Namespace)
			if err != nil {
				i.logger.ErrorContext(ctx, "failed to get pods", "error", err)
				continue
			}

			i.logger.Info("Inspecting pods", "count", slog.IntValue(len(pods)))
			for _, pod := range pods {
				i.logger.Info("Inspecting pod", "pod", pod.Name)
				for label := range pod.Labels {
					handlers, exists := i.handlerMap[label]
					if exists && strings.HasPrefix(label, "im-") {
						for _, h := range handlers {
							err := h.Handle(pod)
							if err != nil {
								i.logger.Error("Failed to handle pod", "pod", pod.Name, "namespace", pod.Namespace, "error", err.Error())
							}
						}
					}
				}
			}
		}
		i.logger.Info("Inspection ended")
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
