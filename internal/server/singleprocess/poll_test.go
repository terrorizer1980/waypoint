package singleprocess

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint/internal/server"
	pb "github.com/hashicorp/waypoint/internal/server/gen"
	serverptypes "github.com/hashicorp/waypoint/internal/server/ptypes"
)

func TestServicePollQueue(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Create our server
	impl, err := New(WithDB(testDB(t)))
	require.NoError(err)
	client := server.TestServer(t, impl)

	// Create a project
	_, err = client.UpsertProject(ctx, &pb.UpsertProjectRequest{
		Project: serverptypes.TestProject(t, &pb.Project{
			Name: "A",
			DataSource: &pb.Job_DataSource{
				Source: &pb.Job_DataSource_Local{
					Local: &pb.Job_Local{},
				},
			},
			DataSourcePoll: &pb.Project_Poll{
				Enabled:  true,
				Interval: "15ms",
			},
		}),
	})
	require.NoError(err)

	// Wait a bit. The interval is so low that this should trigger
	// multiple loops through the poller. But we want to ensure we
	// have only one poll job queued.
	time.Sleep(50 * time.Millisecond)

	// Check for our condition, we do eventually here because if we're
	// in a slow environment then this may still be empty.
	require.Eventually(func() bool {
		// We should have a single poll job
		var jobs []*pb.Job
		raw, err := testServiceImpl(impl).state.JobList()
		for _, j := range raw {
			if j.State != pb.Job_ERROR {
				jobs = append(jobs, j)
			}
		}

		if err != nil {
			t.Logf("err: %s", err)
			return false
		}

		return len(jobs) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// Cancel our poller to ensure it stops
	testServiceImpl(impl).Close()

	// Ensure we don't queue more jobs
	time.Sleep(100 * time.Millisecond)
	raw, err := testServiceImpl(impl).state.JobList()
	require.NoError(err)
	time.Sleep(100 * time.Millisecond)
	raw2, err := testServiceImpl(impl).state.JobList()
	require.NoError(err)
	require.Equal(len(raw), len(raw2))
}

func TestProjectPollHandler(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Create our server
	impl, err := New(WithDB(testDB(t)))
	require.NoError(err)
	client := server.TestServer(t, impl)

	// Create a project
	_, err = client.UpsertProject(ctx, &pb.UpsertProjectRequest{
		Project: serverptypes.TestProject(t, &pb.Project{
			Name: "Example",
			DataSource: &pb.Job_DataSource{
				Source: &pb.Job_DataSource_Local{
					Local: &pb.Job_Local{},
				},
			},
			DataSourcePoll: &pb.Project_Poll{
				Enabled:  true,
				Interval: "15ms",
			},
			Applications: []*pb.Application{
				{
					Project: &pb.Ref_Project{Project: "Example"},
					Name:    "apple-app",
					StatusReportPoll: &pb.Application_Poll{
						Enabled: false,
					},
				},
			},
		}),
	})
	require.NoError(err)

	// Grab next poll time
	state := testServiceImpl(impl).state
	p, pollTime, err := state.ProjectPollPeek(nil)
	require.NoError(err)
	require.NotNil(p)
	require.NotNil(pollTime)

	// Wait a bit. The interval is so low that this should trigger
	// multiple loops through the poller. But we want to ensure we
	// have only one poll job queued.
	time.Sleep(50 * time.Millisecond)

	// Check for our condition, we do eventually here because if we're
	// in a slow environment then this may still be empty.
	require.Eventually(func() bool {
		// We should have a single poll job
		var jobs []*pb.Job
		raw, err := testServiceImpl(impl).state.JobList()
		for _, j := range raw {
			if j.State != pb.Job_ERROR {
				jobs = append(jobs, j)
			}
		}

		if err != nil {
			t.Logf("err: %s", err)
			return false
		}

		return len(jobs) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// Cancel our poller to ensure it stops
	testServiceImpl(impl).Close()

	// ensure the next poll is after the initial poll before waiting
	// next poll time gets set when a project poll is marked complete
	p, nextPollTime, err := state.ProjectPollPeek(nil)
	require.NoError(err)
	require.NotNil(p)
	require.NotNil(nextPollTime)
	require.True(nextPollTime.After(pollTime))
}

func TestApplicationPollHandler(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Create our server
	impl, err := New(WithDB(testDB(t)))
	require.NoError(err)
	client := server.TestServer(t, impl)

	// Create a project with an application
	_, err = client.UpsertProject(ctx, &pb.UpsertProjectRequest{
		Project: serverptypes.TestProject(t, &pb.Project{
			Name: "Example",
			DataSource: &pb.Job_DataSource{
				Source: &pb.Job_DataSource_Local{
					Local: &pb.Job_Local{},
				},
			},
			DataSourcePoll: &pb.Project_Poll{
				Enabled:  true,
				Interval: "15ms",
			},
			Applications: []*pb.Application{
				{
					Project: &pb.Ref_Project{Project: "Example"},
					Name:    "apple-app",
					StatusReportPoll: &pb.Application_Poll{
						Enabled:  false,
						Interval: "15ms",
					},
				},
			},
		}),
	})
	require.NoError(err)

	// Grab next poll time
	state := testServiceImpl(impl).state
	a, _, err := state.ApplicationPollPeek(nil)
	require.NoError(err)
	require.Nil(a) // Apps Next Poll should be 0 since not started yet

	// Wait a bit. The interval is so low that this should trigger
	// multiple loops through the poller. But we want to ensure we
	// have only one poll job queued.
	time.Sleep(50 * time.Millisecond)

	// Do a deployment
	resp, err := client.UpsertDeployment(ctx, &pb.UpsertDeploymentRequest{
		Deployment: serverptypes.TestValidDeployment(t, &pb.Deployment{
			Component: &pb.Component{
				Name: "testapp",
			},
			Application: &pb.Ref_Application{
				Application: "apple-app",
				Project:     "Example",
			},
		}),
	})
	require.NoError(err)
	require.NotNil(resp)

	// Update the app to start polling
	_, err = client.UpsertApplication(ctx, &pb.UpsertApplicationRequest{
		Project: &pb.Ref_Project{Project: "Example"},
		Name:    "apple-app",
		Poll:    true,
	})
	require.NoError(err)

	// App poll time should be set
	a, pollTime, err := state.ApplicationPollPeek(nil)
	require.NoError(err)
	require.NotNil(pollTime)
	require.NotNil(a) // Apps Next Poll should be set

	// Wait a bit. The interval is so low that this should trigger
	// multiple loops through the poller. But we want to ensure we
	// have only one poll job queued.
	time.Sleep(50 * time.Millisecond)

	// Check for our condition, we do eventually here because if we're
	// in a slow environment then this may still be empty.
	require.Eventually(func() bool {
		// We should have a single poll job
		var jobs []*pb.Job
		raw, err := testServiceImpl(impl).state.JobList()
		for _, j := range raw {
			if j.State != pb.Job_ERROR {
				jobs = append(jobs, j)
			}
		}

		if err != nil {
			t.Logf("err: %s", err)
			return false
		}

		return len(jobs) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// Cancel our poller to ensure it stops
	testServiceImpl(impl).Close()

	// ensure the next poll is after the initial poll before waiting
	// next poll time gets set when a project poll is marked complete
	a, nextPollTime, err := state.ApplicationPollPeek(nil)
	require.NoError(err)
	require.NotNil(a)
	require.NotNil(nextPollTime)
	require.True(nextPollTime.After(pollTime))
}
