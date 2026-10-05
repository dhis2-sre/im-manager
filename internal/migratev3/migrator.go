package migratev3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

// Migrator runs the phases of the migration. Each phase acts on every selected deployment that is
// at the step the phase starts from, runs them Concurrency at a time, and records a deployment's
// failure on its state without stopping the others.
type Migrator struct {
	Logger        *slog.Logger
	DB            *gorm.DB
	EncryptionKey string
	Snapshotter   Snapshotter
	API           *API
	Concurrency   int
	// Deployments limits a phase to these deployment ids, for a canary run. Empty means all.
	Deployments []uint
	// DeployTimeout bounds how long the deploy phase waits for one deployment to be seeded and
	// running, which for the largest databases is hours rather than minutes.
	DeployTimeout time.Duration

	mu       sync.Mutex
	clusters map[string]*Cluster
}

func (m *Migrator) selected(deploymentID uint) bool {
	return len(m.Deployments) == 0 || slices.Contains(m.Deployments, deploymentID)
}

func (m *Migrator) cluster(group *model.Group) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d/%s", group.Cluster.ID, group.Namespace)
	if cluster, ok := m.clusters[key]; ok {
		return cluster, nil
	}
	cluster, err := NewCluster(group.Cluster, group.Namespace)
	if err != nil {
		return nil, err
	}
	if m.clusters == nil {
		m.clusters = map[string]*Cluster{}
	}
	m.clusters[key] = cluster
	return cluster, nil
}

func (m *Migrator) group(name string) (*model.Group, error) {
	var group model.Group
	if err := m.DB.Preload("Cluster").First(&group, "name = ?", name).Error; err != nil {
		return nil, fmt.Errorf("failed to load group %q: %v", name, err)
	}
	return &group, nil
}

