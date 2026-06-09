package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"project-management-platform/internal/config"
	"project-management-platform/internal/database"
	"project-management-platform/internal/handlers"
	"project-management-platform/internal/middleware"
	"project-management-platform/internal/repository"
	"project-management-platform/internal/services"
	"project-management-platform/internal/websocket"
)

func main() {
	cfg := config.Load()

	db := database.Connect(cfg)

	// --- Dependency wiring ---
	// Repositories
	userRepo := repository.NewUserRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	issueRepo := repository.NewIssueRepo(db)
	sprintRepo := repository.NewSprintRepo(db)
	commentRepo := repository.NewCommentRepo(db)
	workflowRepo := repository.NewWorkflowRepo(db)
	activityRepo := repository.NewActivityRepo(db)
	notifRepo := repository.NewNotificationRepo(db)
	watcherRepo := repository.NewWatcherRepo(db)
	searchRepo := repository.NewSearchRepo(db)
	customFieldRepo := repository.NewCustomFieldRepo(db)

	// WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Services
	activitySvc := services.NewActivityService(activityRepo, wsHub)
	notifSvc := services.NewNotificationService(notifRepo)
	workflowSvc := services.NewWorkflowService(workflowRepo, activitySvc, notifSvc, watcherRepo, wsHub)
	projectSvc := services.NewProjectService(projectRepo, workflowRepo)
	issueSvc := services.NewIssueService(issueRepo, projectRepo, workflowSvc, activitySvc, notifSvc, watcherRepo, wsHub)
	sprintSvc := services.NewSprintService(sprintRepo, issueRepo, activitySvc, wsHub)
	commentSvc := services.NewCommentService(commentRepo, issueRepo, activitySvc, notifSvc, watcherRepo, wsHub)
	searchSvc := services.NewSearchService(searchRepo)

	// Handlers
	userHandler := handlers.NewUserHandler(userRepo)
	projectHandler := handlers.NewProjectHandler(projectSvc, customFieldRepo)
	issueHandler := handlers.NewIssueHandler(issueSvc)
	sprintHandler := handlers.NewSprintHandler(sprintSvc)
	commentHandler := handlers.NewCommentHandler(commentSvc)
	activityHandler := handlers.NewActivityHandler(activityRepo)
	searchHandler := handlers.NewSearchHandler(searchSvc)
	notifHandler := handlers.NewNotificationHandler(notifRepo)
	watcherHandler := handlers.NewWatcherHandler(watcherRepo)
	wsHandler := handlers.NewWSHandler(wsHub)

	// --- Router ---
	r := gin.New()
	r.Use(middleware.CORS(), middleware.RequestLogger(), middleware.Recovery())

	api := r.Group("/api")
	{
		// Users
		api.POST("/users", userHandler.Create)
		api.GET("/users", userHandler.List)
		api.GET("/users/:id", userHandler.Get)

		// Projects
		api.POST("/projects", projectHandler.Create)
		api.GET("/projects", projectHandler.List)
		api.GET("/projects/:id", projectHandler.Get)
		api.PATCH("/projects/:id", projectHandler.Update)

		// Project workflow configuration
		api.GET("/projects/:id/statuses", projectHandler.ListStatuses)
		api.GET("/projects/:id/transitions", projectHandler.ListTransitions)
		api.POST("/projects/:id/transitions", projectHandler.CreateTransition)

		// Custom field definitions
		api.POST("/projects/:id/custom-fields", projectHandler.CreateCustomField)
		api.GET("/projects/:id/custom-fields", projectHandler.ListCustomFields)

		// Project board view
		api.GET("/projects/:id/board", issueHandler.GetBoard)

		// Issues
		api.POST("/projects/:id/issues", issueHandler.Create)
		api.GET("/projects/:id/issues", issueHandler.List)
		api.GET("/issues/:id", issueHandler.Get)
		api.PATCH("/issues/:id", issueHandler.Update)
		api.DELETE("/issues/:id", issueHandler.Delete)

		// Issue transitions (workflow engine)
		api.POST("/issues/:id/transitions", issueHandler.Transition)

		// Sprints
		api.GET("/projects/:id/sprints", sprintHandler.List)
		api.POST("/projects/:id/sprints", sprintHandler.Create)
		api.PATCH("/sprints/:id", sprintHandler.Update)
		api.POST("/sprints/:id/start", sprintHandler.Start)
		api.POST("/sprints/:id/complete", sprintHandler.Complete)

		// Move issue to/from sprint
		api.POST("/issues/:id/move-to-sprint", issueHandler.MoveToSprint)

		// Comments (threaded)
		api.GET("/issues/:id/comments", commentHandler.List)
		api.POST("/issues/:id/comments", commentHandler.Create)

		// Activity feed
		api.GET("/projects/:id/activity", activityHandler.List)

		// Notifications
		api.GET("/users/:id/notifications", notifHandler.List)
		api.PATCH("/notifications/:id/read", notifHandler.MarkRead)

		// Watchers
		api.POST("/issues/:id/watch", watcherHandler.Watch)
		api.DELETE("/issues/:id/watch", watcherHandler.Unwatch)
		api.GET("/issues/:id/watchers", watcherHandler.List)

		// Search
		api.GET("/search", searchHandler.Search)
	}

	// WebSocket endpoint
	r.GET("/ws", wsHandler.HandleConnection)

	port := cfg.ServerPort
	log.Printf("server starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
