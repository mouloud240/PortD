package runtime

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestFakeStarterRecordsProject(t *testing.T) {
	t.Parallel()

	starter := &FakeStarter{Err: errors.New("failed")}
	project := Project{Directory: "projects/demo", Executable: "./start.sh"}
	_, err := starter.Start(context.Background(), project)
	if !errors.Is(err, starter.Err) {
		t.Fatalf("error = %v, want fake error", err)
	}
	if len(starter.Requests) != 1 ||
		starter.Requests[0].Directory != project.Directory ||
		starter.Requests[0].Executable != project.Executable ||
		!slices.Equal(starter.Requests[0].Arguments, project.Arguments) {
		t.Fatalf("requests = %+v, want %+v", starter.Requests, project)
	}
}