// each runs work for every item, at most Concurrency at a time, and joins the failures.
func each[T any](concurrency int, items []T, work func(T) error) error {
	if concurrency < 1 {
		concurrency = 1
	}
	semaphore := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, item := range items {
		semaphore <- struct{}{}
		wg.Go(func() {
			defer func() { <-semaphore }()
			if err := work(item); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// PlannedDeployment is a legacy deployment with what the plan learnt about it on the cluster.
type PlannedDeployment struct {
	LegacyDeployment
	DatabaseSize int64
	Problem      string
}

// Plan reads which deployments would be migrated, and fails if any of them cannot be. It changes
// nothing, so it runs against the schema of the version before 3.0, while that version still
// serves.
func (m *Migrator) Plan(ctx context.Context) ([]PlannedDeployment, []Unsupported, error) {
	legacy, unsupported, err := FindLegacyDeployments(m.DB, m.EncryptionKey)
	if err != nil {
		return nil, nil, err
	}
	unsupported = slices.DeleteFunc(unsupported, func(u Unsupported) bool { return !m.selected(u.DeploymentID) })

	var planned []PlannedDeployment
	var mu sync.Mutex
	err = each(m.Concurrency, legacy, func(d LegacyDeployment) error {
		if !m.selected(d.Deployment.ID) {
			return nil
		}
		p := PlannedDeployment{LegacyDeployment: d}
		p.DatabaseSize, p.Problem = m.inspect(ctx, d)
		mu.Lock()
		planned = append(planned, p)
		mu.Unlock()
		return nil
	})
	sort.Slice(planned, func(i, j int) bool { return planned[i].DatabaseSize > planned[j].DatabaseSize })
	return planned, unsupported, err
}

// inspect checks the releases the migration acts on exist, and measures the database.
func (m *Migrator) inspect(ctx context.Context, d LegacyDeployment) (int64, string) {
	cluster, err := m.cluster(d.Group)
	if err != nil {
		return 0, err.Error()
	}
	releases := []string{d.ReleaseName(), databaseRelease(d)}
	if d.Minio != nil {
		releases = append(releases, minioRelease(d))
	}
	if d.PgAdmin != nil {
		releases = append(releases, pgAdminRelease(d))
	}
	for _, release := range releases {
		exists, err := cluster.releaseExists(ctx, release)
		if err != nil {
			return 0, err.Error()
		}
		if !exists {
			return 0, fmt.Sprintf("release %q is missing", release)
		}
	}
	statefulSet, err := cluster.statefulSet(ctx, databaseStatefulSet(d))
	if err != nil {
		return 0, err.Error()
	}
	if statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas == 0 {
		// A paused instance is measured during the migration, once its database runs.
		return 0, ""
	}
	size, err := DatabaseSize(ctx, cluster, d)
	if err != nil {
		return 0, fmt.Sprintf("failed to measure the database: %v", err)
	}
	return size, ""
}

// Migrate snapshots every selected legacy deployment, rewrites its rows to dhis2-v2 and removes the
// old releases that would collide with it. It runs the version 3.0 schema migrations first, so it
// must run while no instance manager serves: the version before 3.0 would act on rewritten rows it
// does not know, and version 3.0 cannot read rows that are not rewritten yet.
func (m *Migrator) Migrate(ctx context.Context) error {
	if err := migrateStateTable(m.DB); err != nil {
		return err
	}
	planned, unsupported, err := m.Plan(ctx)
	if err != nil {
		return err
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("%d deployment(s) cannot be migrated, run plan to list them", len(unsupported))
	}

	// Largest first, so the longest dumps overlap everything else instead of trailing at the end.
	failures := each(m.Concurrency, planned, func(p PlannedDeployment) error {
		if p.Problem != "" {
			return fmt.Errorf("deployment %d (%s): %s", p.Deployment.ID, p.Deployment.Name, p.Problem)
		}
		return m.migrate(ctx, p.LegacyDeployment)
	})

	// Deployments rewritten by an earlier run whose release failed are no longer legacy, so they
	// are picked up from their state.
	states, err := findStates(m.DB, StepRewritten)
	if err != nil {
		return errors.Join(failures, err)
	}
	states = slices.DeleteFunc(states, func(s State) bool { return !m.selected(s.DeploymentID) })
	return errors.Join(failures, each(m.Concurrency, states, func(state State) error {
		return m.release(ctx, &state)
	}))
}

func (m *Migrator) migrate(ctx context.Context, d LegacyDeployment) error {
	state, err := findState(m.DB, d.Deployment.ID)
	if err != nil {
		return err
	}
	if state == nil {
		state = &State{
			DeploymentID:       d.Deployment.ID,
			DeploymentName:     d.Deployment.Name,
			GroupName:          d.Deployment.GroupName,
			CoreInstanceID:     d.Core.ID,
			HasMinio:           d.Minio != nil,
			OriginalDatabaseID: d.Database.Parameters["DATABASE_ID"].Value,
		}
		if d.PgAdmin != nil {
			state.PgAdminInstanceID = d.PgAdmin.ID
		}
	}

	logger := m.Logger.With("deploymentId", d.Deployment.ID, "deployment", d.Deployment.Name, "group", d.Deployment.GroupName)
	if state.Step == StepNone {
		logger.InfoContext(ctx, "Snapshotting")
		start := time.Now()
		if err := m.snapshot(ctx, d, state); err != nil {
			return recordFailure(m.DB, state, fmt.Errorf("deployment %d (%s): snapshot: %v", d.Deployment.ID, d.Deployment.Name, err))
		}
		logger.InfoContext(ctx, "Snapshotted", "databaseId", state.SnapshotDatabaseID, "duration", time.Since(start).Round(time.Second))
	}
	if state.Step == StepSnapshotted {
		if err := Rewrite(m.DB, m.EncryptionKey, d, state); err != nil {
			return recordFailure(m.DB, state, fmt.Errorf("deployment %d (%s): rewrite: %v", d.Deployment.ID, d.Deployment.Name, err))
		}
		logger.InfoContext(ctx, "Rewritten to dhis2-v2")
	}
	if state.Step == StepRewritten {
		return m.release(ctx, state)
	}
	return nil
}

// snapshot stops the old core so nothing writes while the database and file store are saved. If
// saving fails, the core is started again and the deployment is left as it was.
func (m *Migrator) snapshot(ctx context.Context, d LegacyDeployment, state *State) error {
	cluster, err := m.cluster(d.Group)
	if err != nil {
		return err
	}

	// A paused instance has every workload scaled to zero, the database included.
	statefulSet, err := cluster.statefulSet(ctx, databaseStatefulSet(d))
	if err != nil {
		return err
	}
	state.WasPaused = statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas == 0
	if state.WasPaused {
		if err := cluster.scaleStatefulSet(ctx, databaseStatefulSet(d), 1); err != nil {
			return err
		}
		if d.Minio != nil {
			if err := m.scaleInstance(ctx, cluster, d.Minio, 1); err != nil {
				return m.undoSnapshot(ctx, cluster, d, state, nil, err)
			}
		}
	}

	key := func(suffix string) string { return d.Deployment.GroupName + "/" + snapshotName(d) + suffix }
	var filestoreKey string
	var filestoreSize int64
	// The filesystem file store lives in the core pod, so it is read before the core stops.
	if d.StorageType() == "filesystem" {
		size, _, err := m.Snapshotter.ArchiveFilestore(ctx, cluster, d, key(".fs.tar.gz"))
		if err != nil {
			return m.undoSnapshot(ctx, cluster, d, state, nil, err)
		}
		filestoreKey, filestoreSize = key(".fs.tar.gz"), size
	}

	cores, err := cluster.deploymentsBySelector(ctx, instanceSelector(d.Core))
	if err != nil {
		return m.undoSnapshot(ctx, cluster, d, state, nil, err)
	}
	stopped := map[string]int32{}
	for _, core := range cores {
		// Kubernetes defaults unset replicas to one.
		stopped[core.Name] = 1
		if core.Spec.Replicas != nil {
			stopped[core.Name] = *core.Spec.Replicas
		}
		if err := cluster.scaleDeployment(ctx, core.Name, 0); err != nil {
			return m.undoSnapshot(ctx, cluster, d, state, stopped, err)
		}
	}
	if err := cluster.waitForNoPods(ctx, instanceSelector(d.Core), 10*time.Minute); err != nil {
		return m.undoSnapshot(ctx, cluster, d, state, stopped, err)
	}

	databaseSize, err := m.Snapshotter.DumpDatabase(ctx, cluster, d, key(".sql.gz"))
	if err != nil {
		return m.undoSnapshot(ctx, cluster, d, state, stopped, err)
	}
	if d.StorageType() == "minio" {
		size, _, err := m.Snapshotter.ArchiveFilestore(ctx, cluster, d, key(".fs.tar.gz"))
		if err != nil {
			return m.undoSnapshot(ctx, cluster, d, state, stopped, err)
		}
		filestoreKey, filestoreSize = key(".fs.tar.gz"), size
	}

	state.SnapshotDatabaseID, err = RecordSnapshot(m.DB, m.Snapshotter.Bucket, d, key(".sql.gz"), databaseSize, filestoreKey, filestoreSize)
	if err != nil {
		return m.undoSnapshot(ctx, cluster, d, state, stopped, err)
	}
	state.Step = StepSnapshotted
	state.Error = ""
	return saveState(m.DB, state)
}

func (m *Migrator) undoSnapshot(ctx context.Context, cluster *Cluster, d LegacyDeployment, state *State, stopped map[string]int32, cause error) error {
	ctx = context.WithoutCancel(ctx)
	errs := []error{cause}
	for name, replicas := range stopped {
		errs = append(errs, cluster.scaleDeployment(ctx, name, replicas))
	}
	if state.WasPaused {
		errs = append(errs, cluster.scaleStatefulSet(ctx, databaseStatefulSet(d), 0))
		if d.Minio != nil {
			errs = append(errs, m.scaleInstance(ctx, cluster, d.Minio, 0))
		}
	}
	return errors.Join(errs...)
}

// scaleInstance scales every deployment of the instance, found by its labels rather than a name
// the chart version may have chosen differently.
func (m *Migrator) scaleInstance(ctx context.Context, cluster *Cluster, instance *model.DeploymentInstance, replicas int32) error {
	deployments, err := cluster.deploymentsBySelector(ctx, instanceSelector(instance))
	if err != nil {
		return err
	}
	if len(deployments) == 0 {
		return fmt.Errorf("no deployment of instance %d", instance.ID)
	}
	for _, deployment := range deployments {
		if err := cluster.scaleDeployment(ctx, deployment.Name, replicas); err != nil {
			return err
		}
	}
	return nil
}

// release removes the old releases whose resource names dhis2-v2 reuses: the core, which shares
// the release name, MinIO, which shares every name including its claim, and pgAdmin. The old
// database collides with nothing, so it is only scaled to zero and kept until dhis2-v2 runs.
func (m *Migrator) release(ctx context.Context, state *State) error {
	group, err := m.group(state.GroupName)
	if err != nil {
		return recordFailure(m.DB, state, err)
	}
	cluster, err := m.cluster(group)
	if err != nil {
		return recordFailure(m.DB, state, err)
	}
	d := LegacyDeployment{Deployment: &model.Deployment{ID: state.DeploymentID, Name: state.DeploymentName}, Group: group}

	fail := func(err error) error {
		return recordFailure(m.DB, state, fmt.Errorf("deployment %d (%s): release: %v", state.DeploymentID, state.DeploymentName, err))
	}
	if err := cluster.uninstall(ctx, d.ReleaseName()); err != nil {
		return fail(err)
	}
	// A filesystem file store's claim can outlive the core release. The dhis2-v2 instance keeps the
	// core's instance id, so this runs before anything of it exists.
	if err := cluster.deletePVCsBySelector(ctx, fmt.Sprintf("im-instance-id=%d", state.CoreInstanceID)); err != nil {
		return fail(err)
	}
	if state.PgAdminInstanceID != 0 {
		if err := cluster.uninstall(ctx, pgAdminRelease(d)); err != nil {
			return fail(err)
		}
	}
	if state.HasMinio {
		if err := cluster.uninstall(ctx, minioRelease(d)); err != nil {
			return fail(err)
		}
		// The minio stack annotated its claim to outlive the release.
		if err := cluster.deletePVC(ctx, minioRelease(d), 10*time.Minute); err != nil {
			return fail(err)
		}
	}
	if err := cluster.scaleStatefulSet(ctx, databaseStatefulSet(d), 0); err != nil {
		return fail(err)
	}

	state.Step = StepReleased
	state.Error = ""
	m.Logger.InfoContext(ctx, "Released the old releases", "deploymentId", state.DeploymentID, "deployment", state.DeploymentName)
	return saveState(m.DB, state)
}

// Deploy deploys every released deployment through the instance manager, waits until it is seeded
// and running, points DATABASE_ID back at the original database and removes the old database.
func (m *Migrator) Deploy(ctx context.Context) error {
	states, err := findStates(m.DB, StepReleased)
	if err != nil {
		return err
	}
	states = slices.DeleteFunc(states, func(s State) bool { return !m.selected(s.DeploymentID) })
	return each(m.Concurrency, states, func(state State) error {
		if err := m.deploy(ctx, &state); err != nil {
			return recordFailure(m.DB, &state, fmt.Errorf("deployment %d (%s): deploy: %v", state.DeploymentID, state.DeploymentName, err))
		}
		return nil
	})
}

func (m *Migrator) deploy(ctx context.Context, state *State) error {
	group, err := m.group(state.GroupName)
	if err != nil {
		return err
	}
	cluster, err := m.cluster(group)
	if err != nil {
		return err
	}
	d := LegacyDeployment{Deployment: &model.Deployment{ID: state.DeploymentID, Name: state.DeploymentName}, Group: group}
	fullname := d.ReleaseName() + "-dhis2"
	logger := m.Logger.With("deploymentId", state.DeploymentID, "deployment", state.DeploymentName, "group", state.GroupName)

	logger.InfoContext(ctx, "Deploying")
	start := time.Now()
	if err := m.API.Deploy(ctx, state.DeploymentID); err != nil {
		return err
	}

	// Seeding runs as a helm hook, and a restore longer than helm's timeout fails the deploy while
	// the seed carries on. So the deploy is judged by the cluster: the seed has completed and the
	// core is available.
	err = waitFor(ctx, m.DeployTimeout, func(ctx context.Context) (bool, error) {
		seeded, err := cluster.jobSucceeded(ctx, fullname+"-db-seed")
		if err != nil || !seeded {
			return false, err
		}
		core, err := cluster.deployment(ctx, fullname)
		if err != nil {
			return false, nil
		}
		return core.Status.AvailableReplicas > 0, nil
	})
	if err != nil {
		return fmt.Errorf("dhis2-v2 was not seeded and running within %s: %v", m.DeployTimeout, err)
	}

	// A deploy that timed out on the seed never reached the companions, and its instances report the
	// timeout. Deploying again is quick once the seed marker exists, and settles both.
	if err := m.waitForDeploy(ctx, state.DeploymentID); err != nil {
		return err
	}
	if !m.allDeployed(ctx, state.DeploymentID) {
		logger.InfoContext(ctx, "Deploying again now that the seed is done")
		if err := m.API.Deploy(ctx, state.DeploymentID); err != nil {
			return err
		}
		if err := m.waitForDeploy(ctx, state.DeploymentID); err != nil {
			return err
		}
		if !m.allDeployed(ctx, state.DeploymentID) {
			return fmt.Errorf("the second deploy did not deploy every instance")
		}
	}
	logger.InfoContext(ctx, "Deployed", "duration", time.Since(start).Round(time.Second))

	if err := RestoreDatabaseID(m.DB, state); err != nil {
		return err
	}
	if err := cluster.uninstall(ctx, databaseRelease(d)); err != nil {
		return err
	}
	// Older dhis2-db releases predate the retention policy that deletes the claim with the release.
	if err := cluster.deletePVC(ctx, databaseClaim(d), 10*time.Minute); err != nil {
		return err
	}
	if state.WasPaused {
		if err := m.API.Pause(ctx, state.CoreInstanceID); err != nil {
			return err
		}
	}

	state.Step = StepDeployed
	state.Error = ""
	return saveState(m.DB, state)
}

// waitForDeploy waits until none of the deployment's instances is queued for or in a deploy.
func (m *Migrator) waitForDeploy(ctx context.Context, deploymentID uint) error {
	return waitFor(ctx, m.DeployTimeout, func(ctx context.Context) (bool, error) {
		deployment, err := m.API.Deployment(ctx, deploymentID)
		if err != nil {
			m.Logger.WarnContext(ctx, "Failed to read the deployment, retrying", "deploymentId", deploymentID, "error", err)
			return false, nil
		}
		for _, instance := range deployment.Instances {
			if instance.DeployStatus == model.DeployStatusPending || instance.DeployStatus == model.DeployStatusDeploying {
				return false, nil
			}
		}
		return true, nil
	})
}

func (m *Migrator) allDeployed(ctx context.Context, deploymentID uint) bool {
	deployment, err := m.API.Deployment(ctx, deploymentID)
	if err != nil {
		return false
	}
	for _, instance := range deployment.Instances {
		if instance.DeployStatus != model.DeployStatusDeployed {
			return false
		}
	}
	return true
}

func waitFor(ctx context.Context, timeout time.Duration, done func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		ok, err := done(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}
}

// Cleanup deletes the snapshots of deployments that have run on dhis2-v2 for at least grace.
func (m *Migrator) Cleanup(ctx context.Context, grace time.Duration) error {
	states, err := findStates(m.DB, StepDeployed)
	if err != nil {
		return err
	}
	states = slices.DeleteFunc(states, func(s State) bool {
		return !m.selected(s.DeploymentID) || time.Since(s.UpdatedAt) < grace
	})
	return each(m.Concurrency, states, func(state State) error {
		if err := m.API.DeleteDatabase(ctx, state.SnapshotDatabaseID); err != nil {
			return recordFailure(m.DB, &state, fmt.Errorf("deployment %d (%s): cleanup: %v", state.DeploymentID, state.DeploymentName, err))
		}
		state.Step = StepCleaned
		state.Error = ""
		return saveState(m.DB, &state)
	})
}

// PrintPlan writes the plan as a table.
func PrintPlan(w io.Writer, planned []PlannedDeployment, unsupported []Unsupported) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "ID\tDEPLOYMENT\tGROUP\tSTORAGE\tPGADMIN\tDATABASE\tPROBLEM")
	var total int64
	for _, p := range planned {
		total += p.DatabaseSize
		_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%t\t%s\t%s\n", p.Deployment.ID, p.Deployment.Name, p.Deployment.GroupName, p.StorageType(), p.PgAdmin != nil, gibibytes(p.DatabaseSize), p.Problem)
	}
	_, _ = fmt.Fprintf(table, "\t%d deployment(s)\t\t\t\t%s\t\n", len(planned), gibibytes(total))
	if err := table.Flush(); err != nil {
		return err
	}
	if len(unsupported) == 0 {
		return nil
	}
	_, _ = fmt.Fprintf(w, "\n%d deployment(s) cannot be migrated:\n", len(unsupported))
	table = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, u := range unsupported {
		_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", u.DeploymentID, u.DeploymentName, u.GroupName, u.Reason)
	}
	return table.Flush()
}

func gibibytes(size int64) string {
	return fmt.Sprintf("%.1f GiB", float64(size)/(1<<30))
}

// PrintStatus writes every recorded migration state as a table.
func (m *Migrator) PrintStatus(w io.Writer) error {
	var states []State
	if err := m.DB.Order("deployment_id").Find(&states).Error; err != nil {
		return fmt.Errorf("failed to load migration states: %v", err)
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "ID\tDEPLOYMENT\tGROUP\tSTEP\tSNAPSHOT\tUPDATED\tERROR")
	for _, s := range states {
		_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%d\t%s\t%s\n", s.DeploymentID, s.DeploymentName, s.GroupName, s.Step, s.SnapshotDatabaseID, s.UpdatedAt.Format(time.RFC3339), s.Error)
	}
	return table.Flush()
}
