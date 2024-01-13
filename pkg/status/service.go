package status

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"log"
	"strconv"
	"time"
)

func NewService(groupService groupService, instanceService instanceService) *instanceStatus {
	return &instanceStatus{
		groupService:          groupService,
		instanceService:       instanceService,
		groupsWithDeployments: make([]instance.GroupsWithDeployments, 0),
	}
}

type groupService interface {
	FindAll(user *model.User, deployable bool) ([]model.Group, error)
}

type instanceService interface {
	FindDeployments(user *model.User) ([]instance.GroupsWithDeployments, error)
	FindDeploymentById(id uint) (*model.Deployment, error)
	FindDeploymentInstanceById(id uint) (*model.DeploymentInstance, error)
	GetStatus(instance *model.DeploymentInstance) (instance.InstanceStatus, error)
	PodStatus(pod *v1.Pod) (instance.InstanceStatus, error)
}

type instanceStatus struct {
	groupService          groupService
	instanceService       instanceService
	groupsWithDeployments []instance.GroupsWithDeployments
}

func (i *instanceStatus) FindDeployments(user *model.User) ([]instance.GroupsWithDeployments, error) {
	log.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	indent, _ := json.MarshalIndent(i.groupsWithDeployments, "", "  ")
	log.Println(string(indent))
	log.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	if user.IsAdministrator() {
		return i.groupsWithDeployments, nil
	}

	groups := findAllFromUser(user, true)
	log.Println("groups:", groups)
	groupsWithDeployments := make([]instance.GroupsWithDeployments, len(groups))
	for _, group := range i.groupsWithDeployments {
		if user.IsMemberOf(group.Name) {
			groupsWithDeployments = append(groupsWithDeployments, group)
		}
	}

	return groupsWithDeployments, nil
}

// Listen
func (i *instanceStatus) Listen() {
	fakeAdministrator := &model.User{
		Groups: []model.Group{
			{
				Name: model.AdministratorGroupName,
			},
			{
				Name: "whoami",
			},
		},
	}

	groupsWithDeployments, err := i.instanceService.FindDeployments(fakeAdministrator)
	if err != nil {
		log.Fatal(err)
	}
	i.groupsWithDeployments = groupsWithDeployments

	log.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	indent, _ := json.MarshalIndent(groupsWithDeployments, "", "  ")
	log.Println(string(indent))
	log.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")

	groups, err := i.groupService.FindAll(fakeAdministrator, true)
	if err != nil {
		log.Fatal(err)
	}

	for _, group := range groups {
		log.Println("group:", group.Name)
		g := group
		go func() {
			kubernetesService, err := instance.NewKubernetesService(g.ClusterConfiguration)
			if err != nil {
				log.Fatal(err)
			}

			client := kubernetesService.GetClient()
			options := metav1.ListOptions{
				LabelSelector: "im=true",
			}
			w, err := client.CoreV1().Pods(g.Name).Watch(context.Background(), options)
			if err != nil {
				log.Fatal(err)
			}

			for event := range w.ResultChan() {
				fmt.Printf("Type: %v\n", event.Type)

				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					// TODO: Log entire event.Object... Entire event?
					log.Fatal("unexpected type")
				}

				fmt.Printf("Name: %v\n", pod.Name)
				fmt.Printf("Namespace: %v\n", pod.Namespace)
				status, err := i.instanceService.PodStatus(pod)
				if err != nil {
					log.Fatal(err)
				}

				idStr := pod.Labels["im-instance-id"]
				id, err := strconv.Atoi(idStr)
				if err != nil {
					log.Fatal(err)
				}

				in, err := i.instanceService.FindDeploymentInstanceById(uint(id))
				if err != nil {
					log.Println(err)
					continue
				}

				if event.Type == watch.Deleted {
					for _, group := range i.groupsWithDeployments {
						for j, deployment := range group.Deployments {
							for k, instance := range deployment.Instances {
								if instance.ID == uint(id) {
									deployment.Instances = append(deployment.Instances[:k], deployment.Instances[k+1:]...)
									log.Println("Removing:", id)
									// TODO: Send to channel...
								}
							}
							// TODO: Delete deployment if needed
							// TODO: Sleep a bit here?
							time.Sleep(3 * time.Second)
							_, err := i.instanceService.FindDeploymentById(deployment.ID)
							if errdef.IsNotFound(err) {
								group.Deployments = append(group.Deployments[:j], group.Deployments[j+1:]...)
							}
						}
					}
					// TODO: Send to channel...
					continue
				}
				/*
					status, err := i.instanceService.GetStatus(in)
					if err != nil {
						log.Println(err)
					}
					log.Println(status)
				*/
				in.Status = string(status)

				if event.Type == watch.Added {
					deployment, ok := i.findDeployment(in.DeploymentID)
					if !ok {
						d, err := i.instanceService.FindDeploymentById(in.DeploymentID)
						if err != nil {
							// TODO: Handle error
							log.Println(err)
						}
						for _, group := range i.groupsWithDeployments {
							if group.Name == d.GroupName {
								group.Deployments = append(group.Deployments, d)
							}
						}
						deployment = d
					}

					for i, deploymentInstance := range deployment.Instances {
						if deploymentInstance.ID == in.ID {
							deployment.Instances[i].Status = string(status)
						}
					}
				}

				if event.Type == watch.Modified || event.Type == watch.Error {
					for _, group := range i.groupsWithDeployments {
						for _, deployment := range group.Deployments {
							for _, deploymentInstance := range deployment.Instances {
								if deploymentInstance.ID == uint(id) {
									deploymentInstance.Status = string(status)
									log.Println("Updating")
									// TODO: Send to channel...
								}
							}

						}
					}
				}
			}
		}()
	}
}

func (i *instanceStatus) findDeployment(id uint) (*model.Deployment, bool) {
	for _, group := range i.groupsWithDeployments {
		for _, deployment := range group.Deployments {
			if deployment.ID == id {
				return deployment, true
			}
		}
	}
	return nil, false
}

// TODO: The below function is also defined in group.repository... Remove duplicate
func findAllFromUser(user *model.User, deployable bool) []model.Group {
	var allGroups []model.Group
	allGroups = append(allGroups, user.Groups...)
	allGroups = append(allGroups, user.AdminGroups...)

	if deployable {
		index := 0
		for _, group := range allGroups {
			if group.Deployable {
				allGroups[index] = group
				index++
			}
		}
		allGroups = allGroups[:index]
	}

	groupsByName := make(map[string]model.Group)
	for _, group := range allGroups {
		groupsByName[group.Name] = group
	}

	return maps.Values(groupsByName)
}
