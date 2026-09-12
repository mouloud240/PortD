package handlers

import (
	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/auth"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/internal/runtime"
)

type AuthHandler struct {
	service  *auth.Service
	activity *activitysvc.Service
}

func NewAuthHandler(service *auth.Service, activity *activitysvc.Service) *AuthHandler {
	return &AuthHandler{service: service, activity: activity}
}

type InternsHandler struct {
	service  *internsvc.Service
	activity *activitysvc.Service
}

func NewInternsHandler(service *internsvc.Service, activity *activitysvc.Service) *InternsHandler {
	return &InternsHandler{service: service, activity: activity}
}

type PortsHandler struct {
	service *portsvc.Service
}

func NewPortsHandler(service *portsvc.Service) *PortsHandler {
	return &PortsHandler{service: service}
}

type ProjectsHandler struct {
	service  *projectsvc.Service
	runtime  *runtime.Manager
	activity *activitysvc.Service
}

func NewProjectsHandler(service *projectsvc.Service, activity *activitysvc.Service, managers ...*runtime.Manager) *ProjectsHandler {
	manager := runtime.NewManager()
	if len(managers) > 0 && managers[0] != nil {
		manager = managers[0]
	}
	return &ProjectsHandler{service: service, runtime: manager, activity: activity}
}
