package handlers

import (
	"github.com/portd/internal/auth"
	internsvc "github.com/portd/internal/interns"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
)

type AuthHandler struct {
	service *auth.Service
}

func NewAuthHandler(service *auth.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

type InternsHandler struct {
	service *internsvc.Service
}

func NewInternsHandler(service *internsvc.Service) *InternsHandler {
	return &InternsHandler{service: service}
}

type PortsHandler struct {
	service *portsvc.Service
}

func NewPortsHandler(service *portsvc.Service) *PortsHandler {
	return &PortsHandler{service: service}
}

type ProjectsHandler struct {
	service *projectsvc.Service
}

func NewProjectsHandler(service *projectsvc.Service) *ProjectsHandler {
	return &ProjectsHandler{service: service}
}
