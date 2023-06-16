package instance_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-gonic/gin"
	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/k3s"
	"github.com/stretchr/testify/require"
)

func TestInstanceHandler(t *testing.T) {
	// TODO(ivo) Create an inttest.SetupK3s once we have one or more working tests
	// it should be designed in the same way as the other helpers. Fail on any error, cleanup any
	// resources using t.Cleanup and return a configured client to interact with k3s
	// like
	// 	 client, err := kubernetes.NewForConfig(kubeconfig)
	// 	 require.NoError(t, err)
	//   return client
	// My approach was that if we have a couple of test we see the typical interactions we have
	// using the client in assertions and special setup. We can then return our own client which is
	// a wrapper of the actual client while exporting the actual client for any non-standard use
	// case.
	p := k3s.Preset(
		k3s.WithVersion("v1.26.5-k3s1"),
		func(p *k3s.P) {
			p.K3sServerFlags = []string{"--debug"} // TODO(ivo) remove this flag before merging! useful for debugging
		},
	)
	c, err := gnomock.Start(
		p,
		gnomock.WithContainerName("k3s"),
		gnomock.WithDebugMode(),
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gnomock.Stop(c))
	}()

	// TODO(ivo) I am only writing the k3sconfig to /tmp so I can debug any issues. Once
	// we have working tests it should either not even be written to disk (as the group keeps it in
	// memory) or to t.TempDir() if our inttest helper client needs it on disk.
	// k3sConfigDir := t.TempDir()
	k3sConfig, err := k3s.ConfigBytes(c)
	require.NoError(t, err)
	// k3sConfigFile := k3sConfigDir + "/k3sconfig"
	// err = ioutil.WriteFile(k3sConfigFile, k3sConfig, 0o600)
	err = ioutil.WriteFile("/tmp/k3sconfig", k3sConfig, 0o666)
	require.NoError(t, err)
	t.Setenv("KUBECONFIG", "/tmp/k3sconfig")

	db := inttest.SetupDB(t)
	stackRepository := stack.NewRepository(db)
	stackService := stack.NewService(stackRepository)

	instanceRepo := instance.NewRepository(db, "")
	helmfileService := instance.NewHelmfileService("../../stacks", stackService, "test")
	groupService := groupService{groupName: "test", groupHostname: "some", k8sConfig: k3sConfig}
	instanceService := instance.NewService(instanceRepo, groupService, stackService, helmfileService)

	err = stack.LoadStacks("../../stacks", stackService)
	require.NoError(t, err, "failed to load stacks")

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		instanceHandler := instance.NewHandler(userService{}, groupService, instanceService)
		instance.Routes(engine, func(ctx *gin.Context) {
			ctx.Set("user", &model.User{
				Email: "user1@dhis2.org",
				Groups: []model.Group{
					{
						Name: "test",
					},
				},
			})
		}, instanceHandler)
	})

	// TODO(ivo) remove these subtests before merging! they are helpful for debugging :)
	// t.Run("GnoMockExampleTest", func(t *testing.T) {
	// 	kubeconfig, err := k3s.Config(c)
	// 	require.NoError(t, err)
	// 	kClient, err := kubernetes.NewForConfig(kubeconfig)
	// 	require.NoError(t, err)
	//
	// 	ctx := context.Background()
	//
	// 	pods, err := kClient.CoreV1().Pods(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{})
	// 	require.NoError(t, err)
	// 	require.Empty(t, pods.Items)
	//
	// 	pod := &v1.Pod{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      "gnomock",
	// 			Namespace: metav1.NamespaceDefault,
	// 		},
	// 		Spec: v1.PodSpec{
	// 			Containers: []v1.Container{
	// 				{
	// 					Name:  "gnomock",
	// 					Image: "docker.io/orlangure/gnomock-test-image",
	// 				},
	// 			},
	// 			RestartPolicy: v1.RestartPolicyNever,
	// 		},
	// 	}
	//
	// 	_, err = kClient.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, pod, metav1.CreateOptions{})
	// 	require.NoError(t, err)
	//
	// 	pods, err = kClient.CoreV1().Pods(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{})
	// 	require.NoError(t, err)
	// 	require.Len(t, pods.Items, 1)
	// 	require.Equal(t, "gnomock", pods.Items[0].Name)
	// }	// TODO(ivo) remove these subtests before merging! they are helpful for debugging :)
	// t.Run("kubectl", func(t *testing.T) {
	// 	cmd := exec.Command("/usr/bin/kubectl", "get", "pods", "--all-namespaces", "-v6", "--kubeconfig", "/tmp/k3sconfig")
	// 	out, err := cmd.Output()
	// 	t.Logf("kubectl get pods output\n%s", out)
	// 	require.NoErrorf(t, err, "%s", string(out))
	// })

	// TODO(ivo) remove these subtests before merging! they are helpful for debugging :)
	// t.Run("helm", func(t *testing.T) {
	// 	cmd := exec.Command("/usr/bin/helm", "ls", "--all-namespaces", "--kubeconfig", "/tmp/k3sconfig")
	// 	out, err := cmd.Output()
	// 	t.Logf("helm ls output\n%s", out)
	// 	require.NoErrorf(t, err, "%s", string(out))
	// })

	t.Run("Deploy", func(t *testing.T) {
		var instance model.Instance
		client.PostJSON(t, "/instances", strings.NewReader(`{
			"name": "test-whoami",
			"groupName": "test",
			"stackName": "whoami-go"
		}`), &instance, inttest.WithAuthToken("sometoken"))

		// TOOD(ivo) remove before merging! helps in debugging as this will wait forever so you can
		// interact with k3s and whoami
		// var wg sync.WaitGroup
		// wg.Add(1)
		// wg.Wait()

		// TODO(ivo) assert whoami is up by making a request like
		//
		// body := client.Get(t, "/whoami")

		// require.Equal(t, "whatever whoami returns", string(body))
	})
}

type userService struct{}

func (us userService) FindById(id uint) (*model.User, error) {
	return nil, nil
}

type groupService struct {
	groupName     string
	groupHostname string
	k8sConfig     []byte
}

func (gs groupService) Find(name string) (*model.Group, error) {
	return &model.Group{
		Name:     gs.groupName,
		Hostname: gs.groupHostname,
		// TODO(ivo) this is a remaining issue. We should rely on the groups.KubernetesConfiguration
		// I am getting unhandled error: "sops metadata not found" if I add this config
		// this is likely as I have not configured sops (nor mocked it).
		// ClusterConfiguration: &model.ClusterConfiguration{
		// 	GroupName:               gs.groupName,
		// 	KubernetesConfiguration: gs.k8sConfig,
		// },
	}, nil
}
